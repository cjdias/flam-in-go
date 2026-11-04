package flam

import (
	"database/sql"
	"time"

	"gorm.io/gorm"
)

// StrPtr returns a pointer to the string held by the nullable string, or
// nil when it is NULL, as when mapping a database row to a JSON model
// with optional fields. The pointer addresses a copy, so changing it does
// not change the given value.
func StrPtr(
	n sql.NullString,
) *string {
	if n.Valid {
		return &n.String
	}
	return nil
}

// I32Ptr returns a pointer to the int32 held by the nullable int32, or nil
// when it is NULL. The pointer addresses a copy of the value.
func I32Ptr(
	n sql.NullInt32,
) *int32 {
	if n.Valid {
		return &n.Int32
	}
	return nil
}

// F64Ptr returns a pointer to the float64 held by the nullable float64, or
// nil when it is NULL. The pointer addresses a copy of the value.
func F64Ptr(
	n sql.NullFloat64,
) *float64 {
	if n.Valid {
		return &n.Float64
	}
	return nil
}

// BoolPtr returns a pointer to the bool held by the nullable bool, or nil
// when it is NULL. The pointer addresses a copy of the value.
func BoolPtr(
	n sql.NullBool,
) *bool {
	if n.Valid {
		return &n.Bool
	}
	return nil
}

// DatePtr returns a pointer to the date of the nullable time, formatted as
// [time.DateOnly] ("2006-01-02") in the time's own location, or nil when
// it is NULL. The clock part of the time is dropped.
func DatePtr(
	n sql.NullTime,
) *string {
	if !n.Valid {
		return nil
	}

	date := n.Time.Format(time.DateOnly)

	return &date
}

// DeletedPtr returns a pointer to the moment of the soft deletion held by
// the gorm deletion marker, or nil when the record is not deleted. The
// pointer addresses a copy of the moment.
func DeletedPtr(
	value gorm.DeletedAt,
) *time.Time {
	if !value.Valid {
		return nil
	}

	moment := value.Time

	return &moment
}
