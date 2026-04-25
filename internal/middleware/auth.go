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
	TenantID string `json:"tenant_id"`
	jwt.RegisteredClaims
}

// JWTAuth 返回一个验证身份的 Gin 中间件。
// 优先解析 Authorization: Bearer <JWT>；
// 若无 JWT，则检查网关转发的 X-USER / X-ROLE 头部（需 X-TOKEN 匹配 gatewayToken）。
// gatewayToken 为空时禁用网关透传模式。
func JWTAuth(secret, gatewayToken string) gin.HandlerFunc {
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
			c.Set("tenant_id", claims.TenantID)
			// 允许 X-TENANT 头覆盖 JWT 中的 tenant_id（用于前端切换租户）
			if xTenant := c.GetHeader("X-TENANT"); xTenant != "" {
				c.Set("tenant_id", xTenant)
			}
			c.Next()
			return
		}

		// 2. 回退：网关透传 X-USER / X-ROLE / X-TENANT（需 X-TOKEN 校验）
		xUser := c.GetHeader("X-USER")
		if xUser != "" {
			if gatewayToken == "" {
				c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "gateway mode is disabled (no gateway-token configured)"})
				return
			}
			if c.GetHeader("X-TOKEN") != gatewayToken {
				c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "invalid X-TOKEN"})
				return
			}
			xRole := c.GetHeader("X-ROLE")
			if xRole == "" {
				xRole = "user"
			}
			xTenant := c.GetHeader("X-TENANT")
			if xTenant == "" {
				xTenant = "default"
			}
			c.Set("username", xUser)
			c.Set("role", xRole)
			c.Set("tenant_id", xTenant)
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

// GetTenantID 从 Gin 上下文中获取当前租户 ID，未设置时返回 "default"
func GetTenantID(c *gin.Context) string {
	if tid, ok := c.Get("tenant_id"); ok {
		if s, ok := tid.(string); ok && s != "" {
			return s
		}
	}
	return "default"
}
