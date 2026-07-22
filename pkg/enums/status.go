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
// by the global exception handler into Result.code=100xx, Result.msg=lookup.
// These MUST match the codes in MyAuthenticationProvider / Status.
var (
	LoginCodeEncryptionFailure      = 10044
	LoginMsgEncryptionFailure       = "Encryption failure"
	LoginCodeUserDisabled           = 10045
	LoginMsgUserDisabled            = "The current user has been locked"
	LoginCodeBadCredentials         = 10047
	LoginMsgBadCredentials          = "The user name and password do not match"
	LoginCodeUserLocked             = 10048
	LoginMsgUserLocked              = "The account is locked due to too many failed login attempts"
	LoginCodeNoRole                 = 10162
	LoginMsgNoRole                  = "No role assigned, contact your administrator"
	LoginCodeInactive               = 10072
	LoginMsgInactive                = "The account is inactive due to prolonged inactivity, contact your administrator"
	LoginCodeIPLocked               = 10296
	LoginMsgIPLocked                = "The IP address has been locked due to too many failed login attempts"
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
