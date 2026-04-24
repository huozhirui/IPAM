package middleware

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
)

// Claims JWT 令牌中携带的用户信息
type Claims struct {
	UserID   uint64 `json:"user_id"`
	Username string `json:"username"`
	Role     string `json:"role"`
	jwt.RegisteredClaims
}

// JWTAuth 返回一个验证身份的 Gin 中间件。
// 优先解析 Authorization: Bearer <JWT>；
// 若无 JWT，则检查网关转发的 X-USER / X-ROLE 头部。
func JWTAuth(secret string) gin.HandlerFunc {
	return func(c *gin.Context) {
		// 1. 优先尝试 JWT
		authHeader := c.GetHeader("Authorization")
		if authHeader != "" && strings.HasPrefix(authHeader, "Bearer ") {
			tokenStr := strings.TrimPrefix(authHeader, "Bearer ")
			claims := &Claims{}
			token, err := jwt.ParseWithClaims(tokenStr, claims, func(t *jwt.Token) (interface{}, error) {
				return []byte(secret), nil
			})
			if err != nil || !token.Valid {
				c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "invalid or expired token"})
				return
			}
			c.Set("user_id", claims.UserID)
			c.Set("username", claims.Username)
			c.Set("role", claims.Role)
			c.Next()
			return
		}

		// 2. 回退：网关透传 X-USER / X-ROLE
		xUser := c.GetHeader("X-USER")
		if xUser != "" {
			xRole := c.GetHeader("X-ROLE")
			if xRole == "" {
				xRole = "user"
			}
			c.Set("username", xUser)
			c.Set("role", xRole)
			c.Next()
			return
		}

		c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "missing or invalid token"})
	}
}

// RequireAdmin 仅允许 admin 角色访问
func RequireAdmin() gin.HandlerFunc {
	return func(c *gin.Context) {
		role, _ := c.Get("role")
		if role != "admin" {
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"error": "需要管理员权限"})
			return
		}
		c.Next()
	}
}
