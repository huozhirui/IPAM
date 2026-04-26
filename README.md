# IPAM — IP 网段规划管理系统

轻量级 IP 网段规划管理工具，帮助运维团队高效完成子网划分、余量查看和分配记录持久化。

Go + Vue 3 单二进制 All-in-One 部署，无需额外依赖。

## 功能特性

- **网段池管理** — 支持 CIDR（如 `10.0.0.0/16`）和 IP 范围（如 `10.0.0.0 - 10.0.255.255`）两种定义方式，自动检测 IP 范围重叠
- **智能子网分配** — 支持两种分配方式：按 IP 数量自动计算最优 CIDR，或直接指定 CIDR 地址分配；支持单条和批量分配，批量分配逐条返回结果，失败项可修改后单独重试
- **分配记录搜索** — 支持按网段池、CIDR（模糊）、用途（模糊）、负责人多条件组合搜索
- **剩余网段查询** — 可视化展示各网段池的使用率和剩余可用段
- **操作审计** — 记录所有分配、回收、增删池操作，支持导出 CSV/JSON
- **用户认证** — JWT Token 登录 + 网关透传（X-USER/X-ROLE）双模式
- **角色权限** — 管理员可增删网段池、管理用户；普通用户可分配/回收子网
- **多租户隔离** — 通过 `tenant_id` 实现数据隔离，不同租户可拥有相同 CIDR 互不冲突

## 快速启动

### 零配置启动（SQLite）

```bash
./network-plan
# 自动使用本地 SQLite 文件 ipam.db，无需任何数据库配置
# 访问 http://localhost:8080
# 默认管理员: admin / admin123
```

### 使用 MySQL

```bash
# 方式一：命令行参数
./network-plan -dsn "user:pass@tcp(127.0.0.1:3306)/ipam?charset=utf8mb4&parseTime=True"

# 方式二：环境变量
export IPAM_DSN="user:pass@tcp(127.0.0.1:3306)/ipam?charset=utf8mb4&parseTime=True"
./network-plan
```

> **DSN 格式**：`用户名:密码@tcp(地址:端口)/数据库名?charset=utf8mb4&parseTime=True`
>
> 注意 `@tcp(...)` 中的 `tcp` 不能省略，用户名和密码之间用 `:` 分隔。

### 配置项

| 配置项 | CLI 参数 | 环境变量 | 默认值 | 说明 |
|--------|----------|----------|--------|------|
| 数据库连接 | `-dsn` | `IPAM_DSN` | 空（使用 SQLite） | MySQL DSN 连接串 |
| 监听端口 | `-port` | `IPAM_PORT` | `8080` | HTTP 服务端口 |
| JWT 密钥 | `-jwt-secret` | `IPAM_JWT_SECRET` | `change-me-in-production` | JWT 签名密钥 |
| 网关 Token | `-gateway-token` | `IPAM_GATEWAY_TOKEN` | 空（禁用网关模式） | 网关透传 X-TOKEN 校验密钥 |

优先级：命令行参数 > 环境变量 > 默认值

## CLI 用户管理

内置 `user` 子命令，无需启动 Web 服务即可管理用户：

```bash
# 新增用户（默认 role=user，默认 tenant=default）
./network-plan user add <用户名> <密码>

# 新增管理员
./network-plan user add <用户名> <密码> --role admin

# 指定租户创建用户
./network-plan user add <用户名> <密码> --tenant team-a

# 列出所有用户（当前租户下）
./network-plan user list
./network-plan user list --tenant team-a

# 修改密码
./network-plan user passwd <用户名> <新密码>

# 删除用户（default 租户的 admin 不可删除）
./network-plan user delete <用户名>

# 使用 MySQL 时加 -dsn 参数
./network-plan user list -dsn "user:pass@tcp(127.0.0.1:3306)/ipam?charset=utf8mb4&parseTime=True"
```

## CLI 租户管理

内置 `tenant` 子命令，管理多租户：

```bash
# 创建租户
./network-plan tenant add team-a
./network-plan tenant add team-a --name "团队 A"

# 查看所有租户
./network-plan tenant list

# 删除租户（default 租户不可删除）
./network-plan tenant delete team-a
```

## 多租户使用指南

系统通过 `tenant_id` 字段实现多租户数据隔离，不同租户的网段池、分配记录、审计日志、用户完全独立。

### 完整操作流程

```bash
# 第一步：创建租户
./network-plan tenant add team-a --name "团队 A"
./network-plan tenant add team-b --name "团队 B"

# 第二步：在租户下创建用户
./network-plan user add alice pass123 --tenant team-a --role admin
./network-plan user add bob   pass123 --tenant team-a

# 第三步：给用户授权多个租户的访问权限
# 权限模型：同一用户名在哪些租户中存在，就有哪些租户的权限
# 例如让 alice 也能访问 team-b：在 team-b 中创建同名用户
./network-plan user add alice pass123 --tenant team-b --role admin

# 此时 alice 登录后可在 Header 下拉切换 team-a 和 team-b
# bob 只能看到 team-a
```

### 认证与切换方式

- **前端登录**：登录页下拉选择租户，登录后 Header 下拉只显示当前用户有权限的租户
- **JWT 登录**：登录时通过 `X-TENANT` 请求头指定租户，JWT Token 自动携带 `tenant_id`
- **网关模式**：通过 `X-TENANT` 头传递租户标识
- **CLI**：`--tenant` 参数指定操作的租户

### API 管理租户（管理员）

```bash
# 创建租户（需管理员 Token）
curl -X POST http://localhost:8080/api/tenants \
  -H "Authorization: Bearer <admin_token>" \
  -H "Content-Type: application/json" \
  -d '{"name":"团队 A","slug":"team-a"}'

# 查看所有租户（公开接口，无需认证）
curl http://localhost:8080/api/tenants

# 查看当前用户可访问的租户（需认证）
curl -H "Authorization: Bearer <token>" http://localhost:8080/api/my-tenants

# 删除租户（需管理员 Token，default 不可删除）
curl -X DELETE http://localhost:8080/api/tenants/team-a \
  -H "Authorization: Bearer <admin_token>"
```

## 网关接入

支持通过外部网关（Nginx / API Gateway）透传已认证用户信息。

> **安全机制**：网关模式需要启动时配置 `-gateway-token`，请求必须携带 `X-TOKEN` 头且值匹配才能通过，防止外部用户伪造 `X-USER` 头绕过认证。未配置 `-gateway-token` 时网关模式处于禁用状态。

```bash
# 启动时配置网关 Token
./network-plan -gateway-token "my-secret-token"

# 或使用环境变量
export IPAM_GATEWAY_TOKEN="my-secret-token"
./network-plan
```

Nginx 配置示例：

```nginx
location /api/ {
    proxy_set_header X-TOKEN  my-secret-token;     # 必须，校验密钥
    proxy_set_header X-USER   $authenticated_user;
    proxy_set_header X-ROLE   $user_role;           # 可选，默认 user
    proxy_set_header X-TENANT $tenant_slug;         # 可选，默认 default
    proxy_pass http://127.0.0.1:8080;
}
```

```bash
# curl 测试（需携带 X-TOKEN）
curl -H "X-TOKEN: my-secret-token" \
     -H "X-USER: zhangsan" \
     -H "X-ROLE: admin" \
     -H "X-TENANT: team-a" \
     http://localhost:8080/api/pools
```

## 从源码构建

依赖：Go 1.23+、Node.js 18+

```bash
# 前端构建
cd web && npm install && npm run build && cd ..
rm -rf static/dist && cp -r web/dist static/dist

# 后端编译
go build -o network-plan ./cmd/main.go

# 或使用 Makefile
make build
```

## 文档

- [技术设计方案](doc/design.md) — 架构设计、数据库设计、API 接口、认证方案等
- [需求文档](doc/need.md) — 产品需求规格
