// network-plan 主入口
// 启动流程：加载配置 → 连接数据库 → 注册路由 → 启动 HTTP 服务
// 支持子命令：./network-plan user add|list|passwd|delete
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
	// 检查是否有子命令
	if len(os.Args) > 1 && os.Args[1] == "user" {
		handleUserCommand()
		return
	}

	// --- 正常启动服务 ---
	cfg := config.Load()

	db, err := store.InitDB(cfg.DSN)
	if err != nil {
		log.Fatalf("Database init failed: %v", err)
	}
	log.Println("Database connected and migrated successfully")

	poolRepo := store.NewPoolRepo(db)
	allocRepo := store.NewAllocRepo(db)
	auditRepo := store.NewAuditRepo(db)
	userRepo := store.NewUserRepo(db)

	// 首次启动时自动创建默认管理员
	if count, _ := userRepo.Count(); count == 0 {
		hashedPw, _ := bcrypt.GenerateFromPassword([]byte("admin123"), bcrypt.DefaultCost)
		admin := &model.User{
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

	r := router.Setup(poolRepo, allocRepo, auditRepo, userRepo, cfg.JWTSecret, frontendFS)

	addr := fmt.Sprintf(":%s", cfg.Port)
	log.Printf("Server starting on http://localhost%s", addr)
	if err := r.Run(addr); err != nil {
		log.Fatalf("Server failed: %v", err)
	}
}

// handleUserCommand 处理 ./network-plan user <action> 子命令
func handleUserCommand() {
	if len(os.Args) < 3 {
		printUserUsage()
		os.Exit(1)
	}

	action := os.Args[2]

	// 解析 user 子命令的 flags（支持 -dsn）
	fs := flag.NewFlagSet("user", flag.ExitOnError)
	cfg := &config.Config{}
	cfg.RegisterFlags(fs)
	fs.Parse(os.Args[3:])
	cfg.ApplyEnv()
	args := fs.Args()

	db, err := store.InitDB(cfg.DSN)
	if err != nil {
		log.Fatalf("Database init failed: %v", err)
	}
	userRepo := store.NewUserRepo(db)

	switch action {
	case "add":
		if len(args) < 2 {
			fmt.Println("Usage: network-plan user add <username> <password> [--role admin|user] [-dsn ...]")
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
		user := &model.User{Username: username, PasswordHash: string(hashedPw), Role: role}
		if err := userRepo.Create(user); err != nil {
			log.Fatalf("Failed to create user: %v", err)
		}
		fmt.Printf("User '%s' created (ID: %d, role: %s)\n", username, user.ID, user.Role)

	case "list":
		users, err := userRepo.List()
		if err != nil {
			log.Fatalf("Failed to list users: %v", err)
		}
		w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
		fmt.Fprintln(w, "ID\tUsername\tRole\tCreatedAt")
		for _, u := range users {
			fmt.Fprintf(w, "%d\t%s\t%s\t%s\n", u.ID, u.Username, u.Role, u.CreatedAt.Format("2006-01-02 15:04:05"))
		}
		w.Flush()

	case "passwd":
		if len(args) < 2 {
			fmt.Println("Usage: network-plan user passwd <username> <new_password> [-dsn ...]")
			os.Exit(1)
		}
		username, newPassword := args[0], args[1]
		if len(newPassword) < 6 {
			log.Fatal("Password must be at least 6 characters")
		}
		// 确认用户存在
		if _, err := userRepo.FindByUsername(username); err != nil {
			log.Fatalf("User '%s' not found", username)
		}
		hashedPw, _ := bcrypt.GenerateFromPassword([]byte(newPassword), bcrypt.DefaultCost)
		if err := userRepo.UpdatePassword(username, string(hashedPw)); err != nil {
			log.Fatalf("Failed to update password: %v", err)
		}
		fmt.Printf("Password updated for user '%s'\n", username)

	case "delete":
		if len(args) < 1 {
			fmt.Println("Usage: network-plan user delete <username> [-dsn ...]")
			os.Exit(1)
		}
		username := args[0]
		if username == "admin" {
			log.Fatal("Cannot delete the default admin user")
		}
		if _, err := userRepo.FindByUsername(username); err != nil {
			log.Fatalf("User '%s' not found", username)
		}
		if err := userRepo.DeleteByUsername(username); err != nil {
			log.Fatalf("Failed to delete user: %v", err)
		}
		fmt.Printf("User '%s' deleted\n", username)

	default:
		printUserUsage()
		os.Exit(1)
	}
}

func printUserUsage() {
	fmt.Println(`Usage: network-plan user <command> [args] [-dsn ...]

Commands:
  add     <username> <password> [--role admin|user]  Create a new user (default role: user)
  list                                               List all users
  passwd  <username> <new_password>                  Change user password
  delete  <username>                                 Delete a user`)
}
