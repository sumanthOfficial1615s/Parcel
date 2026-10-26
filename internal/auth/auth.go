package auth

import (
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
)

const tokenTTL = 24 * time.Hour

type Claims struct {
	Role                 string `json:"role"`
	jwt.RegisteredClaims        // Subject = user id
}

// NewToken creates a signed JWT for the given user.
func NewToken(secret, userID, role string) (string, error) {
	claims := Claims{
		Role: role,
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   userID,
			IssuedAt:  jwt.NewNumericDate(time.Now()),
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(tokenTTL)),
		},
	}

	return jwt.NewWithClaims(jwt.SigningMethodHS256, claims).
		SignedString([]byte(secret))
}

// Required is a Gin middleware that rejects requests without a valid
// "Authorization: Bearer <token>" header. On success it stores the
// user id and role on the context.
func Required(secret string) gin.HandlerFunc {
	return func(c *gin.Context) {
		header := c.GetHeader("Authorization")
		tokenString, found := strings.CutPrefix(header, "Bearer ")
		if !found || tokenString == "" {
			unauthorized(c, "missing bearer token")
			return
		}

		claims := &Claims{}
		token, err := jwt.ParseWithClaims(
			tokenString,
			claims,
			func(t *jwt.Token) (any, error) {
				return []byte(secret), nil
			},
			jwt.WithValidMethods([]string{"HS256"}),
			jwt.WithExpirationRequired(),
		)
		if err != nil || !token.Valid || claims.Subject == "" {
			unauthorized(c, "invalid or expired token")
			return
		}

		c.Set("user_id", claims.Subject)
		c.Set("role", claims.Role)
		c.Next()
	}
}

// AdminOnly must be used after Required.
func AdminOnly() gin.HandlerFunc {
	return func(c *gin.Context) {
		if Role(c) != "admin" {
			c.AbortWithStatusJSON(
				http.StatusForbidden,
				gin.H{"error": "admin access required"},
			)
			return
		}
		c.Next()
	}
}

func UserID(c *gin.Context) string { return c.GetString("user_id") }
func Role(c *gin.Context) string   { return c.GetString("role") }

func unauthorized(c *gin.Context, msg string) {
	c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": msg})
}

var ErrWeakSecret = errors.New("JWT_SECRET must be at least 32 characters")

// CheckSecret makes sure the server never starts with a weak signing key.
func CheckSecret(secret string) error {
	if len(secret) < 32 {
		return ErrWeakSecret
	}
	return nil
}
