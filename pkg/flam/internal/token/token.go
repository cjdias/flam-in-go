// Package token holds the random claim tokens shared by the flam packages
// claiming rows of a database, such as the jobs and the outbox.
package token

import (
	"crypto/rand"
	"encoding/hex"
)

// New returns a new random token, of 32 hexadecimal digits.
func New() string {
	b := make([]byte, 16)
	_, _ = rand.Read(b) // never fails
	return hex.EncodeToString(b)
}
