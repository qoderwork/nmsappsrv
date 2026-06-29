package middleware

import (
	"context"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"

	"nmsappsrv/pkg/logger"
	"nmsappsrv/pkg/redis"
	"nmsappsrv/pkg/utils"
)

// JWTSecret is the signing key for JWT tokens. Override at startup if needed.
var JWTSecret = []byte("nmsappsrv-secret-key")

// Claims defines the payload carried inside each JWT.
type Claims struct {
	UserID    int    `json:"user_id"`
	Username  string `json:"username"`
	LicenseID int    `json:"license_id"`
	jwt.RegisteredClaims
}

// GenerateToken creates a signed JWT that is valid for 60 minutes.
// It also stores the token in Redis under SECURITY_JWT_LOGIN:{username}
// so that only the most recently issued token is valid.
func GenerateToken(userId int, username string, licenseId int) (string, error) {
	now := time.Now()
	claims := Claims{
		UserID:    userId,
		Username:  username,
		LicenseID: licenseId,
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
	loginKey := "security:jwt:login:" + username
	if err := redis.Set(ctx, loginKey, tokenString, 60*time.Minute); err != nil {
		logger.Warnf("GenerateToken: failed to store login key for %s: %v", username, err)
	}

	return tokenString, nil
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

		claims := &Claims{}
		token, err := jwt.ParseWithClaims(tokenString, claims, func(t *jwt.Token) (interface{}, error) {
			// Guard against algorithm confusion attacks.
			if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
				return nil, jwt.ErrSignatureInvalid
			}
			return JWTSecret, nil
		})

		if err != nil || !token.Valid {
			logger.Warnf("JWT validation failed: %v", err)
			utils.Error(c, http.StatusUnauthorized, "unauthorized")
			c.Abort()
			return
		}

		// Check JWT blacklist
		ctx := context.Background()
		blackKey := "security:jwt:black:" + tokenString
		if redis.Exists(ctx, blackKey) {
			logger.Warnf("JWT blacklisted for user %s", claims.Username)
			utils.Error(c, http.StatusUnauthorized, "token has been invalidated")
			c.Abort()
			return
		}

		// Check login key: only the most recently issued JWT is valid
		loginKey := "security:jwt:login:" + claims.Username
		storedToken, err := redis.Get(ctx, loginKey)
		if err != nil || storedToken == "" {
			// No login key means user has been logged out
			logger.Warnf("No login key for user %s", claims.Username)
			utils.Error(c, http.StatusUnauthorized, "session expired")
			c.Abort()
			return
		}
		if storedToken != tokenString {
			// This is not the most recent token — user has re-logged in
			logger.Warnf("JWT mismatch for user %s (re-login detected)", claims.Username)
			utils.Error(c, http.StatusUnauthorized, "session superseded by new login")
			c.Abort()
			return
		}

		// Inject user info into context for downstream handlers.
		c.Set("user_id", claims.UserID)
		c.Set("username", claims.Username)
		c.Set("license_id", claims.LicenseID)

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
