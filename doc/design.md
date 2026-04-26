# 技术设计方案

## 1. 技术选型

```
┌──────────────────────────────────────────────────────────┐
│                     Web 前端 (嵌入式)                      │
│               Vue 3 + Vite + Ant Design Vue               │
│   - 构建产物通过 Go embed 嵌入二进制                        │
│   - 单页应用 SPA，路由在前端完成                            │
│   - 响应式布局，支持桌面/平板访问                           │
│   - JWT Token 存储在 localStorage                          │
│   - Axios 拦截器自动附加 Token / 401 跳转登录              │
├──────────────────────────────────────────────────────────┤
│                     Go 后端 (Gin)                          │
│   - RESTful API 提供所有业务接口                           │
│   - JWT 中间件保护 API（登录接口除外）                      │
│   - 用户认证（bcrypt 密码哈希 + JWT 签发）                  │
│   - 网段计算引擎 (net/netip + 自定义算法)                   │
│   - 业务逻辑层 (分配/回收/冲突检测)                         │
│   - 静态文件服务 (serve 嵌入的前端资源)                     │
├──────────────────────────────────────────────────────────┤
│                     数据存储层                              │
│              MySQL / SQLite (自动切换)                      │
│   - 生产环境使用 MySQL，开发环境自动回退 SQLite             │
│   - GORM 作为 ORM，自动迁移表结构                          │
└──────────────────────────────────────────────────────────┘
```

**最终方案：Go (Gin + embed) + Vue 3 + MySQL/SQLite → 单二进制 All-in-One**

核心设计思路：
1. **All-in-One 单文件部署**：前端 Vue 构建产物通过 `go:embed` 打包进 Go 二进制，最终产出一个可执行文件，包含 Web 服务器 + API + 前端页面
2. **双数据库支持**：传入 MySQL DSN 则连接 MySQL；不传则自动使用本地 SQLite 文件 `ipam.db`，零配置即可运行
3. **零前端服务器**：Go 的 Gin 同时 serve API 和前端静态文件，无需 Nginx
4. **部署极简**：`./network-plan -dsn "user:pass@tcp(127.0.0.1:3306)/ipam"` 即可启动

## 2. 核心模块划分

```
network-plan/
├── cmd/                        # 入口
│   └── main.go                 # 启动：解析配置 → 连接 DB → 启动 HTTP
├── internal/
│   ├── config/                 # 配置管理
│   │   └── config.go           # MySQL DSN、端口、JWT 密钥、环境变量读取
│   ├── model/                  # 数据模型 (GORM Model)
│   │   ├── tenant.go           # 租户
│   │   ├── pool.go             # 网段池
│   │   ├── allocation.go       # 分配记录
│   │   ├── audit.go            # 操作日志
│   │   └── user.go             # 用户表
│   ├── ipam/                   # IP 地址管理核心算法
│   │   ├── calculator.go       # IP 数量 → CIDR 转换、子网计算
│   │   ├── allocator.go        # 空闲段查找、最佳匹配分配
│   │   └── validator.go        # 冲突检测、输入校验
│   ├── store/                  # 数据持久化 (MySQL/SQLite + GORM)
│   │   ├── db.go               # 数据库连接、AutoMigrate
│   │   ├── tenant_scope.go     # 多租户 GORM Scope
│   │   ├── tenant_repo.go      # 租户 CRUD
│   │   ├── pool_repo.go        # 网段池 CRUD
│   │   ├── alloc_repo.go       # 分配记录 CRUD
│   │   ├── audit_repo.go       # 日志写入/查询
│   │   └── user_repo.go        # 用户 CRUD
│   ├── middleware/              # HTTP 中间件
│   │   └── auth.go             # JWT 认证中间件
│   ├── handler/                # HTTP API 层 (Gin handlers)
│   │   ├── pool.go             # /api/pools/*
│   │   ├── allocation.go       # /api/allocations/*
│   │   ├── audit.go            # /api/audit/*
│   │   ├── dashboard.go        # /api/dashboard
│   │   ├── export.go           # /api/export (CSV/JSON)
│   │   ├── auth.go             # /api/login, /api/register, /api/me
│   │   └── tenant.go           # /api/tenants, /api/my-tenants
│   └── router/                 # 路由注册
│       └── router.go           # API 路由 + 前端静态文件 serve
├── web/                        # 前端项目 (Vue 3 + Vite)
│   ├── src/
│   │   ├── views/
│   │   │   ├── Login.vue       # 登录页面
│   │   │   ├── Dashboard.vue   # 首页仪表盘：网段池总览 + 使用率
│   │   │   ├── PoolManage.vue  # 网段池增删改查
│   │   │   ├── Allocate.vue    # 子网分配操作页
│   │   │   ├── FreeBlocks.vue  # 剩余可用网段查看
│   │   │   └── AuditLog.vue    # 操作日志
│   │   ├── components/         # 公共组件
│   │   ├── api/                # axios 封装，调后端 API
│   │   ├── App.vue
│   │   └── main.ts
│   ├── package.json
│   └── vite.config.ts
├── static/                     # go:embed 目标目录 (构建时从 web/dist 复制)
│   └── dist/                   # Vue 构建产物
├── doc/
│   ├── design.md               # 本文档：技术设计方案
│   └── need.md                 # 需求文档
├── Makefile                    # make build: 前端构建 → 后端编译 → 单二进制
├── go.mod
└── go.sum
```

## 3. 数据库设计

> **注意**：实际表结构由 GORM AutoMigrate 自动管理，以下 SQL 仅供参考。
> GORM model 中全大写字段（如 `CIDR`）需显式声明 `column:cidr`，否则 GORM 会将其转为 `c_id_r`。

```sql
CREATE DATABASE IF NOT EXISTS ipam DEFAULT CHARSET utf8mb4;
USE ipam;

-- 租户表
CREATE TABLE tenant (
    id          BIGINT UNSIGNED AUTO_INCREMENT PRIMARY KEY,
    name        VARCHAR(128) NOT NULL COMMENT '租户名称',
    slug        VARCHAR(64)  NOT NULL UNIQUE COMMENT '租户标识，如 "team-a"',
    created_at  DATETIME     NOT NULL DEFAULT CURRENT_TIMESTAMP,
    UNIQUE INDEX idx_slug (slug)
) ENGINE=InnoDB COMMENT='租户表';

-- 网段池
CREATE TABLE ip_pool (
    id          BIGINT UNSIGNED AUTO_INCREMENT PRIMARY KEY,
    tenant_id   VARCHAR(64)  NOT NULL DEFAULT 'default' COMMENT '所属租户',
    cidr        VARCHAR(100) NOT NULL COMMENT '网段 CIDR 或 "startIP - endIP" 显示串',
    start_ip    VARCHAR(43)  NOT NULL DEFAULT '' COMMENT '范围起始 IP',
    end_ip      VARCHAR(43)  NOT NULL DEFAULT '' COMMENT '范围结束 IP',
    name        VARCHAR(128) NOT NULL DEFAULT '' COMMENT '网段池名称',
    description VARCHAR(512) DEFAULT '' COMMENT '备注',
    created_at  DATETIME     NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at  DATETIME     NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    UNIQUE INDEX idx_tenant_cidr (tenant_id, cidr),
    INDEX idx_tenant_id (tenant_id)
) ENGINE=InnoDB COMMENT='网段池';

-- 子网分配记录
CREATE TABLE allocation (
    id            BIGINT UNSIGNED AUTO_INCREMENT PRIMARY KEY,
    tenant_id     VARCHAR(64)  NOT NULL DEFAULT 'default' COMMENT '所属租户',
    pool_id       BIGINT UNSIGNED NOT NULL COMMENT '所属网段池',
    cidr          VARCHAR(43)  NOT NULL COMMENT '分配的子网 CIDR',
    ip_count      INT UNSIGNED NOT NULL COMMENT '用户申请的 IP 数量',
    actual_count  INT UNSIGNED NOT NULL COMMENT '实际分配 IP 数 (2^n)',
    purpose       VARCHAR(256) NOT NULL COMMENT '用途标签：VPC / 业务线 / 环境',
    allocated_by  VARCHAR(128) DEFAULT '' COMMENT '负责人（默认当前登录用户）',
    allocated_at  DATETIME     NOT NULL DEFAULT CURRENT_TIMESTAMP,
    INDEX idx_pool_id (pool_id),
    UNIQUE INDEX idx_tenant_alloc_cidr (tenant_id, cidr),
    INDEX idx_tenant_id (tenant_id),
    CONSTRAINT fk_pool FOREIGN KEY (pool_id) REFERENCES ip_pool(id)
) ENGINE=InnoDB COMMENT='子网分配记录';

-- 操作审计日志
CREATE TABLE audit_log (
    id          BIGINT UNSIGNED AUTO_INCREMENT PRIMARY KEY,
    tenant_id   VARCHAR(64)  NOT NULL DEFAULT 'default' COMMENT '所属租户',
    action      VARCHAR(32)  NOT NULL COMMENT 'ALLOCATE / RECLAIM / CREATE_POOL / DELETE_POOL',
    detail      TEXT         DEFAULT NULL COMMENT '操作详情（JSON 格式字符串）',
    operator    VARCHAR(128) DEFAULT '',
    created_at  DATETIME     NOT NULL DEFAULT CURRENT_TIMESTAMP,
    INDEX idx_action (action),
    INDEX idx_created_at (created_at),
    INDEX idx_tenant_id (tenant_id)
) ENGINE=InnoDB COMMENT='操作审计日志';

-- 用户表
CREATE TABLE user (
    id            BIGINT UNSIGNED AUTO_INCREMENT PRIMARY KEY,
    tenant_id     VARCHAR(64)  NOT NULL DEFAULT 'default' COMMENT '所属租户',
    username      VARCHAR(64)  NOT NULL COMMENT '用户名',
    password_hash VARCHAR(255) NOT NULL COMMENT 'bcrypt 哈希密码',
    role          VARCHAR(16)  NOT NULL DEFAULT 'user' COMMENT '角色：admin / user',
    created_at    DATETIME     NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at    DATETIME     NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    UNIQUE INDEX idx_tenant_username (tenant_id, username),
    INDEX idx_tenant_id (tenant_id)
) ENGINE=InnoDB COMMENT='用户表';
```

### GORM Model 注意事项

- `audit_log.detail` 使用 `TEXT` 类型（非 `JSON`），兼容低版本 MySQL（< 5.7.8）
- `ip_pool.cidr` 和 `allocation.cidr` 的 GORM tag 必须包含 `column:cidr`，因为 Go 字段名 `CIDR`（全大写）会被 GORM 默认命名策略转为 `c_id_r`
- 网段池创建时通过 IP 范围重叠检测（而非 CIDR 字符串比较）防止重复，支持 CIDR 和 IP 范围两种模式的交叉检测
- 所有业务表的唯一索引均为 `(tenant_id, ...)` 复合索引，不同租户可拥有相同 CIDR 而互不冲突
- 所有 `tenant_id` 字段默认值为 `'default'`，确保存量数据平滑迁移

### 多租户数据隔离

系统通过 `tenant_id` 字段实现多租户数据隔离：

```
┌─────────────────────────────────────────────────┐
│                   请求入口                        │
│  JWT Claims 中的 tenant_id / X-TENANT 请求头     │
├─────────────────────────────────────────────────┤
│              middleware.GetTenantID()             │
│  从 gin.Context 提取 tenant_id（默认 "default"） │
├─────────────────────────────────────────────────┤
│            repo.WithTenant(tenantID)             │
│  返回带 GORM Scope 的 Repo 副本                  │
│  所有查询自动追加 WHERE tenant_id = ?            │
├─────────────────────────────────────────────────┤
│                   数据库                          │
│  ip_pool / allocation / audit_log / user         │
│  每张表的 tenant_id 索引确保查询性能              │
└─────────────────────────────────────────────────┘
```

**隔离机制：**

- **Store 层**：`TenantScope(tenantID)` 返回 GORM Scope，自动注入 `WHERE tenant_id = ?` 条件
- **Repo 层**：每个 Repo 提供 `WithTenant(tenantID)` 方法，返回一个绑定了租户 Scope 的新 Repo 实例
- **Handler 层**：所有业务 Handler 方法开头调用 `middleware.GetTenantID(c)` 获取租户 ID，然后通过 `repo.WithTenant(tenantID)` 创建隔离的 Repo 进行操作
- **写入保护**：创建记录时显式设置 `TenantID` 字段，确保数据归属正确

## 4. 核心算法：子网分配

系统支持两种分配方式：

### 方式一：按 IP 数量自动分配

```
用户输入: "需要 50 个 IP"

Step 1: IP 数量 → 掩码位
   50 → 向上取 2^n ≥ 50 → 64 = 2^6 → 掩码 = 32 - 6 = /26

Step 2: 计算目标网段池的空闲区间
   已分配: [10.1.0.0/26, 10.1.0.128/25]
   总池:   10.1.0.0/24
   空闲段: [10.1.0.64/26, 10.2.0.0/25, ...]  (通过集合差运算得出)

Step 3: 最佳匹配 (Best-Fit)
   从空闲段中找最小的、能容纳 /26 的段 → 10.1.0.64/26

Step 4: 冲突检测
   确认 10.1.0.64/26 与所有已分配段无重叠

Step 5: 写入数据库，返回结果
   → 分配成功: 10.1.0.64/26 (64 个 IP，含网络地址和广播地址)
```

### 方式二：指定 CIDR 地址分配

```
用户输入: "分配 10.1.0.64/26"

Step 1: 验证 CIDR 格式
   IPv4 + canonical form（如 10.1.0.65/26 会提示应使用 10.1.0.64/26）

Step 2: 验证 CIDR 在目标网段池范围内
   CIDR 模式池 → ValidateSubnetInPool 检查包含关系
   IP 范围模式池 → 转换为 uint32 范围后检查边界

Step 3: 冲突检测
   与池内已有分配逐一检查是否重叠

Step 4: 写入数据库，返回结果
   → 分配成功: 10.1.0.64/26 (64 个 IP)
```

### 批量分配

批量分配接口逐条处理每个请求（每条可独立选择按数量或按 CIDR），返回每条的独立结果：

```json
{
  "results": [
    {"index": 0, "success": true,  "allocation": {...}},
    {"index": 1, "success": false, "error": "CIDR 10.1.0.0/26 overlaps with existing allocation"},
    {"index": 2, "success": true,  "allocation": {...}}
  ],
  "total": 3,
  "success_count": 2,
  "fail_count": 1
}
```

成功项立即生效，失败项返回具体错误信息，前端可修改后单独重试失败项。

## 5. 认证与鉴权设计

系统支持两种认证模式，认证中间件按优先级依次尝试：

### 模式一：JWT Token（直接访问）

```
浏览器                    Go 后端 (Gin)                  数据库
  │                          │                            │
  │  POST /api/login         │                            │
  │  {username, password}    │                            │
  │  X-TENANT: team-a        │                            │
  │ ─────────────────────▶   │                            │
  │                          │  按租户+用户名查询 + bcrypt │
  │                          │ ──────────────────────────▶ │
  │                          │  ◀────────────────────────  │
  │                          │  签发 JWT (含 tenant_id)    │
  │  ◀─────────────────────  │                            │
  │  {token, user}           │                            │
  │                          │                            │
  │  GET /api/pools          │                            │
  │  Authorization: Bearer   │                            │
  │ ─────────────────────▶   │                            │
  │                          │  JWT 中间件验证签名+过期    │
  │                          │  ✓ → 注入 user_id/tenant_id│
  │                          │  ✗ → 返回 401              │
  │  ◀─────────────────────  │                            │
  │  {data} / {error: 401}   │                            │
```

### 模式二：网关透传（通过外部网关接入）

```
用户 → 外部网关（Nginx / API Gateway / BFE 等） → IPAM 后端
       │  已完成身份认证                          │
       │  添加 X-TOKEN / X-USER / X-ROLE / X-TENANT│
       │ ─────────────────────────────────────▶   │
       │                                          │
       │  中间件检测到 X-USER 头部                 │
       │  → 校验 X-TOKEN 匹配启动配置的 gateway-token│
       │  → 信任网关认证结果                       │
       │  → 注入 username / role / tenant_id       │
       │  → X-ROLE 缺省时默认为 "user"             │
       │  → X-TENANT 缺省时默认为 "default"        │
```

> **安全机制**：必须通过 `-gateway-token` 或 `IPAM_GATEWAY_TOKEN` 配置一个密钥，请求中的 `X-TOKEN` 必须匹配此密钥才能通过。未配置时网关模式禁用，直接返回 401。

### 认证中间件判定逻辑

```
请求进入
  │
  ├─ 有 Authorization: Bearer <token> ?
  │   ├─ YES → 验证 JWT 签名 + 过期时间
  │   │         ├─ 有效 → 注入 user_id / username / role / tenant_id → 放行
  │   │         └─ 无效 → 返回 401
  │   │
  │   └─ NO → 有 X-USER 头部 ?
  │            ├─ YES → 校验 X-TOKEN 是否匹配 gateway-token
  │            │         ├─ 匹配 → 读取 X-USER / X-ROLE / X-TENANT → 放行
  │            │         └─ 不匹配或未配置 → 返回 401
  │            └─ NO  → 返回 401
```

**关键实现：**

- **密码存储**：使用 `bcrypt` 哈希，`json:"-"` 确保密码 hash 不会泄露到 API 响应
- **Token 结构**：JWT Claims 包含 `user_id`、`username`、`role`、`tenant_id`、`exp`（过期时间）
- **权限划分**：管理员（admin）可增删网段池、管理用户；普通用户（user）可分配/回收/编辑子网、查看所有数据、导出
- **网关接入注意事项**：网关需确保外部请求不能伪造 `X-USER` / `X-ROLE` / `X-TENANT` 头部；后端通过 `X-TOKEN` 校验确保只有受信网关能使用透传模式
- **多租户隔离**：登录时通过 `X-TENANT` 头指定租户，JWT 自动携带 tenant_id，后续请求自动按租户过滤数据

## 6. RESTful API 设计

### 公开接口（无需认证）

| 方法 | 路径 | 说明 |
|------|------|------|
| `POST` | `/api/login` | 用户登录，返回 JWT Token |
| `GET` | `/api/tenants` | 获取所有租户列表（供登录页下拉用） |

### 需认证接口

| 方法 | 路径 | 权限 | 说明 |
|------|------|------|------|
| `GET` | `/api/me` | 所有用户 | 获取当前登录用户信息（含角色） |
| `GET` | `/api/my-tenants` | 所有用户 | 获取当前用户可访问的租户列表 |
| `GET` | `/api/dashboard` | 所有用户 | 仪表盘统计数据 |
| `GET` | `/api/pools` | 所有用户 | 获取所有网段池（含使用率统计） |
| `POST` | `/api/pools` | **管理员** | 新增网段池（支持 CIDR 和 IP 范围两种模式） |
| `DELETE` | `/api/pools/:id` | **管理员** | 删除网段池 |
| `POST` | `/api/register` | **管理员** | 添加新用户（可指定角色） |
| `POST` | `/api/tenants` | **管理员** | 创建租户 |
| `DELETE` | `/api/tenants/:slug` | **管理员** | 删除租户（default 不可删除） |
| `POST` | `/api/allocations` | 所有用户 | 分配子网（支持按数量或指定 CIDR） |
| `POST` | `/api/allocations/batch` | 所有用户 | 批量分配子网（逐条处理，返回每条结果） |
| `PUT` | `/api/allocations/:id` | 所有用户 | 编辑分配记录（用途、负责人） |
| `GET` | `/api/allocations?pool_id=X&cidr=Y&purpose=Z&allocated_by=W` | 所有用户 | 查询分配记录（支持多条件组合搜索，cidr/purpose 模糊匹配） |
| `DELETE` | `/api/allocations/:id` | 所有用户 | 回收子网 |
| `GET` | `/api/pools/:id/free-blocks` | 所有用户 | 查询剩余可用网段 |
| `POST` | `/api/calculate` | 所有用户 | 预计算推荐 CIDR |
| `GET` | `/api/audit` | 所有用户 | 查询操作日志 |
| `GET` | `/api/export?format=csv&type=allocation` | 所有用户 | 导出分配记录 |
| `GET` | `/api/export?format=csv&type=audit` | 所有用户 | 导出审计日志 |

## 7. Web 页面设计

### 页面结构 (6 个页面)

| 页面 | 路由 | 说明 |
|------|------|------|
| 登录页 | `/login` | 用户名密码登录（独立布局，无侧边栏） |
| 仪表盘 | `/` | 网段池总览 + 使用率进度条 + 最近分配记录 |
| 网段池管理 | `/pools` | 网段池增删 + 使用率展示 |
| 分配子网 | `/allocate` | 分配记录列表 + 弹窗分配（单条/批量）+ 编辑/回收 |
| 剩余查询 | `/free-blocks` | 按池查看空闲段列表 + 容量提示 |
| 操作日志 | `/audit` | 审计日志表格 + 筛选 + 导出 |

### 分配子网页面

- 主体为分配记录列表，显示所有池的分配记录（含"所属网段池"列）
- **搜索栏**：支持按网段池（下拉选择）、CIDR（模糊）、用途（模糊）、负责人（精确）多条件组合搜索，文本框支持回车搜索和清空自动刷新
- 点击"分配子网"按钮弹出 Modal 表单（宽 880px）
- **操作模式选择**：弹窗顶部先选择「单个分配」或「批量分配」，两种模式互斥展示
- **公共区域**：选择网段池，实时显示可用 IP 数量和使用率
- **单个分配**：支持「按数量」和「按 CIDR」两种分配方式
  - 按数量：输入 IP 数量，系统实时推荐 CIDR
  - 按 CIDR：直接输入 CIDR 地址（如 `10.0.1.0/24`），系统验证后分配
- **批量分配**：CSS Grid 表格布局，每行可独立选择分配方式（数量/CIDR）
  - 每行显示状态标记：待提交（灰）、成功（绿色背景+锁定）、失败（红色背景+错误信息）
  - 提交后成功行自动禁用编辑，失败行可修改后重试
  - 重新提交时自动跳过已成功的行，只提交未成功的记录
  - 底部显示汇总统计（共 X 条，成功 Y 条，失败 Z 条）
- 分配/回收后自动刷新记录列表和池统计数据

## 8. 关键依赖

| 依赖 | 用途 |
|------|------|
| `github.com/gin-gonic/gin` | HTTP 框架 |
| `gorm.io/gorm` | ORM 框架 |
| `gorm.io/driver/mysql` | MySQL 驱动 |
| `gorm.io/driver/sqlite` | SQLite 驱动（零配置 fallback） |
| `github.com/golang-jwt/jwt/v5` | JWT Token 签发与验证 |
| `golang.org/x/crypto/bcrypt` | 密码 bcrypt 哈希 |
| `net/netip` (标准库) | IP 地址和前缀计算 |
| `embed` (标准库) | 嵌入前端静态资源 |
| `Vue 3` + `Vite` | 前端框架 + 构建工具 |
| `Ant Design Vue` | UI 组件库 |
| `axios` | HTTP 请求 |

## 9. 构建流程

```makefile
# Makefile
.PHONY: build

build: build-frontend build-backend

build-frontend:
	cd web && npm install && npm run build
	rm -rf static/dist && cp -r web/dist static/dist

build-backend:
	go build -o network-plan ./cmd/main.go

# 最终产物: ./network-plan (单个二进制文件，约 15-20MB)
```

### 租户权限模型

用户的租户访问权限通过**同名用户多租户注册**实现，无需额外的关联表：

```
┌───────────────────────────────────────────────────────┐
│  user 表 (tenant_id, username 复合唯一索引)            │
├────────────┬───────────┬──────────────────────────────┤
│ tenant_id  │ username  │ 说明                          │
├────────────┼───────────┼──────────────────────────────┤
│ default    │ admin     │ admin 可访问 default          │
│ team-a     │ admin     │ admin 可访问 team-a           │
│ team-a     │ alice     │ alice 只能访问 team-a         │
│ team-b     │ admin     │ admin 可访问 team-b           │
└────────────┴───────────┴──────────────────────────────┘

查询 admin 可访问的租户：
SELECT DISTINCT tenant_id FROM user WHERE username = 'admin'
→ [default, team-a, team-b]

查询 alice 可访问的租户：
SELECT DISTINCT tenant_id FROM user WHERE username = 'alice'
→ [team-a]
```

**授权操作**：在目标租户中创建同名用户即完成授权，删除对应记录即撤销授权。

```bash
# 授权：让 alice 也能访问 team-b
./network-plan user add alice pass123 --tenant team-b

# 撤销：移除 alice 对 team-b 的访问权限
./network-plan user delete alice --tenant team-b
```

## 10. 后续可扩展方向

1. **与云平台对接**：自动同步 AWS/阿里云/百度云 VPC 实际网段使用情况
2. **IPAM 标准协议**：对接 NetBox 等开源 IPAM 系统
3. **告警**：网段池使用率超过阈值时通知
4. **IaC 集成**：供 Terraform 等工具通过现有 API 自动化调用分配
