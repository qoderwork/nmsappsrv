package enums

// Status mirrors the Java Status enum (com.waveoss.common.enumset.Status)
// exactly — same numeric codes, same semantics. These values are part of the
// wire contract with the original frontend; DO NOT renumber.
type Status struct {
	Code    int
	Message string
}

func NewStatus(code int, message string) Status {
	return Status{Code: code, Message: message}
}

var (
	SUCCESS                = NewStatus(200, "Success")
	ERROR                  = NewStatus(500, "Internal error")
	LOGOUT                 = NewStatus(200, "Success")
	UNAUTHORIZED           = NewStatus(401, "Please login")
	ACCESS_DENIED          = NewStatus(403, "No permission")
	REQUEST_NOT_FOUND      = NewStatus(404, "API does not exist")
	HTTP_BAD_METHOD        = NewStatus(405, "Request method error")
	BAD_REQUEST            = NewStatus(400, "Bad request")
	PARAM_NOT_MATCH        = NewStatus(400, "Parameter mismatch")
	PARAM_NOT_NULL         = NewStatus(400, "The parameter cannot be null")
	USER_DISABLED          = NewStatus(403, "The current user has been locked")
	TOO_MANY_REQUESTS      = NewStatus(429, "Too many requests")
	USERNAME_PASSWORD_ERROR = NewStatus(5001, "The user name and password do not match")
	TOKEN_EXPIRED          = NewStatus(5002, "Please login")
	TOKEN_PARSE_ERROR      = NewStatus(5002, "Please login")
	TOKEN_OUT_OF_CTRL      = NewStatus(5003, "Please login")
	KICKOUT_SELF           = NewStatus(5004, "Unable to manually kick yourself out, please try to exit the login operation")
	INVALID_TOKEN          = NewStatus(5009, "Please login")
	BLACK_TOKEN            = NewStatus(5005, "Please login")
)

// Login / security specific numeric codes. Thrown by Java as exception
// message strings (e.g. throw new BadCredentialsException("10047")); translated
// by the global exception handler (GlobalExceptionHandler.getLoginFault)
// into Result.code=100xx, Result.msg=lookup. Messages MUST match Java
// byte-for-byte because the original frontend keyed on exact code+msg.
var (
	LoginCodeEncryptionFailure      = 10044
	LoginMsgEncryptionFailure       = "Encryption failure"
	LoginCodeUserDisabled           = 10045
	LoginMsgUserDisabled            = "The user has been disabled"
	LoginCodeInternalError          = 10046
	LoginMsgInternalError           = "Internal error"
	LoginCodeBadCredentials         = 10047
	LoginMsgBadCredentials          = "The user name and password do not match"
	LoginCodeUserLocked             = 10048
	LoginMsgUserLocked              = "The user has been locked due to excessive login failures"
	LoginCodeAssignTenant           = 10049
	LoginMsgAssignTenant            = "Assign a tenant to the current user"
	LoginCodeTenantExpired          = 10050
	LoginMsgTenantExpired           = "The tenant to which the current user belongs has expired"
	LoginCodeCaptchaBad             = 10051
	LoginMsgCaptchaBad              = "Verification code error"
	LoginCodeAssignRole             = 10052
	LoginMsgAssignRole              = "Assign a role to the current user"
	LoginCodeNoRole                 = 10162
	LoginMsgNoRole                  = "Users must be assigned roles"
	LoginCodeRadiusUnavailable      = 10071
	LoginMsgRadiusUnavailable       = "Radius server is unavailable"
	LoginCodeInactive               = 10072
	LoginMsgInactive                = "The user has not logged in for a long time and is locked"
	LoginCodeEmailCaptchaBad        = 10185
	LoginMsgEmailCaptchaBad         = "The email verification code is incorrect"
	LoginCodeIPLocked               = 10296
	LoginMsgIPLocked                = "The IP address has been locked for too many login failures. Procedure Please try again in half an hour"
	LoginCodeUsernameExists         = 10075
	LoginMsgUsernameExists          = "The user name already exists. Please use another user name"
	LoginCodeUsernameDeletedUsed    = 10282
	LoginMsgUsernameDeletedUsed     = "The username has been used before, please use another username"
)

// Redis key prefixes mirror Java constants for IP-level lockout.
var (
	IPFailedLoginPrefix = "IP_FAILED_LOGIN_PREFIX:"
	IPLoginLockPrefix   = "IP_LOGIN_LOCK_PREFIX:"
)

// IP lockout thresholds (matches Java defaults).
const (
	IPFailedLoginMaxAttempts = 20
	IPLockDurationMinutes    = 30
)
