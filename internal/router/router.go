// Package router 注册所有 API 路由，并 serve 嵌入的前端静态文件。
package router

import (
	"io/fs"
	"net/http"
	"strings"

	"network-plan/internal/handler"
	"network-plan/internal/middleware"
	"network-plan/internal/store"

	"github.com/gin-gonic/gin"
)

// Setup 初始化 Gin 路由引擎，注册 API 和前端静态资源。
// frontendFS 为 go:embed 嵌入的前端构建产物（传 nil 则不 serve 前端）。
func Setup(
	poolRepo *store.PoolRepo,
	allocRepo *store.AllocRepo,
	auditRepo *store.AuditRepo,
	userRepo *store.UserRepo,
	tenantRepo *store.TenantRepo,
	jwtSecret string,
	gatewayToken string,
	frontendFS fs.FS,
) *gin.Engine {
	r := gin.Default()

	// --- Handler 实例化 ---
	poolH := handler.NewPoolHandler(poolRepo, allocRepo, auditRepo)
	allocH := handler.NewAllocationHandler(poolRepo, allocRepo, auditRepo)
	auditH := handler.NewAuditHandler(auditRepo)
	exportH := handler.NewExportHandler(allocRepo, auditRepo)
	dashH := handler.NewDashboardHandler(poolRepo, allocRepo)
	authH := handler.NewAuthHandler(userRepo, jwtSecret)
	tenantH := handler.NewTenantHandler(tenantRepo, userRepo)

	// --- 公开路由（无需认证）---
	r.POST("/api/login", authH.Login)
	r.GET("/api/tenants", tenantH.ListAll)

	// --- 需认证的 API 路由组 ---
	api := r.Group("/api")
	api.Use(middleware.JWTAuth(jwtSecret, gatewayToken))
	{
		// 当前用户
		api.GET("/me", authH.Me)

		// 当前用户可访问的租户列表
		api.GET("/my-tenants", tenantH.ListMyTenants)

		// 仪表盘（所有用户）
		api.GET("/dashboard", dashH.Get)

		// 网段池 — 查看（所有用户）
		api.GET("/pools", poolH.List)

		// 网段池 — 增删（仅管理员）
		admin := api.Group("")
		admin.Use(middleware.RequireAdmin())
		{
			admin.POST("/pools", poolH.Create)
			admin.DELETE("/pools/:id", poolH.Delete)
			admin.POST("/register", authH.Register)
			admin.POST("/tenants", tenantH.Create)
			admin.DELETE("/tenants/:slug", tenantH.Delete)
		}

		// 子网分配（所有用户）
		api.GET("/allocations", allocH.List)
		api.POST("/allocations", allocH.Allocate)
		api.POST("/allocations/batch", allocH.BatchAllocate)
		api.PUT("/allocations/:id", allocH.Update)
		api.DELETE("/allocations/:id", allocH.Reclaim)

		// 剩余网段查询（所有用户）
		api.GET("/pools/:id/free-blocks", allocH.FreeBlocks)

		// 预计算（所有用户）
		api.POST("/calculate", allocH.Calculate)

		// 审计日志（所有用户）
		api.GET("/audit", auditH.List)

		// 数据导出（所有用户）
		api.GET("/export", exportH.Export)
	}

	// --- 前端静态文件 ---
	if frontendFS != nil {
		// serve 嵌入的前端资源，所有非 /api 路径 fallback 到 index.html（SPA 路由）
		staticServer := http.FileServer(http.FS(frontendFS))
		r.NoRoute(func(c *gin.Context) {
			path := c.Request.URL.Path
			// 尝试打开静态文件（带 . 的路径视为静态资源）
			if strings.Contains(path, ".") {
				staticServer.ServeHTTP(c.Writer, c.Request)
				return
			}
			// 其余路径一律返回 index.html，交给前端路由处理
			c.Request.URL.Path = "/"
			staticServer.ServeHTTP(c.Writer, c.Request)
		})
	}

	return r
}
