//go:build no_license

package middleware

import "github.com/gin-gonic/gin"

// LicenseMiddleware is a compile-time no-op when built with -tags no_license
// (the public / 公开版 build). It never gates any request, and the license
// enforcement code paths are effectively compiled out of the binary.
func LicenseMiddleware(gate LicenseGate) gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Next()
	}
}
