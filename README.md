# IP 网段规划管理系统 — 需求与设计文档

## 一、项目背景

运维团队日常管理大量 IP 网段资源，在为不同 VPC / 业务分配子网时，经常面临以下问题：

- 手工用 Excel 记录网段分配，容易出错、不易协同
- 难以快速计算"从一个大网段中划出 N 个 IP 的子网"
- 无法直观看到剩余可用网段，导致重复分配或浪费
- 缺少历史规划记录，审计追溯困难

需要一个**轻量级的 IP 网段规划管理工具**，帮助运维高效完成子网划分、余量查看和分配记录持久化。

---

## 二、核心需求

### 2.1 网段池管理

| 编号 | 需求 | 说明 |
|------|------|------|
| R-01 | 添加总网段池 | 支持录入一个或多个大网段（如 `10.0.0.0/8`、`172.16.0.0/12`），作为可分配的地址池 |
| R-02 | 查看网段池 | 展示所有已录入的网段池及其使用率（已分配/总量） |
| R-03 | 删除网段池 | 在无子网分配时允许删除；有分配时提示用户先回收 |

### 2.2 子网规划与分配

| 编号 | 需求 | 说明 |
|------|------|------|
| R-04 | 按需计算子网 | 用户输入"需要 N 个 IP"，系统自动计算最合适的 CIDR（如需要 50 个 → 推荐 /26 = 64 个） |
| R-05 | 从指定网段池分配 | 用户选择目标网段池，系统从可用空间中找到连续空闲块进行分配 |
| R-06 | 标注用途 | 分配时必须填写用途标签（如 VPC 名称、业务线、环境等） |
| R-06a | 负责人 | 分配时可填写负责人，默认取当前登录用户名；支持后续修改 |
| R-07 | 批量规划 | 支持一次提交多条分配需求（如：VPC-A 需 50 个、VPC-B 需 100 个），系统统一计算并分配 |
| R-08 | 冲突检测 | 分配前自动检测是否与已分配网段重叠，重叠则拒绝并提示 |

### 2.3 剩余网段查询

| 编号 | 需求 | 说明 |
|------|------|------|
| R-09 | 查看剩余可用网段 | 以列表形式展示每个网段池中尚未分配的连续空闲段 |
| R-10 | 剩余段容量提示 | 每个空闲段显示可容纳的最大 IP 数，方便下次规划参考 |
| R-11 | 使用率可视化 | 以进度条 / 比例展示各网段池的使用率 |

### 2.4 回收与变更

| 编号 | 需求 | 说明 |
|------|------|------|
| R-12 | 回收已分配子网 | 释放不再使用的子网，归还到可用池 |
| R-13 | 操作日志 | 记录每一次分配、回收操作的时间、操作人、详情，用于审计 |
| R-14a | 编辑分配记录 | 支持修改已分配子网的用途和负责人（CIDR 不可修改） |

### 2.5 用户认证与管理

| 编号 | 需求 | 说明 |
|------|------|------|
| R-14 | 用户登录 | 基于 JWT Token 的登录认证，token 有效期 24 小时 |
| R-15 | 接口保护 | 除登录接口外，所有 API 均需携带有效 Token 或网关头部 |
| R-15a | 网关接入 | 支持外部网关（Nginx / API Gateway）通过 `X-USER` + `X-ROLE` 头部透传已认证用户信息，无需 JWT |
| R-16 | 角色权限 | 管理员（admin）可增删网段池、管理用户；普通用户（user）可分配子网、查看数据 |
| R-17 | 用户管理（Web） | 管理员登录后可通过 API 添加新用户并指定角色 |
| R-18 | 用户管理（CLI） | 二进制支持 `user` 子命令：增删改查用户、修改密码、指定角色 |
| R-19 | 种子用户 | 首次启动自动创建默认管理员 admin/admin123（role=admin） |
| R-20 | 前端权限控制 | 普通用户前端隐藏新增/删除网段池按钮，后端同步拦截返回 403 |

### 2.6 数据持久化

| 编号 | 需求 | 说明 |
|------|------|------|
| R-21 | 数据库存储 | 所有网段池、分配记录、操作日志、用户信息持久化到数据库 |
| R-22 | 数据导出 | 支持将分配记录和审计日志导出为 CSV / JSON，便于与其他系统对接 |

---

## 三、技术设计方案

### 3.1 技术选型

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
│                       MySQL                               │
│   - 生产级关系型数据库，适合多人并发场景                     │
│   - GORM 作为 ORM，自动迁移表结构                          │
└──────────────────────────────────────────────────────────┘
```

**最终方案：Go (Gin + embed) + Vue 3 + MySQL → 单二进制 All-in-One**

核心设计思路：
1. **All-in-One 单文件部署**：前端 Vue 构建产物通过 `go:embed` 打包进 Go 二进制，最终产出一个可执行文件，包含 Web 服务器 + API + 前端页面
2. **MySQL 持久化**：通过启动参数 / 环境变量 / 配置文件传入 MySQL DSN 连接串
3. **零前端服务器**：Go 的 Gin 同时 serve API 和前端静态文件，无需 Nginx
4. **部署极简**：`./network-plan --dsn="user:pass@tcp(127.0.0.1:3306)/ipam"` 即可启动

### 3.2 核心模块划分

```
network-plan/
├── cmd/                        # 入口
│   └── main.go                 # 启动：解析配置 → 连接 DB → 启动 HTTP
├── internal/
│   ├── config/                 # 配置管理
│   │   └── config.go           # MySQL DSN、端口、JWT 密钥、环境变量读取
│   ├── model/                  # 数据模型 (GORM Model)
│   │   ├── pool.go             # 网段池
│   │   ├── allocation.go       # 分配记录
│   │   ├── audit.go            # 操作日志
│   │   └── user.go             # 用户表
│   ├── ipam/                   # IP 地址管理核心算法
│   │   ├── calculator.go       # IP 数量 → CIDR 转换、子网计算
│   │   ├── allocator.go        # 空闲段查找、最佳匹配分配
│   │   └── validator.go        # 冲突检测、输入校验
│   ├── store/                  # 数据持久化 (MySQL + GORM)
│   │   ├── db.go               # 数据库连接、AutoMigrate
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
│   │   ├── export.go           # /api/export (CSV/JSON)
│   │   └── auth.go             # /api/login, /api/register, /api/me
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
│   └── need.md                 # 本文档
├── Makefile                    # make build: 前端构建 → 后端编译 → 单二进制
├── go.mod
└── go.sum
```

### 3.3 数据库设计 (MySQL)

```sql
CREATE DATABASE IF NOT EXISTS ipam DEFAULT CHARSET utf8mb4;
USE ipam;

-- 网段池
CREATE TABLE ip_pool (
    id          BIGINT UNSIGNED AUTO_INCREMENT PRIMARY KEY,
    cidr        VARCHAR(43)  NOT NULL UNIQUE COMMENT '网段 CIDR，如 10.0.0.0/8',
    name        VARCHAR(128) NOT NULL DEFAULT '' COMMENT '网段池名称',
    description VARCHAR(512) DEFAULT '' COMMENT '备注',
    created_at  DATETIME     NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at  DATETIME     NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    INDEX idx_cidr (cidr)
) ENGINE=InnoDB COMMENT='网段池';

-- 子网分配记录
CREATE TABLE allocation (
    id            BIGINT UNSIGNED AUTO_INCREMENT PRIMARY KEY,
    pool_id       BIGINT UNSIGNED NOT NULL COMMENT '所属网段池',
    cidr          VARCHAR(43)  NOT NULL UNIQUE COMMENT '分配的子网 CIDR',
    ip_count      INT UNSIGNED NOT NULL COMMENT '用户申请的 IP 数量',
    actual_count  INT UNSIGNED NOT NULL COMMENT '实际分配 IP 数 (2^n)',
    purpose       VARCHAR(256) NOT NULL COMMENT '用途标签：VPC / 业务线 / 环境',
    allocated_by  VARCHAR(128) DEFAULT '' COMMENT '负责人（默认当前登录用户）',
    allocated_at  DATETIME     NOT NULL DEFAULT CURRENT_TIMESTAMP,
    INDEX idx_pool_id (pool_id),
    INDEX idx_cidr (cidr),
    CONSTRAINT fk_pool FOREIGN KEY (pool_id) REFERENCES ip_pool(id)
) ENGINE=InnoDB COMMENT='子网分配记录';

-- 操作审计日志
CREATE TABLE audit_log (
    id          BIGINT UNSIGNED AUTO_INCREMENT PRIMARY KEY,
    action      VARCHAR(32)  NOT NULL COMMENT 'ALLOCATE / RECLAIM / CREATE_POOL / DELETE_POOL',
    detail      JSON         DEFAULT NULL COMMENT '操作详情',
    operator    VARCHAR(128) DEFAULT '',
    created_at  DATETIME     NOT NULL DEFAULT CURRENT_TIMESTAMP,
    INDEX idx_action (action),
    INDEX idx_created_at (created_at)
) ENGINE=InnoDB COMMENT='操作审计日志';

-- 用户表
CREATE TABLE user (
    id            BIGINT UNSIGNED AUTO_INCREMENT PRIMARY KEY,
    username      VARCHAR(64)  NOT NULL UNIQUE COMMENT '用户名',
    password_hash VARCHAR(255) NOT NULL COMMENT 'bcrypt 哈希密码',
    role          VARCHAR(16)  NOT NULL DEFAULT 'user' COMMENT '角色：admin / user',
    created_at    DATETIME     NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at    DATETIME     NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    UNIQUE INDEX idx_username (username)
) ENGINE=InnoDB COMMENT='用户表';
```

### 3.4 核心算法：子网分配

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

### 3.5 认证与鉴权设计

系统支持两种认证模式，认证中间件按优先级依次尝试：

#### 模式一：JWT Token（直接访问）

```
浏览器                    Go 后端 (Gin)                  数据库
  │                          │                            │
  │  POST /api/login         │                            │
  │  {username, password}    │                            │
  │ ─────────────────────▶   │                            │
  │                          │  查询用户 + bcrypt 验证     │
  │                          │ ──────────────────────────▶ │
  │                          │  ◀────────────────────────  │
  │                          │  签发 JWT (HS256, 24h)      │
  │  ◀─────────────────────  │                            │
  │  {token, user}           │                            │
  │                          │                            │
  │  GET /api/pools          │                            │
  │  Authorization: Bearer   │                            │
  │ ─────────────────────▶   │                            │
  │                          │  JWT 中间件验证签名+过期    │
  │                          │  ✓ → 注入 user_id 到 ctx   │
  │                          │  ✗ → 返回 401              │
  │  ◀─────────────────────  │                            │
  │  {data} / {error: 401}   │                            │
```

#### 模式二：网关透传（通过外部网关接入）

```
用户 → 外部网关（Nginx / API Gateway / BFE 等） → IPAM 后端
       │  已完成身份认证                          │
       │  添加 X-USER / X-ROLE 头部               │
       │ ─────────────────────────────────────▶   │
       │                                          │
       │  请求示例:                                │
       │  GET /api/pools                          │
       │  X-USER: zhangsan                        │
       │  X-ROLE: admin                           │
       │                                          │
       │  中间件检测到 X-USER 头部                 │
       │  → 信任网关认证结果                       │
       │  → 注入 username / role 到 gin.Context    │
       │  → X-ROLE 缺省时默认为 "user"             │
```

#### 认证中间件判定逻辑

```
请求进入
  │
  ├─ 有 Authorization: Bearer <token> ?
  │   ├─ YES → 验证 JWT 签名 + 过期时间
  │   │         ├─ 有效 → 注入 user_id / username / role → 放行
  │   │         └─ 无效 → 返回 401
  │   │
  │   └─ NO → 有 X-USER 头部 ?
  │            ├─ YES → 读取 X-USER 和 X-ROLE（默认 "user"）→ 放行
  │            └─ NO  → 返回 401
```

**关键实现：**

- **密码存储**：使用 `bcrypt` 哈希，`json:"-"` 确保密码 hash 不会泄露到 API 响应
- **Token 结构**：JWT Claims 包含 `user_id`、`username`、`role`、`exp`（过期时间）
- **认证中间件**：双模式 — 优先 JWT Token，回退 `X-USER` / `X-ROLE` 网关头部
- **鉴权中间件**：`RequireAdmin()` 检查 `role` 字段，非 admin 返回 403
- **权限划分**：
  - 管理员（admin）：增删网段池、添加用户
  - 普通用户（user）：分配/回收/编辑子网、查看所有数据、导出
- **网关接入注意事项**：
  - 网关需确保外部请求不能伪造 `X-USER` / `X-ROLE` 头部（应在网关层剥离后重新注入）
  - `X-ROLE` 可选，不传时默认 `user` 角色
  - 网关模式下无需本地用户表，适用于已有统一认证平台的企业环境
- **前端拦截器**：Axios 请求拦截器自动附加 Token，响应拦截器收到 401 自动跳转登录页
- **前端权限控制**：根据 `localStorage.role` 控制按钮显示/隐藏（新增/删除网段池）
- **路由守卫**：Vue Router `beforeEach` 检查 localStorage 中的 Token，无 Token 跳转 `/login`
- **登录页布局**：独立全屏居中布局，不显示侧边栏

### 3.6 认证相关数据模型

```go
// User 用户表（internal/model/user.go）
type User struct {
    ID           uint64    `json:"id" gorm:"primaryKey;autoIncrement"`
    Username     string    `json:"username" gorm:"type:varchar(64);uniqueIndex;not null"`
    PasswordHash string    `json:"-" gorm:"type:varchar(255);not null"`
    Role         string    `json:"role" gorm:"type:varchar(16);not null;default:user"`
    CreatedAt    time.Time `json:"created_at"`
    UpdatedAt    time.Time `json:"updated_at"`
}

// 角色常量
const RoleAdmin = "admin" // 管理员：可增删网段池、管理用户
const RoleUser  = "user"  // 普通用户：可分配子网、查看数据

// JWT Claims（internal/middleware/auth.go）
type Claims struct {
    UserID   uint64 `json:"user_id"`
    Username string `json:"username"`
    Role     string `json:"role"`
    jwt.RegisteredClaims
}
```

### 3.7 Web 页面设计

#### 页面结构 (6 个页面)

```
┌─ 侧边栏导航 ──────────────────────────────────────────────────────────┐
│                                                                       │
│  🔐 登录页       ──→  用户名密码登录（独立布局，无侧边栏）              │
│  📊 仪表盘       ──→  Dashboard 首页                                   │
│  🗂  网段池管理   ──→  网段池 CRUD + 使用率进度条                       │
│  ✂  分配子网     ──→  分配表单 + 批量分配                              │
│  📋 剩余查询     ──→  空闲段列表 + 容量提示                            │
│  📝 操作日志     ──→  审计日志表格 + 筛选                              │
│                                                                       │
│  Header 右侧显示当前用户名 + [退出登录] 按钮                          │
│                                                                       │
└───────────────────────────────────────────────────────────────────────┘
```

#### Dashboard 首页

```
┌─────────────────────────────────────────────────────────────────────┐
│  IP 网段规划管理系统                                                 │
├─────────────────────────────────────────────────────────────────────┤
│                                                                     │
│  ┌─ 统计卡片 ──────────────────────────────────────────────────┐    │
│  │  网段池总数: 5    已分配子网: 23    总使用率: 45.2%          │    │
│  └─────────────────────────────────────────────────────────────┘    │
│                                                                     │
│  网段池概览                                                         │
│  ┌───────────────────┬────────┬───────────────────────────┐        │
│  │ 名称 / CIDR       │ 使用率  │ 进度                      │        │
│  ├───────────────────┼────────┼───────────────────────────┤        │
│  │ 生产环境           │        │                           │        │
│  │ 10.0.0.0/16       │ 23.4%  │ ████░░░░░░░░░░░░         │        │
│  │ 办公网络           │        │                           │        │
│  │ 172.16.0.0/16     │ 67.8%  │ ████████████░░░░         │        │
│  └───────────────────┴────────┴───────────────────────────┘        │
│                                                                     │
│  最近分配记录                                                       │
│  • 10.0.1.0/26  → VPC-Prod-A   (64 IPs)    2024-03-15             │
│  • 10.0.2.0/25  → VPC-Staging  (128 IPs)   2024-03-14             │
│                                                                     │
└─────────────────────────────────────────────────────────────────────┘
```

#### 分配子网页面

```
┌─────────────────────────────────────────────────────────────────────┐
│  分配子网                                                [批量模式]  │
├─────────────────────────────────────────────────────────────────────┤
│                                                                     │
│  选择网段池:  [ 生产环境 10.0.0.0/16        ▼ ]                     │
│                                                                     │
│  ┌─ 单条分配 ─────────────────────────────────────────────────┐    │
│  │  需要 IP 数量:  [ 50        ]                               │    │
│  │  用途标签:      [ VPC-Prod-C ]                              │    │
│  │  负责人:        [ zhangsan   ] (默认当前登录用户)            │    │
│  │                                                             │    │
│  │  → 系统推荐: /26 (64 个 IP)                                 │    │
│  │  → 将从 10.0.3.0/26 分配                                   │    │
│  │                                           [ 确认分配 ]      │    │
│  └─────────────────────────────────────────────────────────────┘    │
│                                                                     │
│  ┌─ 批量分配 (批量模式展开) ──────────────────────────────────────┐    │
│  │  #1  IP数: [ 50  ]  用途: [ VPC-A ]  负责人: [ 可选 ] [删除]  │    │
│  │  #2  IP数: [ 100 ]  用途: [ VPC-B ]  负责人: [ 可选 ] [删除]  │    │
│  │  #3  IP数: [ 200 ]  用途: [ VPC-C ]  负责人: [ 可选 ] [删除]  │    │
│  │                                    [+ 添加] [统一提交]         │    │
│  └────────────────────────────────────────────────────────────────┘    │
│                                                                     │
│  分配记录                                        [导出 CSV] [导出 JSON] │
│  ┌──────────────┬──────┬──────┬──────────┬────────┬──────────┐     │
│  │ CIDR          │ 申请 │ 实际 │ 用途      │ 负责人  │ 操作      │     │
│  ├──────────────┼──────┼──────┼──────────┼────────┼──────────┤     │
│  │ 10.0.1.0/26  │  50  │  64  │ VPC-A    │ admin  │ 编辑 回收 │     │
│  │ 10.0.2.0/25  │ 100  │ 128  │ VPC-B    │ admin  │ 编辑 回收 │     │
│  └──────────────┴──────┴──────┴──────────┴────────┴──────────┘     │
│                                                                     │
│  编辑弹窗（点击"编辑"弹出 Modal）：                                  │
│  ┌─────────────────────────────────────────┐                       │
│  │  编辑分配记录                             │                       │
│  │  CIDR:    10.0.1.0/26 (不可编辑)          │                       │
│  │  用途:    [ VPC-Prod-A      ]             │                       │
│  │  负责人:  [ zhangsan        ]             │                       │
│  │                         [取消] [确定]     │                       │
│  └─────────────────────────────────────────┘                       │
│                                                                     │
└─────────────────────────────────────────────────────────────────────┘
```

### 3.8 RESTful API 设计

#### 公开接口（无需认证）

| 方法 | 路径 | 说明 |
|------|------|------|
| `POST` | `/api/login` | 用户登录，返回 JWT Token |

#### 需认证接口（需携带 `Authorization: Bearer <token>` 或 `X-USER` 头部）

| 方法 | 路径 | 权限 | 说明 |
|------|------|------|------|
| `GET` | `/api/me` | 所有用户 | 获取当前登录用户信息（含角色） |
| `GET` | `/api/dashboard` | 所有用户 | 仪表盘统计数据 |
| `GET` | `/api/pools` | 所有用户 | 获取所有网段池 |
| `POST` | `/api/pools` | **管理员** | 新增网段池 |
| `DELETE` | `/api/pools/:id` | **管理员** | 删除网段池 |
| `POST` | `/api/register` | **管理员** | 添加新用户（可指定角色） |
| `POST` | `/api/allocations` | 所有用户 | 分配子网 |
| `POST` | `/api/allocations/batch` | 所有用户 | 批量分配子网 |
| `PUT` | `/api/allocations/:id` | 所有用户 | 编辑分配记录（用途、负责人） |
| `GET` | `/api/allocations?pool_id=X` | 所有用户 | 查询分配记录 |
| `DELETE` | `/api/allocations/:id` | 所有用户 | 回收子网 |
| `GET` | `/api/pools/:id/free-blocks` | 所有用户 | 查询剩余可用网段 |
| `POST` | `/api/calculate` | 所有用户 | 预计算推荐 CIDR |
| `GET` | `/api/audit` | 所有用户 | 查询操作日志 |
| `GET` | `/api/export?format=csv` | 所有用户 | 导出分配记录（CSV / JSON） |
| `GET` | `/api/export?format=csv&type=audit` | 所有用户 | 导出审计日志（CSV / JSON） |

### 3.9 关键依赖

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

### 3.10 All-in-One 构建流程

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

Go 嵌入前端资源的关键代码：
```go
//go:embed static/dist/*
var frontendFS embed.FS

func setupRouter() *gin.Engine {
    r := gin.Default()

    // API 路由
    api := r.Group("/api")
    { /* 注册各 handler */ }

    // 前端静态文件 (所有非 /api 请求 fallback 到 index.html)
    r.NoRoute(gin.WrapH(http.FileServer(http.FS(frontendFS))))

    return r
}
```

### 3.11 启动与配置

```bash
# 最简启动（使用本地 SQLite）
./network-plan

# 指定 MySQL
./network-plan -dsn "user:pass@tcp(127.0.0.1:3306)/ipam?charset=utf8mb4&parseTime=True"

# 或通过环境变量
export IPAM_DSN="user:pass@tcp(127.0.0.1:3306)/ipam?charset=utf8mb4&parseTime=True"
export IPAM_PORT=8080
export IPAM_JWT_SECRET="your-secret-key"
./network-plan

# 启动后访问 http://localhost:8080 即可使用
# 首次启动自动创建默认管理员: admin / admin123
```

**配置项一览：**

| 配置项 | CLI 参数 | 环境变量 | 默认值 | 说明 |
|--------|----------|----------|--------|------|
| 数据库连接 | `-dsn` | `IPAM_DSN` | 空（使用 SQLite） | MySQL DSN 连接串 |
| 监听端口 | `-port` | `IPAM_PORT` | `8080` | HTTP 服务端口 |
| JWT 密钥 | `-jwt-secret` | `IPAM_JWT_SECRET` | `change-me-in-production` | JWT 签名密钥 |

### 3.12 CLI 用户管理

二进制文件内置 `user` 子命令，可直接在命令行管理用户，无需启动 Web 服务：

```bash
# 新增普通用户（默认 role=user）
./network-plan user add <用户名> <密码>

# 新增管理员
./network-plan user add <用户名> <密码> --role admin

# 列出所有用户（含角色）
./network-plan user list

# 修改密码
./network-plan user passwd <用户名> <新密码>

# 删除用户（admin 不可删除）
./network-plan user delete <用户名>

# 使用 MySQL 时加 -dsn 参数
./network-plan user list -dsn "user:pass@tcp(127.0.0.1:3306)/ipam?charset=utf8mb4&parseTime=True"
```

### 3.13 实现里程碑

| 阶段 | 内容 | 产出 |
|------|------|------|
| **M1 — 核心引擎** | ipam 包：CIDR 计算、空闲段查找、冲突检测 + 单元测试 | 可独立调用的网段计算库 |
| **M2 — 数据层** | MySQL 表结构、GORM Model、CRUD Repository | 完整的持久化能力 |
| **M3 — API 层** | Gin HTTP 路由 + 全部 RESTful API + 配置管理 | 可通过 curl/Postman 调用 |
| **M4 — 前端页面** | Vue 3 全部 6 个页面 + API 对接 + go:embed 集成 | 可交互的 Web 界面 |
| **M5 — 认证** | JWT 登录、中间件保护、路由守卫、用户管理 CLI | 安全的多用户访问 |
| **M6 — 完善** | 批量分配、导出 CSV/JSON、操作日志筛选、Makefile 构建 | All-in-One 单文件可发布 |

### 3.14 其他细节

#### 时间格式化

前端所有时间字段（分配时间、审计日志时间、仪表盘最近记录）统一格式化为 `YYYY-MM-DD HH:mm:ss`，
避免显示 Go 原始时间格式 `2006-01-02T15:04:05.999999999+08:00`。

#### 数据导出

导出接口 `GET /api/export` 支持两个参数：

| 参数 | 可选值 | 默认值 | 说明 |
|------|--------|--------|------|
| `format` | `csv` / `json` | `csv` | 导出格式 |
| `type` | `allocation` / `audit` | `allocation` | 导出数据类型 |

前端通过 Axios 发起下载请求（自动携带 JWT Token），以 Blob 方式触发浏览器保存文件，
避免 `window.open()` 无法携带 Authorization 头部导致 401 的问题。

导出入口：
- **分配记录页**：卡片右上角 [导出 CSV] [导出 JSON] 按钮
- **审计日志页**：筛选栏 [导出 CSV] [导出 JSON] 按钮

#### 负责人字段

分配记录的 `allocated_by` 字段语义为"负责人"（而非操作人）：
- 分配时可手动填写负责人，留空则默认取当前登录用户名
- 批量分配时每条记录可独立填写负责人
- 分配后可通过编辑弹窗修改负责人
- 审计日志中的 `operator` 字段始终记录实际操作人（来自 JWT / X-USER）

#### 网关接入配置示例

**Nginx 反向代理透传认证信息：**

```nginx
location /api/ {
    # 先在网关层完成认证（如 OAuth2 / SSO），然后注入头部
    proxy_set_header X-USER  $authenticated_user;
    proxy_set_header X-ROLE  $user_role;  # 可选，默认 user
    proxy_pass http://127.0.0.1:8080;
}
```

**curl 测试网关模式：**

```bash
# 以 admin 角色访问
curl -H "X-USER: zhangsan" -H "X-ROLE: admin" http://localhost:8080/api/pools

# 不传 X-ROLE，默认 user 角色
curl -H "X-USER: zhangsan" http://localhost:8080/api/pools
```

---

## 四、后续可扩展方向

1. **多租户**：不同团队管理各自的网段池，增加角色与权限控制（管理员 / 普通用户）
2. **与云平台对接**：自动同步 AWS/阿里云/百度云 VPC 实际网段使用情况
3. **IPAM 标准协议**：对接 NetBox 等开源 IPAM 系统
4. **告警**：网段池使用率超过阈值时通知
5. **IaC 集成**：供 Terraform 等工具通过现有 API 自动化调用分配
