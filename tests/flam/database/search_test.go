package database

import (
	"path/filepath"
	"testing"

	gormsqlite "gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"github.com/cjdias/flam-in-go/pkg/flam/database"
	"github.com/cjdias/flam-in-go/tests/flam/testutil"
)

func Test_SearchTerm(t *testing.T) {
	for term, want := range map[string]string{
		"":          `%%`,
		"john":      `%john%`,
		"50%":       `%50\%%`,
		"a_b":       `%a\_b%`,
		`c:\dir`:    `%c:\\dir%`,
		`\%_`:       `%\\\%\_%`,
		`already\%`: `%already\\\%%`,
	} {
		testutil.Equal(t, "SearchTerm("+term+")", database.SearchTerm(term), want)
	}
}

func Test_SearchTerm_Query(t *testing.T) {
	db, err := gorm.Open(gormsqlite.Open(filepath.Join(t.TempDir(), "test.db")), &gorm.Config{Logger: logger.Discard})
	testutil.NoError(t, err)
	testutil.NoError(t, db.AutoMigrate(&user{}))
	for _, name := range []string{"50% off", "500 off", "a_b", "axb", `c:\dir`, "c:dir", "John"} {
		testutil.NoError(t, db.Create(&user{Name: name}).Error)
	}

	for term, want := range map[string][]string{
		"50%":  {"50% off"},
		"_":    {"a_b"},
		`\`:    {`c:\dir`},
		"JOHN": {"John"},
		"":     {"50% off", "500 off", "a_b", "axb", `c:\dir`, "c:dir", "John"},
	} {
		r := &database.Request{}
		r.AddTextFilter("name", database.SearchTerm(term)).AddSort("id")
		var users []user
		testutil.NoError(t, db.Scopes(r.Scope).Find(&users).Error)
		names := make([]string, len(users))
		for i, u := range users {
			names[i] = u.Name
		}
		testutil.Equal(t, "names of "+term, names, want)
	}
}

func Test_Request_Paginate(t *testing.T) {
	t.Run("should set the offset and the limit", func(t *testing.T) {
		r := &database.Request{}
		testutil.Equal(t, "Paginate()", r.Paginate(40, 20), r)
		testutil.Equal(t, "Offset", r.Offset, intPtr(40))
		testutil.Equal(t, "Limit", r.Limit, intPtr(20))

		sql, vars := query(t, r.Scope)
		testutil.Equal(t, "Scope", sql, "SELECT * FROM `users` WHERE `users`.`deleted_at` IS NULL LIMIT ? OFFSET ?")
		testutil.Equal(t, "Vars", vars, []any{20, 40})
	})

	t.Run("should replace a previous page", func(t *testing.T) {
		r := (&database.Request{}).Paginate(1, 2).Paginate(-1, 0)
		sql, vars := query(t, r.Scope)
		testutil.Equal(t, "Scope", sql, "SELECT * FROM `users` WHERE `users`.`deleted_at` IS NULL LIMIT ?")
		testutil.Equal(t, "Vars", vars, []any{1})
	})
}
