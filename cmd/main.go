// network-plan 主入口
// 启动流程：加载配置 → 连接数据库 → 注册路由 → 启动 HTTP 服务
// 支持子命令：./network-plan [全局flags] user add|list|passwd|delete
//              ./network-plan [全局flags] tenant add|list|delete
package main

import (
	"flag"
	"fmt"
	"io/fs"
	"log"
	"os"
	"text/tabwriter"

	"network-plan/internal/config"
	"network-plan/internal/model"
	"network-plan/internal/router"
	"network-plan/internal/store"
	staticFS "network-plan/static"

	"golang.org/x/crypto/bcrypt"
)

func main() {
	// 先解析全局 flags（-dsn, -port, -jwt-secret, -gateway-token）
	cfg := &config.Config{}
	cfg.RegisterFlags(flag.CommandLine)
	flag.Parse()
	cfg.ApplyEnv()

	// flag.Args() 返回 flags 之后的剩余参数，即子命令部分
	args := flag.Args()

	if len(args) > 0 {
		switch args[0] {
		case "user":
			handleUserCommand(cfg, args[1:])
			return
		case "tenant":
			handleTenantCommand(cfg, args[1:])
			return
		}
	}

	// --- 正常启动服务 ---
	db, err := store.InitDB(cfg.DSN)
	if err != nil {
		log.Fatalf("Database init failed: %v", err)
	}
	log.Println("Database connected and migrated successfully")

	poolRepo := store.NewPoolRepo(db)
	allocRepo := store.NewAllocRepo(db)
	auditRepo := store.NewAuditRepo(db)
	userRepo := store.NewUserRepo(db)
	tenantRepo := store.NewTenantRepo(db)

	// 首次启动时自动创建默认租户和管理员
	db.FirstOrCreate(&model.Tenant{Slug: "default"}, &model.Tenant{
		Name: "Default",
		Slug: "default",
	})

	if count, _ := userRepo.Count(); count == 0 {
		hashedPw, _ := bcrypt.GenerateFromPassword([]byte("admin123"), bcrypt.DefaultCost)
		admin := &model.User{
			TenantID:     "default",
			Username:     "admin",
			PasswordHash: string(hashedPw),
			Role:         model.RoleAdmin,
		}
		if err := userRepo.Create(admin); err != nil {
			log.Printf("Warning: failed to seed admin user: %v", err)
		} else {
			log.Println("Seeded default admin user (admin / admin123) — CHANGE PASSWORD IN PRODUCTION")
		}
	}

	frontendFS, err := fs.Sub(staticFS.FrontendFS, "dist")
	if err != nil {
		log.Printf("Warning: frontend assets not found, API-only mode: %v", err)
		frontendFS = nil
	}

	r := router.Setup(poolRepo, allocRepo, auditRepo, userRepo, tenantRepo, cfg.JWTSecret, cfg.GatewayToken, frontendFS)

	addr := fmt.Sprintf(":%s", cfg.Port)
	log.Printf("Server starting on http://localhost%s", addr)
	if err := r.Run(addr); err != nil {
		log.Fatalf("Server failed: %v", err)
	}
}

// handleUserCommand 处理 ./network-plan [flags] user <action> [args]
func handleUserCommand(cfg *config.Config, args []string) {
	if len(args) < 1 {
		printUserUsage()
		os.Exit(1)
	}

	action := args[0]
	args = args[1:]

	db, err := store.InitDB(cfg.DSN)
	if err != nil {
		log.Fatalf("Database init failed: %v", err)
	}
	userRepo := store.NewUserRepo(db)

	// 解析 --tenant 参数（默认 "default"）
	tenant := "default"
	var cleanArgs []string
	for i := 0; i < len(args); i++ {
		if args[i] == "--tenant" && i+1 < len(args) {
			tenant = args[i+1]
			i++ // skip value
		} else {
			cleanArgs = append(cleanArgs, args[i])
		}
	}
	args = cleanArgs
	scopedUserRepo := userRepo.WithTenant(tenant)

	switch action {
	case "add":
		if len(args) < 2 {
			fmt.Println("Usage: network-plan user add <username> <password> [--role admin|user] [--tenant slug] [-dsn ...]")
			os.Exit(1)
		}
		username, password := args[0], args[1]
		if len(password) < 6 {
			log.Fatal("Password must be at least 6 characters")
		}
		// 解析 --role 参数
		role := model.RoleUser
		for i, a := range args {
			if a == "--role" && i+1 < len(args) {
				if args[i+1] == model.RoleAdmin {
					role = model.RoleAdmin
				}
			}
		}
		hashedPw, _ := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
		user := &model.User{TenantID: tenant, Username: username, PasswordHash: string(hashedPw), Role: role}
		if err := scopedUserRepo.Create(user); err != nil {
			log.Fatalf("Failed to create user: %v", err)
		}
		fmt.Printf("User '%s' created (ID: %d, role: %s, tenant: %s)\n", username, user.ID, user.Role, tenant)

	case "list":
		users, err := scopedUserRepo.List()
		if err != nil {
			log.Fatalf("Failed to list users: %v", err)
		}
		w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
		fmt.Fprintln(w, "ID\tUsername\tRole\tTenant\tCreatedAt")
		for _, u := range users {
			fmt.Fprintf(w, "%d\t%s\t%s\t%s\t%s\n", u.ID, u.Username, u.Role, u.TenantID, u.CreatedAt.Format("2006-01-02 15:04:05"))
		}
		w.Flush()

	case "passwd":
		if len(args) < 2 {
			fmt.Println("Usage: network-plan user passwd <username> <new_password> [--tenant slug] [-dsn ...]")
			os.Exit(1)
		}
		username, newPassword := args[0], args[1]
		if len(newPassword) < 6 {
			log.Fatal("Password must be at least 6 characters")
		}
		// 确认用户存在
		if _, err := scopedUserRepo.FindByUsername(username); err != nil {
			log.Fatalf("User '%s' not found in tenant '%s'", username, tenant)
		}
		hashedPw, _ := bcrypt.GenerateFromPassword([]byte(newPassword), bcrypt.DefaultCost)
		if err := scopedUserRepo.UpdatePassword(username, string(hashedPw)); err != nil {
			log.Fatalf("Failed to update password: %v", err)
		}
		fmt.Printf("Password updated for user '%s' (tenant: %s)\n", username, tenant)

	case "delete":
		if len(args) < 1 {
			fmt.Println("Usage: network-plan user delete <username> [--tenant slug] [-dsn ...]")
			os.Exit(1)
		}
		username := args[0]
		if username == "admin" && tenant == "default" {
			log.Fatal("Cannot delete the default admin user")
		}
		if _, err := scopedUserRepo.FindByUsername(username); err != nil {
			log.Fatalf("User '%s' not found in tenant '%s'", username, tenant)
		}
		if err := scopedUserRepo.DeleteByUsername(username); err != nil {
			log.Fatalf("Failed to delete user: %v", err)
		}
		fmt.Printf("User '%s' deleted (tenant: %s)\n", username, tenant)

	default:
		printUserUsage()
		os.Exit(1)
	}
}

func printUserUsage() {
	fmt.Println(`Usage: network-plan [-dsn ...] user <command> [args] [--tenant slug]

Commands:
  add     <username> <password> [--role admin|user]  Create a new user (default role: user)
  list                                               List all users
  passwd  <username> <new_password>                  Change user password
  delete  <username>                                 Delete a user

Options:
  --tenant slug   Tenant slug (default: "default")
  -dsn string     Database DSN (global flag, must be placed before subcommand)`)
}

// handleTenantCommand 处理 ./network-plan [flags] tenant <action> [args]
func handleTenantCommand(cfg *config.Config, args []string) {
	if len(args) < 1 {
		printTenantUsage()
		os.Exit(1)
	}

	action := args[0]
	args = args[1:]

	db, err := store.InitDB(cfg.DSN)
	if err != nil {
		log.Fatalf("Database init failed: %v", err)
	}
	tenantRepo := store.NewTenantRepo(db)

	switch action {
	case "add":
		if len(args) < 1 {
			fmt.Println("Usage: network-plan [-dsn ...] tenant add <slug> [--name display_name]")
			os.Exit(1)
		}
		slug := args[0]
		name := slug
		for i, a := range args {
			if a == "--name" && i+1 < len(args) {
				name = args[i+1]
			}
		}
		t := &model.Tenant{Name: name, Slug: slug}
		if err := tenantRepo.Create(t); err != nil {
			log.Fatalf("Failed to create tenant: %v", err)
		}
		fmt.Printf("Tenant '%s' created (ID: %d, name: %s)\n", slug, t.ID, name)

	case "list":
		tenants, err := tenantRepo.ListAll()
		if err != nil {
			log.Fatalf("Failed to list tenants: %v", err)
		}
		w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
		fmt.Fprintln(w, "ID\tSlug\tName\tCreatedAt")
		for _, t := range tenants {
			fmt.Fprintf(w, "%d\t%s\t%s\t%s\n", t.ID, t.Slug, t.Name, t.CreatedAt.Format("2006-01-02 15:04:05"))
		}
		w.Flush()

	case "delete":
		if len(args) < 1 {
			fmt.Println("Usage: network-plan [-dsn ...] tenant delete <slug>")
			os.Exit(1)
		}
		slug := args[0]
		if slug == "default" {
			log.Fatal("Cannot delete the default tenant")
		}
		if _, err := tenantRepo.FindBySlug(slug); err != nil {
			log.Fatalf("Tenant '%s' not found", slug)
		}
		if err := tenantRepo.DeleteBySlug(slug); err != nil {
			log.Fatalf("Failed to delete tenant: %v", err)
		}
		fmt.Printf("Tenant '%s' deleted\n", slug)

	default:
		printTenantUsage()
		os.Exit(1)
	}
}

func printTenantUsage() {
	fmt.Println(`Usage: network-plan [-dsn ...] tenant <command> [args]

Commands:
  add     <slug> [--name display_name]  Create a new tenant
  list                                  List all tenants
  delete  <slug>                        Delete a tenant (cannot delete "default")

Options:
  -dsn string   Database DSN (global flag, must be placed before subcommand)`)
}
