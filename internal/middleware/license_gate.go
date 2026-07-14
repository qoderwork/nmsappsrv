package middleware

// LicenseGate is the minimal interface the license middleware needs from the
// enforcer. Declared in a build-tag-free file so both the active and the
// no_license (public build) variants can reference it.
type LicenseGate interface {
	Required() bool
	IsValid() bool
}
