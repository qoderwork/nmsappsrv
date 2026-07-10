package constants

// Redis key prefixes used across multiple modules.
const (
	RedisKeyDeviceOnline = "device:online:"  // + serial_number
	RedisKeyJWTLogin     = "security:jwt:login:" // + username
	RedisKeyJWTBlack     = "security:jwt:black:" // + token
	RedisKeyHeartbeat    = "heartbeat:last:" // + serial_number
)

// Default pagination values.
const (
	DefaultPage     = 1
	DefaultPageSize = 20
)
