package handler

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
	"golang.org/x/crypto/bcrypt"

	"network-plan/internal/middleware"
	"network-plan/internal/model"
	"network-plan/internal/store"
)

// AuthHandler 处理用户认证相关接口
type AuthHandler struct {
	userRepo  *store.UserRepo
	jwtSecret []byte
}

func NewAuthHandler(ur *store.UserRepo, jwtSecret string) *AuthHandler {
	return &AuthHandler{userRepo: ur, jwtSecret: []byte(jwtSecret)}
}

// Login 用户登录，返回 JWT token
func (h *AuthHandler) Login(c *gin.Context) {
	var req struct {
		Username string `json:"username" binding:"required"`
		Password string `json:"password" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "用户名和密码不能为空"})
		return
	}

	// 从请求头或请求体获取租户标识
	tenantID := c.GetHeader("X-TENANT")
	if tenantID == "" {
		tenantID = "default"
	}

	user, err := h.userRepo.WithTenant(tenantID).FindByUsername(req.Username)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "用户名或密码错误"})
		return
	}

	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(req.Password)); err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "用户名或密码错误"})
		return
	}

	token, err := h.generateToken(user.ID, user.Username, user.Role, user.TenantID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "生成 token 失败"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"token": token,
		"user": gin.H{
			"id":        user.ID,
			"username":  user.Username,
			"role":      user.Role,
			"tenant_id": user.TenantID,
		},
	})
}

// Register 注册新用户（仅管理员可调用，由路由层中间件保证）
func (h *AuthHandler) Register(c *gin.Context) {
	var req struct {
		Username string `json:"username" binding:"required"`
		Password string `json:"password" binding:"required,min=6"`
		Role     string `json:"role"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "用户名不能为空，密码至少 6 位"})
		return
	}

	// 默认角色为普通用户
	role := req.Role
	if role != model.RoleAdmin && role != model.RoleUser {
		role = model.RoleUser
	}

	hashedPw, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "密码加密失败"})
		return
	}

	tenantID := middleware.GetTenantID(c)
	user := &model.User{
		TenantID:     tenantID,
		Username:     req.Username,
		PasswordHash: string(hashedPw),
		Role:         role,
	}
	if err := h.userRepo.WithTenant(tenantID).Create(user); err != nil {
		c.JSON(http.StatusConflict, gin.H{"error": "用户名已存在"})
		return
	}

	c.JSON(http.StatusCreated, gin.H{
		"id":        user.ID,
		"username":  user.Username,
		"role":      user.Role,
		"tenant_id": user.TenantID,
	})
}

// Me 获取当前登录用户信息
func (h *AuthHandler) Me(c *gin.Context) {
	userID, _ := c.Get("user_id")
	user, err := h.userRepo.FindByID(userID.(uint64))
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "用户不存在"})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"id":         user.ID,
		"username":   user.Username,
		"role":       user.Role,
		"created_at": user.CreatedAt,
	})
}

func (h *AuthHandler) generateToken(userID uint64, username, role, tenantID string) (string, error) {
	claims := &middleware.Claims{
		UserID:   userID,
		Username: username,
		Role:     role,
		TenantID: tenantID,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(24 * time.Hour)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
		},
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString(h.jwtSecret)
}
