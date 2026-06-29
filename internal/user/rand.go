package user

import (
	"crypto/rand"
	"io"
)

// randRead wraps crypto/rand.Read for testability.
var randRead = func(b []byte) (int, error) {
	return io.ReadFull(rand.Reader, b)
}
