package security

import (
	cryptorand "crypto/rand"
	"math/big"
	mathrand "math/rand"
)

// cryptoRandRead is a package-level var so tests can swap it.
var cryptoRandRead = cryptorand.Read

// pseudoRandIntn returns a number in [0, max) using math/rand. Only called
// when crypto/rand fails — cryptoRandRead should almost never fail in
// practice on Linux/Windows Go runtimes.
func pseudoRandIntn(max int) int {
	n, err := cryptorand.Int(cryptorand.Reader, big.NewInt(int64(max)))
	if err == nil {
		return int(n.Int64())
	}
	return mathrand.Intn(max)
}
