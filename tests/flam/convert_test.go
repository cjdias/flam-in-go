package flam

import (
	"database/sql"
	"testing"
	"time"

	"gorm.io/gorm"

	"github.com/cjdias/flam-in-go/pkg/flam"
	"github.com/cjdias/flam-in-go/tests/flam/testutil"
)

func TestStrPtr(t *testing.T) {
	t.Run("returns nil when NULL", func(t *testing.T) {
		testutil.Equal(t, "StrPtr", flam.StrPtr(sql.NullString{String: "ignored"}), (*string)(nil))
	})

	t.Run("returns a pointer to a copy of the value", func(t *testing.T) {
		n := sql.NullString{String: "value", Valid: true}
		got := flam.StrPtr(n)
		if got == nil {
			t.Fatal("StrPtr = nil, want a pointer")
		}
		testutil.Equal(t, "*StrPtr", *got, "value")
		*got = "changed"
		testutil.Equal(t, "n.String", n.String, "value")
	})

	t.Run("keeps an empty valid string", func(t *testing.T) {
		got := flam.StrPtr(sql.NullString{Valid: true})
		if got == nil {
			t.Fatal("StrPtr = nil, want a pointer")
		}
		testutil.Equal(t, "*StrPtr", *got, "")
	})
}

func TestI32Ptr(t *testing.T) {
	t.Run("returns nil when NULL", func(t *testing.T) {
		testutil.Equal(t, "I32Ptr", flam.I32Ptr(sql.NullInt32{Int32: 7}), (*int32)(nil))
	})

	t.Run("returns a pointer to a copy of the value", func(t *testing.T) {
		n := sql.NullInt32{Int32: 42, Valid: true}
		got := flam.I32Ptr(n)
		if got == nil {
			t.Fatal("I32Ptr = nil, want a pointer")
		}
		testutil.Equal(t, "*I32Ptr", *got, int32(42))
		*got = 1
		testutil.Equal(t, "n.Int32", n.Int32, int32(42))
	})

	t.Run("keeps a valid zero", func(t *testing.T) {
		got := flam.I32Ptr(sql.NullInt32{Valid: true})
		if got == nil {
			t.Fatal("I32Ptr = nil, want a pointer")
		}
		testutil.Equal(t, "*I32Ptr", *got, int32(0))
	})
}

func TestF64Ptr(t *testing.T) {
	t.Run("returns nil when NULL", func(t *testing.T) {
		testutil.Equal(t, "F64Ptr", flam.F64Ptr(sql.NullFloat64{Float64: 1.5}), (*float64)(nil))
	})

	t.Run("returns a pointer to a copy of the value", func(t *testing.T) {
		n := sql.NullFloat64{Float64: 3.25, Valid: true}
		got := flam.F64Ptr(n)
		if got == nil {
			t.Fatal("F64Ptr = nil, want a pointer")
		}
		testutil.Equal(t, "*F64Ptr", *got, 3.25)
		*got = 0
		testutil.Equal(t, "n.Float64", n.Float64, 3.25)
	})

	t.Run("keeps a valid zero", func(t *testing.T) {
		got := flam.F64Ptr(sql.NullFloat64{Valid: true})
		if got == nil {
			t.Fatal("F64Ptr = nil, want a pointer")
		}
		testutil.Equal(t, "*F64Ptr", *got, 0.0)
	})
}

func TestBoolPtr(t *testing.T) {
	t.Run("returns nil when NULL", func(t *testing.T) {
		testutil.Equal(t, "BoolPtr", flam.BoolPtr(sql.NullBool{Bool: true}), (*bool)(nil))
	})

	t.Run("returns a pointer to a copy of the value", func(t *testing.T) {
		n := sql.NullBool{Bool: true, Valid: true}
		got := flam.BoolPtr(n)
		if got == nil {
			t.Fatal("BoolPtr = nil, want a pointer")
		}
		testutil.Equal(t, "*BoolPtr", *got, true)
		*got = false
		testutil.Equal(t, "n.Bool", n.Bool, true)
	})

	t.Run("keeps a valid false", func(t *testing.T) {
		got := flam.BoolPtr(sql.NullBool{Valid: true})
		if got == nil {
			t.Fatal("BoolPtr = nil, want a pointer")
		}
		testutil.Equal(t, "*BoolPtr", *got, false)
	})
}

func TestDatePtr(t *testing.T) {
	t.Run("returns nil when NULL", func(t *testing.T) {
		n := sql.NullTime{Time: time.Date(2024, 1, 2, 0, 0, 0, 0, time.UTC)}
		testutil.Equal(t, "DatePtr", flam.DatePtr(n), (*string)(nil))
	})

	t.Run("returns the date dropping the clock", func(t *testing.T) {
		n := sql.NullTime{Time: time.Date(2024, 2, 29, 23, 59, 59, 999, time.UTC), Valid: true}
		got := flam.DatePtr(n)
		if got == nil {
			t.Fatal("DatePtr = nil, want a pointer")
		}
		testutil.Equal(t, "*DatePtr", *got, "2024-02-29")
	})

	t.Run("uses the location of the time", func(t *testing.T) {
		loc := time.FixedZone("UTC+2", 2*60*60)
		n := sql.NullTime{Time: time.Date(2024, 3, 1, 1, 0, 0, 0, loc), Valid: true}
		got := flam.DatePtr(n)
		if got == nil {
			t.Fatal("DatePtr = nil, want a pointer")
		}
		testutil.Equal(t, "*DatePtr", *got, "2024-03-01")
	})

	t.Run("formats a valid zero time", func(t *testing.T) {
		got := flam.DatePtr(sql.NullTime{Valid: true})
		if got == nil {
			t.Fatal("DatePtr = nil, want a pointer")
		}
		testutil.Equal(t, "*DatePtr", *got, "0001-01-01")
	})
}

func TestDeletedPtr(t *testing.T) {
	t.Run("returns nil when not deleted", func(t *testing.T) {
		value := gorm.DeletedAt{Time: time.Date(2024, 1, 2, 3, 4, 5, 0, time.UTC)}
		testutil.Equal(t, "DeletedPtr", flam.DeletedPtr(value), (*time.Time)(nil))
	})

	t.Run("returns a pointer to a copy of the moment", func(t *testing.T) {
		moment := time.Date(2024, 1, 2, 3, 4, 5, 6, time.UTC)
		value := gorm.DeletedAt{Time: moment, Valid: true}
		got := flam.DeletedPtr(value)
		if got == nil {
			t.Fatal("DeletedPtr = nil, want a pointer")
		}
		testutil.Equal(t, "*DeletedPtr", *got, moment)
		*got = time.Time{}
		testutil.Equal(t, "value.Time", value.Time, moment)
	})
}
