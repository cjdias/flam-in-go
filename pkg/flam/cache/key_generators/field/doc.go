// Package field is the cache key generator driver keying the objects by
// the value of one of their struct fields, "Id" unless configured, after
// an optional prefix and their type, such as "models.User:7" for a
// *models.User of Id 7. Its bundle configures the key generator of id [Id], the
// default of the adaptors.
//
// The objects of equal field values, and of the same type when the type
// is in the keys, share a key: those of a zero value, such as records not
// yet saved, share it too, so set them aside.
package field
