package middleware

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"

	"nmsappsrv/pkg/constants"
	"nmsappsrv/pkg/logger"
	"nmsappsrv/pkg/redis"
	"nmsappsrv/pkg/utils"
)

// JWTSecret is the signing key for JWT tokens. Must be set via SetJWTSecret
// at startup from configuration. Must be >=32 bytes.
var JWTSecret []byte

// SetJWTSecret sets the JWT signing key from configuration.
// The secret must be at least 32 bytes; otherwise the function panics
// to prevent running with a weak key.
func SetJWTSecret(secret string) {
	if len(secret) < 32 {
		logger.Fatalf("JWT secret must be at least 32 bytes, got %d bytes", len(secret))
	}
	JWTSecret = []byte(secret)
	logger.Info("JWT secret loaded from configuration")
}

// Claims defines the payload carried inside each JWT.
// Aligned with Java: includes roleNames and ssoType fields.
type Claims struct {
	UserID    int      `json:"user_id"`
	Username  string   `json:"username"`
	LicenseID int      `json:"license_id"`
	RoleNames []string `json:"role_names"`
	SsoType   string   `json:"sso_type"`
	jwt.RegisteredClaims
}

// GenerateToken creates a signed JWT that is valid for 60 minutes.
// It also stores the token in Redis under SECURITY_JWT_LOGIN:{username}
// so that only the most recently issued token is valid.
func GenerateToken(userId int, username string, licenseId int, roleNames []string, ssoType string) (string, error) {
	now := time.Now()
	claims := Claims{
		UserID:    userId,
		Username:  username,
		LicenseID: licenseId,
		RoleNames: roleNames,
		SsoType:   ssoType,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(now.Add(60 * time.Minute)),
			IssuedAt:  jwt.NewNumericDate(now),
			NotBefore: jwt.NewNumericDate(now),
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	tokenString, err := token.SignedString(JWTSecret)
	if err != nil {
		return "", err
	}

	// Store in Redis: SECURITY_JWT_LOGIN:{username} = token with 60min TTL
	ctx := context.Background()
	loginKey := constants.RedisKeyJWTLogin + username
	if err := redis.Set(ctx, loginKey, tokenString, 60*time.Minute); err != nil {
		logger.Warnf("GenerateToken: failed to store login key for %s: %v", username, err)
	}

	return tokenString, nil
}

// ValidateToken parses and fully validates a raw JWT string (signature,
// expiry, blacklist, and the single-session login key) and returns the decoded
// Claims. It is shared by AuthMiddleware (HTTP) and the WebSocket handler,
// which receives the token via a query parameter because browsers cannot set
// custom headers on the WebSocket handshake.
func ValidateToken(tokenString string) (*Claims, error) {
	claims := &Claims{}
	token, err := jwt.ParseWithClaims(tokenString, claims, func(t *jwt.Token) (interface{}, error) {
		// Guard against algorithm confusion attacks.
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, jwt.ErrSignatureInvalid
		}
		return JWTSecret, nil
	})
	if err != nil || !token.Valid {
		return nil, fmt.Errorf("invalid token: %w", err)
	}

	ctx := context.Background()

	// Check JWT blacklist
	blackKey := constants.RedisKeyJWTBlack + tokenString
	if redis.Exists(ctx, blackKey) {
		return nil, fmt.Errorf("token has been invalidated")
	}

	// Check login key: only the most recently issued JWT is valid
	loginKey := constants.RedisKeyJWTLogin + claims.Username
	storedToken, err := redis.Get(ctx, loginKey)
	if err != nil || storedToken == "" {
		// No login key means user has been logged out
		return nil, fmt.Errorf("session expired")
	}
	if storedToken != tokenString {
		// This is not the most recent token — user has re-logged in
		return nil, fmt.Errorf("session superseded by new login")
	}

	return claims, nil
}

// AuthMiddleware returns a gin.HandlerFunc that validates the JWT found in
// the "Authorization: Bearer <token>" header and injects user info into the
// gin.Context so downstream handlers can retrieve it via the Get* helpers.
func AuthMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		authHeader := c.GetHeader("Authorization")
		if authHeader == "" {
			utils.Error(c, http.StatusUnauthorized, "unauthorized")
			c.Abort()
			return
		}

		// Expect "Bearer <token>"
		parts := strings.SplitN(authHeader, " ", 2)
		if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") {
			utils.Error(c, http.StatusUnauthorized, "unauthorized")
			c.Abort()
			return
		}

		tokenString := strings.TrimSpace(parts[1])

		claims, err := ValidateToken(tokenString)
		if err != nil {
			logger.Warnf("JWT validation failed: %v", err)
			utils.Error(c, http.StatusUnauthorized, "unauthorized")
			c.Abort()
			return
		}

		// Inject user info into context for downstream handlers.
		c.Set("user_id", claims.UserID)
		c.Set("username", claims.Username)
		c.Set("license_id", claims.LicenseID)
		c.Set("role_names", claims.RoleNames)
		c.Set("sso_type", claims.SsoType)

		c.Next()
	}
}

// GetUserId extracts the authenticated user's ID from the gin context.
// Returns 0 if the value is missing or has an unexpected type.
func GetUserId(c *gin.Context) int {
	v, ok := c.Get("user_id")
	if !ok {
		return 0
	}
	id, ok := v.(int)
	if !ok {
		return 0
	}
	return id
}

// GetUsername extracts the authenticated user's username from the gin context.
// Returns an empty string if the value is missing or has an unexpected type.
func GetUsername(c *gin.Context) string {
	v, ok := c.Get("username")
	if !ok {
		return ""
	}
	s, ok := v.(string)
	if !ok {
		return ""
	}
	return s
}

// GetLicenseId extracts the authenticated user's license ID from the gin context.
// Returns 0 if the value is missing or has an unexpected type.
func GetLicenseId(c *gin.Context) int {
	v, ok := c.Get("license_id")
	if !ok {
		return 0
	}
	id, ok := v.(int)
	if !ok {
		return 0
	}
	return id
}

// GetRoleNames extracts the authenticated user's role names from the gin context.
// Returns an empty string if the value is missing or has an unexpected type.
func GetRoleNames(c *gin.Context) []string {
	v, ok := c.Get("role_names")
	if !ok {
		return nil
	}
	names, ok := v.([]string)
	if !ok {
		return nil
	}
	return names
}

// GetSsoType extracts the authenticated user's SSO type from the gin context.
// Returns an empty string if the value is missing or has an unexpected type.
func GetSsoType(c *gin.Context) string {
	v, ok := c.Get("sso_type")
	if !ok {
		return ""
	}
	s, ok := v.(string)
	if !ok {
		return ""
	}
	return s
}
