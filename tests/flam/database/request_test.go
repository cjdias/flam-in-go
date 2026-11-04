package database

import (
	"testing"

	gormmysql "gorm.io/driver/mysql"
	"gorm.io/gorm"
	"gorm.io/gorm/utils/tests"

	"github.com/cjdias/flam-in-go/pkg/flam"
	"github.com/cjdias/flam-in-go/pkg/flam/database"
	"github.com/cjdias/flam-in-go/tests/flam/testutil"
)

type user struct {
	gorm.Model
	Name string
}

func dryRun(t *testing.T) *gorm.DB {
	t.Helper()

	db, e := gorm.Open(tests.DummyDialector{}, &gorm.Config{DryRun: true})
	testutil.NoError(t, e)

	return db
}

func sqlOf(t *testing.T, scope func(*gorm.DB) *gorm.DB) string {
	t.Helper()

	sql, _ := query(t, scope)

	return sql
}

func query(t *testing.T, scope func(*gorm.DB) *gorm.DB) (string, []any) {
	t.Helper()

	stmt := dryRun(t).Model(&user{}).Scopes(scope).Find(&[]user{}).Statement

	return stmt.SQL.String(), stmt.Vars
}

func intPtr(v int) *int {
	return &v
}

func Test_Request_Add(t *testing.T) {
	t.Run("should add filters keyed by field, the last one winning", func(t *testing.T) {
		r := &database.Request{}
		got := r.AddFilter("age", ">", 1).AddFilter("age", "<", 9).AddFilter("name", "=", "x")

		testutil.Equal(t, "chained", got, flam.IRequest(r))
		testutil.Equal(t, "Filters", r.Filters, map[string]database.RequestFilter{
			"age":  {Field: "age", Operator: "<", Value: 9},
			"name": {Field: "name", Operator: "=", Value: "x"},
		})
	})

	t.Run("should fold the text filters of like operators", func(t *testing.T) {
		r := &database.Request{}
		r.AddTextFilter("name", "JoHn").AddTextFilter("code", "AbC", "=").AddTextFilter("mail", "X@Y", " NOT ILIKE ")

		testutil.Equal(t, "Filters", r.Filters, map[string]database.RequestFilter{
			"lower(name)": {Field: "lower(name)", Operator: "like", Value: "john"},
			"code":        {Field: "code", Operator: "=", Value: "AbC"},
			"lower(mail)": {Field: "lower(mail)", Operator: " NOT ILIKE ", Value: "x@y"},
		})
	})

	t.Run("should keep the sort order, the last direction winning", func(t *testing.T) {
		r := &database.Request{}
		r.AddSort("kind").AddSort("name", flam.RequestSortDesc).AddSort("kind", flam.RequestSortDesc).AddSort("age", "bogus")

		testutil.Equal(t, "Sorters", r.Sorters, []database.RequestSort{
			{Field: "kind", Dir: flam.RequestSortDesc},
			{Field: "name", Dir: flam.RequestSortDesc},
			{Field: "age", Dir: flam.RequestSortAsc},
		})
	})

	t.Run("should add relations keyed by name", func(t *testing.T) {
		r := &database.Request{}
		r.AddRelation("Orders").AddRelation("Tags", "active = ?")

		testutil.Equal(t, "Relations", r.Relations, map[string]database.RequestRelation{
			"Orders": {Relation: "Orders"},
			"Tags":   {Relation: "Tags", Condition: "active = ?"},
		})
	})

	t.Run("should not write into the caller's option slices", func(t *testing.T) {
		ops := make([]string, 0, 1)
		dirs := make([]flam.RequestSortDirection, 0, 1)
		conds := make([]string, 0, 1)

		r := &database.Request{}
		r.AddTextFilter("name", "x", ops...).AddSort("name", dirs...).AddRelation("Tags", conds...)

		testutil.Equal(t, "ops", ops[:1], []string{""})
		testutil.Equal(t, "dirs", dirs[:1], []flam.RequestSortDirection{""})
		testutil.Equal(t, "conds", conds[:1], []string{""})
	})
}

func Test_Request_MySQL(t *testing.T) {
	t.Run("should leave the escape clause out on mysql", func(t *testing.T) {
		db, err := gorm.Open(gormmysql.New(gormmysql.Config{
			DSN: "root@tcp(127.0.0.1:1)/flam", SkipInitializeWithVersion: true,
		}), &gorm.Config{DryRun: true, DisableAutomaticPing: true})
		testutil.NoError(t, err)

		r := (&database.Request{}).AddTextFilter("mail", "a")
		stmt := db.Model(&user{}).Scopes(r.FilterScope).Find(&[]user{}).Statement
		testutil.Equal(t, "FilterScope", stmt.SQL.String(),
			"SELECT * FROM `users` WHERE lower(mail) like ? AND `users`.`deleted_at` IS NULL")
	})
}

func Test_Request_Scope(t *testing.T) {
	t.Run("should apply nothing for an empty request", func(t *testing.T) {
		r := &database.Request{}

		testutil.Equal(t, "Scope", sqlOf(t, r.Scope),
			"SELECT * FROM `users` WHERE `users`.`deleted_at` IS NULL")
	})

	t.Run("should apply the whole request, in a stable order", func(t *testing.T) {
		r := &database.Request{Offset: intPtr(20), Limit: intPtr(10)}
		r.AddFilter("name", "=", "x").AddFilter("age", ">", 1).AddTextFilter("mail", "a").
			AddSort("name", flam.RequestSortDesc).AddSort("age")

		want := "SELECT * FROM `users` WHERE age > ? AND lower(mail) like ? escape '\\' AND name = ?" +
			" AND `users`.`deleted_at` IS NULL ORDER BY name desc,age asc LIMIT ? OFFSET ?"
		for range 5 {
			sql, vars := query(t, r.Scope)
			testutil.Equal(t, "Scope", sql, want)
			testutil.Equal(t, "Vars", vars, []any{1, "a", "x", 10, 20})
		}
	})

	t.Run("should bound the offset and the limit", func(t *testing.T) {
		r := &database.Request{Offset: intPtr(-5), Limit: intPtr(0)}

		sql, vars := query(t, r.Scope)
		testutil.Equal(t, "Scope", sql, "SELECT * FROM `users` WHERE `users`.`deleted_at` IS NULL LIMIT ?")
		testutil.Equal(t, "Vars", vars, []any{1})
	})

	t.Run("should include the soft-deleted rows when asked", func(t *testing.T) {
		r := &database.Request{SoftDeleted: true}
		r.AddFilter("age", ">", 1)

		testutil.Equal(t, "Scope", sqlOf(t, r.Scope), "SELECT * FROM `users` WHERE age > ?")
		testutil.Equal(t, "FilterScope", sqlOf(t, r.FilterScope), "SELECT * FROM `users` WHERE age > ?")
	})

	t.Run("should apply only the filters with FilterScope", func(t *testing.T) {
		r := &database.Request{Offset: intPtr(2), Limit: intPtr(3)}
		r.AddFilter("age", ">", 1).AddSort("name").AddRelation("Orders")

		testutil.Equal(t, "FilterScope", sqlOf(t, r.FilterScope),
			"SELECT * FROM `users` WHERE age > ? AND `users`.`deleted_at` IS NULL")
	})

	t.Run("should preload the relations, with their condition", func(t *testing.T) {
		r := &database.Request{}
		r.AddRelation("Tags", "active = ?").AddRelation("Orders")

		preloads := r.Scope(dryRun(t)).Statement.Preloads
		testutil.Equal(t, "Preloads", preloads, map[string][]any{
			"Orders": nil,
			"Tags":   {"active = ?"},
		})
	})
}
