# 账小鸭开发文档（供新环境 / 新模型接手）

> 账小鸭：一句话记账 · 语音记账 · 智能查询 · 报表可视化 · 多类型共享账本。
> 本文档目标：让接手者**不看历史对话**也能完整理解现状、对齐所有功能与约定，并避免重复踩坑。
> **上线目标**：优先网页端 + 微信小程序；部署在**阿里云 ECS + RDS MySQL**。
> 仓库是 git 仓库，改动请走标准 git 工作流（commit / branch / PR）。
> **产品文档**：[PRODUCT.md](./PRODUCT.md)（定位、规则、进度、早上验收清单）。

### 当前实现状态（2026-08-07 首版终检）

- **已落地**：个人/家庭/旅游/礼账账本、手动+AI/语音记账（个人账本）、`PushToLedger` / `PushMonth` 单向同步、改删源级联副本、目标账本单独移除副本、明细/报表账本切换、当月 overview（`start`/`end`）、预算设置 UI、个人账户管理。
- **首期已纳入**：微信 openid 登录、麦克风用途声明、录音前微信隐私授权弹层、H5/小程序构建检查、ECS systemd + Nginx 部署模板。小程序后台隐私指引、内容安全策略和真机审核仍属于上线前人工任务。
- **终检通过**：`go test ./...`、`go vet ./...`、`govulncheck ./...`（可调用漏洞 0）、H5 build、小程序 build/check、独立 SQLite E2E；小程序构建约 540KB。
- **自测脚本**：后端起好后可跑 `bash backend/e2e_test.sh`（默认打 `127.0.0.1:18080`）。

---

## 1. 技术栈与目录


| 层     | 技术                                                                                    |
| ----- | ------------------------------------------------------------------------------------- |
| 后端    | Go 1.26.5（`toolchain` 已锁定）+ Gin + GORM。生产 MySQL；开发用纯 Go SQLite（无 CGO）      |
| 前端    | uni-app 5.23 alpha（Vue3，Options API），一套代码出 H5 + 微信小程序；Vue/Vite 已固定版本            |
| AI 对话 | 阿里云百炼 DashScope，OpenAI 兼容接口，`github.com/sashabaranov/go-openai` + Function Calling    |
| AI 语音 | 百炼 `qwen-audio-3.0-asr-flash`，走 OpenAI 兼容 `chat/completions` + `input_audio`（**非** Whisper 接口） |
| 报表    | `github.com/xuri/excelize/v2`                                                         |
| 认证    | `github.com/golang-jwt/jwt/v5`                                                        |
| 金额    | `github.com/shopspring/decimal`（DB 用 `decimal(18,2)`）                                 |
| 配置    | `github.com/spf13/viper`                                                              |


```
accounter/
├── backend/
│   ├── cmd/server/main.go          # 入口：装配 service/ai/handler，AI 配置来源，启动 gin
│   ├── configs/config.yaml         # 本机私有配置，已 gitignore；仓库只提交 .example
│   └── internal/
│       ├── model/model.go          # 全部 GORM 模型 + AllModels()
│       ├── config/config.go        # viper 加载 + 环境变量覆盖 + 默认值
│       ├── repository/db.go        # NewDB + AutoMigrate + migrateToLedgers（账本迁移）
│       ├── middleware/middleware.go# JWT鉴权 + RequireLedger账本上下文 + CORS
│       ├── service/                # account/auth/category/transaction/stats/budget/ledger/setting/seed
│       ├── ai/                     # ai.go(对话) + asr.go(语音) + tools.go(function定义)
│       ├── report/excel.go         # excelize 导出
│       └── handler/                # HTTP 层；路由注册在 cmd/server/main.go
├── frontend/src/
│   ├── pages/{chat,record,list,report,profile,login}/
│   ├── common/{request.js,config.js,auth.js}
│   ├── pages.json                  # 路由 + tabBar
│   └── App.vue                     # 全局样式（含多个 H5 修复）
├── deploy/                         # ECS env/systemd/Nginx 模板与部署说明
└── dev.sh                          # 本地一键启动（SQLite）
```

分层依赖方向：`handler → service → model`；`ai` 依赖 `service`；`middleware` 依赖 `model`+`gorm`。

---

## 2. 快速开始与配置

### 本地开发（SQLite + H5，日常调试用）

**开发策略**：本地用 **SQLite + H5** 快速迭代，生产用 **MySQL + 小程序** 上线。两者通过 GORM 抽象层兼容，开发期无需安装 MySQL。

```bash
./dev.sh   # 后端 Go+SQLite :8080（库文件 backend/dev.db，日志 backend/dev-server.log）+ 前端 H5 :5173
```

**dev.sh 环境变量**：

- `DB_DRIVER=sqlite`、`SQLITE_PATH=dev.db`（命令在 `backend/` 内执行）、`JWT_SECRET=dev-secret`
- **⚠️ 不再自动注入 `DASHSCOPE_API_KEY`**（历史上它读 `~/.bailian/config.json` 注入，会**覆盖 config.yaml 的对话 key** 导致 401，已移除，见 §9 踩坑）。AI 配置一律走 `backend/configs/config.yaml`。

**本地开发工作流**：

1. **后端热重载**（可选）：`cd backend && go run ./cmd/server` 或安装 `air`（`go install github.com/cosmtrek/air@latest` → `air` 自动监听文件变化重启）
2. **前端热重载**：`cd frontend && pnpm dev:h5`（vite，自动刷新）
3. **数据库管理**：
  - 查看数据：`sqlite3 backend/dev.db`（命令行）或用 DB Browser for SQLite（GUI）
  - 重置数据：删除 `backend/dev.db` → 重启后端自动 AutoMigrate + 种子数据
  - 备份：`cp backend/dev.db backend/dev.db.bak`
4. **测试 AI 功能**：确保 `backend/configs/config.yaml` 中 `ai.api_key` 有效（阿里云百炼控制台申请）
5. **浏览器访问**：`http://localhost:5173`（vite 代理 `/api/`* 到 `:8080`）

**SQLite vs MySQL 差异注意**（开发期要意识到，上线前验证）：

- **时区**：SQLite 存 UTC，MySQL 按 `loc=Local` 存。按天分组时 SQLite 可能 −1 天，已用"补 12:00"绕过（见 §9.3、§14.7）
- **日期函数**：SQLite 用 `strftime`，MySQL 用 `DATE_FORMAT`。代码里 `stats.go` 的 `periodExpr` 已按方言分支
- **布尔类型**：SQLite 用 0/1，MySQL 用 TINYINT(1)。GORM 自动处理
- **索引语法**：GORM AutoMigrate 自动处理，无需关心
- **性能**：SQLite 单写并发，MySQL 多写并发。本地开发够用，上线前压测用 MySQL

**本地开发优势**：

- 零依赖：不需要安装 MySQL、Redis 等
- 快速重置：删文件即可重置数据库
- 便于调试：可以直接 SQL 查询 dev.db
- 前端热重载：vite 秒级刷新

**何时切换到 MySQL 测试**：

- 上线前 1-2 周，本地起一个 MySQL 容器（`docker run -p 3306:3306 -e MYSQL_ROOT_PASSWORD=root mysql:8`）
- 修改 `config.yaml` 或环境变量切换到 MySQL
- 验证 AutoMigrate、索引、方言差异、并发行为

### 生产（首版采用 systemd + Nginx）

仓库现有 [`deploy/`](./deploy/) 模板：`env.example`、`accounter.service`、`nginx.conf.example`。首版不依赖 Docker；后端为单个 Linux 二进制，H5 为 Nginx 静态文件。

### 阿里云 ECS + RDS 部署（目标上线方案）

**架构概览**：

```
用户 → 域名(HTTPS) → 阿里云 SLB(可选) → ECS(Nginx:80/443)
                                         ├─ /api/* → Go 后端(:8080) → RDS MySQL
                                         └─ /*    → 静态 H5 文件(/var/www/accounter)
微信小程序 → 微信服务器 → 同上
```

**ECS 侧**：

- 系统：推荐 Ubuntu 22.04 / Alibaba Cloud Linux 3
- 安全组：开放 22(SSH)、80(HTTP)、443(HTTPS)，**RDS 3306 不对外开放**
- 软件栈：Nginx + Go 二进制（systemd 托管）+ certbot（Let's Encrypt）
- 目录布局：
  ```
  /opt/accounter/
    ├── bin/accounter                # Go 二进制
    └── web/                         # H5 构建产物
  /etc/accounter/env                 # 生产环境变量（权限 600）
  /etc/nginx/sites-available/accounter.conf
  /etc/systemd/system/accounter.service
  ```

**RDS MySQL 侧**：

- 实例规格：起步 1C2G 够用（记账 QPS 低），按量付费或包年
- 白名单：只允许 ECS 内网 IP（同 VPC 走内网，免流量费、延迟 < 1ms）
- 账号：为应用单独建账号（非 root），只给目标库 DML + DDL 权限
- 数据库：`accounter`，字符集 `utf8mb4`（支持 emoji 备注）
- 备份：RDS 自动备份保留 7 天，关键变更前手动打快照

**生产环境变量**（完整模板见 `deploy/env.example`）：

```dotenv
DB_DRIVER=mysql
DB_HOST=rm-xxx.mysql.rds.aliyuncs.com
DB_PORT=3306
DB_USER=accounter
DB_PASSWORD=CHANGE_ME
DB_NAME=accounter
JWT_SECRET=用 openssl rand -hex 32 生成
WECHAT_APP_ID=wx6339d0862921ce0e
WECHAT_APP_SECRET=CHANGE_ME
DASHSCOPE_API_KEY=CHANGE_ME
```

**推荐**：生产环境**不保留** config.yaml 中的 API Key，改用环境变量注入（`DASHSCOPE_API_KEY` / `AI_BASE_URL` / `JWT_SECRET` / `DB_DSN`），配合 systemd 的 `EnvironmentFile=/etc/accounter/env`（权限 600）。

**Nginx 配置要点**：

```nginx
server {
    listen 443 ssl http2;
    server_name accounter.yourdomain.com;
    ssl_certificate     /etc/letsencrypt/live/.../fullchain.pem;
    ssl_certificate_key /etc/letsencrypt/live/.../privkey.pem;

    # 同源托管 H5
    root /opt/accounter/web;
    index index.html;

    # API 反代
    location /api/ {
        proxy_pass http://127.0.0.1:8080;
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto $scheme;
    }
    # H5 history 路由回退
    location / { try_files $uri $uri/ /index.html; }
}
server { listen 80; server_name accounter.yourdomain.com; return 301 https://$host$request_uri; }
```

**CORS**：网页端同源部署后无需配置；小程序请求通常不带浏览器 Origin。H5 分域时用 `CORS_ALLOWED_ORIGINS=https://网页域名`，后端已按白名单放行，不再反射任意 Origin。

### 配置系统（`config/config.go`）

**优先级：环境变量 > config.yaml > 代码默认值。** `applyEnvOverrides` 逐个 `os.Getenv` 覆盖。

`config.yaml` 的 `ai` 段（**对话与语音是两套独立接入**）：

```yaml
ai:
  enabled: false         # 默认关闭；true 时才开放文字 AI、语音、激活及 AI 草稿确认
  base_url: "https://token-plan.cn-beijing.maas.aliyuncs.com/compatible-mode/v1"
  api_key: "..."
  model: "qwen-max"
  asr:                  # 语音识别（可与对话不同的 base_url/api_key）
    base_url: "https://token-plan.cn-beijing.maas.aliyuncs.com/compatible-mode/v1"
    api_key: ""         # 留空则复用对话模型的 api_key
    model: "qwen-audio-3.0-asr-flash"
```

环境变量：`DB_DRIVER DB_DSN SQLITE_PATH DB_HOST/PORT/USER/PASSWORD/NAME JWT_SECRET JWT_EXPIRE SERVER_PORT CORS_ALLOWED_ORIGINS AI_ENABLED DASHSCOPE_API_KEY AI_BASE_URL AI_MODEL AI_ASR_BASE_URL AI_ASR_API_KEY AI_ASR_MODEL WECHAT_APP_ID WECHAT_APP_SECRET WECHAT_API_BASE`。
默认值：driver=sqlite、port=8080、jwt.expire=168h、`AI_ENABLED=false`、ai.model=qwen-max、asr.model=qwen-audio-3.0-asr-flash，对话与 ASR 默认同源到 token-plan MaaS；ASR key 为空时复用对话 key。AI 必须通过 `config.yaml` 或环境变量显式开启；MySQL 模式会拒绝短 JWT 或开发密钥启动（AI 关闭时不校验 AI 激活配置）。

---

## 3. 数据模型（`model/model.go`，全部进 `AllModels()` 走 AutoMigrate）

隔离维度：**业务数据（account/category/transaction/budget）按 `ledger_id` 隔离**，不是 user_id。

```go
User        { ID, Username(uniq), PasswordHash(json:"-"), Nickname, AvatarURL, 软删除 }
Ledger      { ID, Name, OwnerID, IsPersonal bool, Type(personal|family|travel|gift), InviteCode(size:16), CreatedAt }   // 账本
Membership  { ID, LedgerID, UserID, Role(owner|member), Alias, uniqueIndex(ledger,user) } // 成员关系
Account     { ID, LedgerID, UserID(创建者), Name, Type(alipay|wechat|cash|savings|credit),
              Balance decimal(18,2), Sort, IsArchived, LinkedAccounts(响应填充), 软删除 }
AccountLink { ID, LedgerID, PaymentAccountID(支付宝/微信), FundingAccountID(储蓄卡/信用卡) }
Category    { ID, LedgerID, UserID, Name, ParentID *uint(二级), Kind(income|expense), Icon, Sort, 软删除
              Children []Category `gorm:"-"`, Parent *Category }  // 树在内存组装
Transaction { ID, LedgerID, UserID(记账/归属成员), AccountID(实际资金账户),
              PaymentAccountID*(支付宝/微信支付渠道), ToAccountID*(转账转入),
              CategoryID*, OpType, Amount decimal, Remark, OccurredAt, SourceID *uint, 软删除
              Account/Category/Member 关联(Preload 用) }
Budget      { ID, LedgerID, UserID, CategoryID *uint(nil=总预算), Amount decimal }  // 月度循环预算
BookkeepingPreference { ID, LedgerID(uniq), UserID, AccountMode(default|strict),
                         DefaultAccountID*, DefaultPaymentAccountID* } // AI 账户策略
Setting     { Key(pk), Value text }  // 历史全局 KV，当前模型配置不从客户端写入
```

- **OpType**（`ValidOpType` 校验）：`expense 支出 / income 收入 / borrow_in 借入 / borrow_out 借出 / transfer 转账`。
- **余额语义**：现金/储蓄卡/支付宝/微信的 `balance` 是资产余额；信用卡的 `balance` 是当前欠款。信用卡消费增加欠款，退款与还款减少欠款。
- **支付渠道与实际账户分离**：如“支付宝绑定招商信用卡消费”，`account_id=招商信用卡`、`payment_account_id=支付宝`；支付宝余额消费则 `account_id=支付宝`、`payment_account_id=NULL`。
- **转账/平账不是收支**：`transfer` 不进入收入、支出、分类统计或预算。储蓄卡转入信用卡表示还款，同时减少储蓄卡余额和信用卡欠款；净资产为普通账户余额合计减信用卡欠款。
- **CategoryKind**：`income | expense`。
- **Role**：`owner 创建者 / member 成员`。
- `Transaction.SourceID`：非空表示这是从别的账本「同步」来的副本，指向源流水 id（防重复推送 + 单向镜像，见 §7.8）。
- 各 `LedgerID` 的 gorm tag 必须带 `default:0`（否则 SQLite 给已有数据表加 NOT NULL 列失败，见 §9）。

### 老数据迁移（`repository/db.go: migrateToLedgers`，每次启动跑，幂等）

1. 确保每个 user 有个人账本（`IsPersonal=true`）+ owner 成员关系；没有则创建。
2. **独立幂等回填**（无论是否新建账本）：把该 user 名下 `ledger_id=0 或 NULL` 的 account/category/transaction/budget 更新为其个人账本 id。用 `**db.Unscoped()`** 连软删除行一并回填。

---

## 4. 认证与账本上下文（`middleware/middleware.go`）

- `Auth(secret)`：解析 `Authorization: Bearer <jwt>`，注入 `userID`（claim `uid`）。`NewToken` 签发，默认 168h。
- `RequireLedger(db)`：读请求头 `**X-Ledger-Id**`；为空则用该用户的**个人账本**（`owner_id=uid AND is_personal=true`）；非空则校验 `Membership(user,ledger)` 存在，否则 403「你不是该账本成员」。注入 `ledgerID`。
- `middleware.UserID(c)` / `middleware.LedgerID(c)` 取上下文。

**路由分两组**（注册于 `cmd/server/main.go`）：

- **账本管理组**（仅 `Auth`，不绑定具体账本）：`/ledgers`*。
- **业务组**（`Auth + RequireLedger`，按 ledger 隔离）：accounts / categories / transactions / push-month / stats / budgets / report / ai。

---

## 5. 完整 API 列表（均在 `/api` 下）

公开：

- `POST /auth/register {username,password,nickname?}` → `{token,user}`（注册即建个人账本+播种默认账户/分类）
- `POST /auth/login` → `{token,user}`（H5 本地调试）
- `POST /auth/wechat {code}` → `{token,user}`（微信小程序 openid 登录；首次登录自动注册并创建个人账本）
- `POST /auth/wechat-phone {code,phone_code}` → `{token,user}`（微信小程序授权手机号登录；首次登录自动注册并创建个人账本）
- `GET /health` → `{status,ai}`

用户资料（需登录）：

- `GET /me` / `PUT /me {nickname}` → 读取或更新当前用户公开资料
- `POST /me/avatar`（multipart 字段 `avatar`）→ 上传 JPG/PNG 头像，限制 2MB、最大 4096×4096；文件保存到 `UPLOAD_DIR/avatars`

生产默认 `UPLOAD_DIR=/opt/accounter/uploads`；发布脚本只确保目录权限，不会替换或回滚该目录，避免版本发布时丢失用户头像。

账本管理（需登录）：

- `GET /ledgers` → 我加入的账本列表 `[{...ledger, role, member_count}]`（个人账本在前）
- `POST /ledgers {name,type?}` → 创建家庭/旅游/礼账账本（`type` 缺省为 `family`，生成邀请码 + 播种默认账户/分类）
- `POST /ledgers/join {code}` → 邀请码加入（幂等）
- `GET /ledgers/:id/members` → 成员列表 `[{user_id,username,nickname,alias,role}]`
- `DELETE /ledgers/:id/members/:uid` → owner 移除成员
- `POST /ledgers/:id/leave` → 成员退出（owner/个人账本不可退）

业务（需登录 + `X-Ledger-Id`，缺省=个人账本）：

- 账户 `GET/POST/PUT /accounts/:id/DELETE`；创建/编辑支持 `linked_account_ids`，仅支付宝/微信可关联储蓄卡或信用卡
- 分类 `GET/POST/PUT /categories/:id/DELETE`（`?kind=income|expense`）
- 流水 `GET /transactions`（筛选 `start end account_id category_id member_id op_type keyword page page_size`）、`POST`、`PUT/:id`、`DELETE/:id`
  - `GET /transaction-months` → 全部有流水月份的 `month/count/income/expense` 摘要；展开后按月份懒加载明细
  - `POST /transactions/:id/push {target_ledger_id}` → 单笔推送到目标账本
  - `POST /push-month {month:"YYYY-MM", target_ledger_id}` → 整月批量推送（**独立路径**，避开 `:id` 路由冲突）
- 统计 `GET /stats/overview|trend|category|monthly`（overview 支持可选 `start`/`end`，返回 income/expense/balance/total_assets/transfer；trend `?granularity=day|month`；category `?kind=`；monthly `?months=`）
- 预算 `GET /budgets`（附本月已用 spent）、`PUT /budgets {category_id,amount}`、`DELETE /budgets/:id`
- 报表 `GET /report/monthly?month=YYYY-MM`、`GET /report/export?month=`（返回 xlsx，**必须带 `X-Ledger-Id`**）
- AI `POST /ai/chat {message, history?}`（`history` 为近期 `user`/`assistant` 文本，最多 12 条，不含本轮；返回 `draft`/`drafts`）、`POST /ai/transcribe {audio(base64),format}`、`POST /ai/confirm {draft|drafts}`、`GET /ai/settings`（只读占位）
  - `GET /ai/account-preference` → 当前账户策略；旧用户首次读取自动建立“便捷模式 + 支付宝余额”
  - `PUT /ai/account-preference {account_mode,default_account_id,default_payment_account_id?}` → 保存便捷/精确模式与默认支付路径

响应统一 `{data:...}`（`handler.ok`）或 `{error:...}`（`handler.fail`）；前端 `request.js` 自动剥 `data`。

---

## 6. 关键业务规则（务必对齐）

1. **记账只在个人账本**：`TransactionService.Create` 与 `AIConfirm` 均校验个人账本，其他类型账本直接记账一律拒绝。家庭/旅游/礼账账本数据**只能通过同步**（`PushToLedger` 走内部 `createInternal`，跳过个人账本守卫）。同步副本（`SourceID != nil`）禁止 Update，但允许从当前目标账本单独删除。
2. **前端无全局"当前账本"切换**：记账页（record/chat）请求不带 `X-Ledger-Id` → 后端默认个人账本。只有**明细/报表**页可切换查看账本（只读）。
3. **目标账本明细只读**：查看家庭/旅游/礼账账本时不能新增或修改流水，仅展示 + 显示同步成员；可单独移除同步副本。
4. **同步是单向镜像**：源（个人账本）→ 目标账本副本。改源自动改仍存在的副本，删源级联删副本；删除目标副本不影响源，并允许后续重新同步。
5. **AI 账户策略由后端强制执行**：`default` 便捷模式下，完全没说账户时草稿强制覆盖为设置的默认账户；只说支付宝/微信可直接按该渠道余额记。`strict` 精确模式下，未明确到余额或具体卡时不生成可确认草稿，必须先追问。不能只依赖模型提示词。

---

## 7. 各功能模块细节

### 7.1 账户 `service/account.go`

按 `ledger_id` 隔离。账户类型严格限定为支付宝、微信、现金、储蓄卡、信用卡；旧 `bank` 启动时幂等迁移为 `savings`。支付宝/微信通过 `account_links` 多对多关联未归档储蓄卡/信用卡，列表响应填充 `linked_accounts`。删除前检查流水和关联关系，有则拒绝（可归档）。

### 7.2 分类 `service/category.go`

二级树（`ParentID` 空为一级）。`ListTree` 内存组装 children；`ListFlat` 供 AI 上下文。`Create(ledgerID, userID, c)` 校验 parent 归属同账本，子类继承父类 kind。删除前检查有无流水 / 子分类。

### 7.3 流水与余额同步 `service/transaction.go`（核心）

- `applyBalance(tx, t, sign)`：在**数据库事务内**按账户类型调整余额。普通账户按资产增减；信用卡消费增加欠款、退款减少欠款；转入信用卡即还款、欠款减少。更新/删除必须先反向恢复旧流水再应用新流水。
- `PaymentAccountID` 只允许支付宝/微信，且必须存在 `(payment_account_id, funding_account_id=AccountID)` 关联；转账不得设置支付渠道。
- `Create(ledgerID, userID, t)`：设 LedgerID/UserID → `validate`（账户/分类归属按 ledger 校验，金额>0，转账需不同转入账户）→ 事务内 Create + applyBalance(+1)。
- `Update(ledgerID, id, next)`：事务内 applyBalance(old,-1) → Updates → applyBalance(next,+1)；保留原 UserID/CreatedAt；**更新后调 `syncCopies(next)`**。
- `Delete(ledgerID, id)`：事务内 applyBalance(-1)+删除；**删除后调 `deleteCopies(id)`**。
- `List(ledgerID,f)`：预加载实际账户、支付渠道、转入账户、分类父级和成员；分页并按时间倒序。`ListMonths` 只返回月份摘要，前端展开后再查当月明细。

### 7.4 统计 `service/stats.go`

全部按 `ledger_id`。`periodExpr` 按方言：sqlite `strftime`，mysql `DATE_FORMAT`。转账/还款不进入收支趋势、月度对比、分类与预算；Overview 单独返回 `transfer`。`CategoryBreakdown` 把子类金额归并到一级分类，**无数据时返回 `[]`**。`total_assets` = 普通账户资产合计 - 信用卡欠款（不受日期过滤）。

### 7.5 报表 `service/report/excel.go` + `handler/report.go`

复用 `TxFilter` 拉全量流水写 xlsx。导出走 `exportExcel(c, ledgerID, f, ...)`。前端 H5 用 fetch+blob 下载、小程序用 downloadFile+openDocument，**都要带 `X-Ledger-Id` 头**才能导出所查看账本。

### 7.6 预算 `service/budget.go` + `handler/budget.go`

按月循环（不绑定具体月份）。`Set(ledgerID, userID, categoryID, amount)` upsert（categoryID nil=总预算）。`ListBudgets` 返回每条 `{amount, spent}`，spent 用本月 `CategoryBreakdown`（总预算=本月总支出）。前端设置弹层批量 upsert/删除。

### 7.7 AI 对话记账 `ai/ai.go` + `ai/tools.go`

- `Service` 支持运行时热更新配置（读写锁）。**两套配置**：对话 `client/model/apiKey/baseURL` + 语音 `asrModel/asrAPIKey/asrBaseURL`。`Configure(base,key,model)` 配对话；`ConfigureASR(base,key,model)` 配语音（空则回落对话配置，故 NewService 里**先 Configure 再 ConfigureASR**）。
- `Chat(ctx, ledgerID, userID, msg, history)`：`buildSystemPrompt` + 客户端携带的近期历史（无状态、最多 12 条）拼成多轮上下文；三个 tool（`create_transaction` / `create_transactions` / `query_transactions`）。一段话多笔时优先 `create_transactions`；信息不足时先自然语言追问，用户补充后结合历史再出草稿；**不直接落库**，返回 `draft` + `drafts`；查询走 stats。
- 创建工具字段 `account_specified` 表示用户说明是否满足当前策略：便捷模式只在完全没说账户时为 `false`，精确模式则必须明确完整路径。便捷模式的 `false` 会由后端覆盖为默认路径，精确模式的 `false` 返回追问。模型不得把系统默认值伪装成用户明确说明。
- `ConfirmDraft` / `ConfirmDrafts`：落库（handler 已拦非个人账本；confirm 支持单笔 `draft` 或批量 `drafts`）。精确模式在确认阶段会再次拒绝 `account_specified=false`，避免客户端绕过对话校验。
- 相对时间：AI 未说明具体时间时，`normalizeTime` 会在用户描述的日期上沿用当前时分秒，避免实时记账全部显示为 `12:00`。

### 7.8 语音识别 `ai/asr.go`

- **百炼不支持 Whisper 风格 `/audio/transcriptions`（404）**。走 OpenAI 兼容 `chat/completions`，body：`{model:qwen-audio-3.0-asr-flash, messages:[{role:user, content:[{type:"input_audio", input_audio:{data:"data:audio/<fmt>;base64,<b64>"}}]}]}`，返回 `choices[0].message.content` 即文本。
- `Transcribe(ctx, audioB64, format)` 用 `net/http` 直接调（**不经 go-openai**，因 data URI 结构特殊），复用 asr 那套 base_url/key/model；`audioMime()` 映射格式→MIME（mp3/wav/webm/m4a/aac/ogg…）。
- ASR 端点路径注意：百炼「专属部署/MaaS」域名（`llm-xxx.cn-beijing.maas.aliyuncs.com`）兼容路径是 `**/compatible-mode/v1**`，不是 `/api/v1`。
- **上线前必须验证模型可调用**：请求 `GET {AI_ASR_BASE_URL}/models` 确认返回中包含 `qwen-audio-3.0-asr-flash`，并使用真实录音完成一次识别冒烟测试；当前 `token-plan` 已确认提供该模型。

### 7.9 AI 记账偏好 `service/bookkeeping_preference.go` + `handler/bookkeeping_preference.go`

- 每个个人账本一条 `BookkeepingPreference`。首次读取时自动选择未归档支付宝余额；没有支付宝则选第一个未归档账户。
- 默认路径可表示余额（`default_account_id=支付宝/微信`、payment 为空）、直接账户，或支付渠道关联卡（`default_account_id=实际卡`、`default_payment_account_id=支付宝/微信`）。保存时必须校验账户归属、归档状态和 `account_links`。
- 默认账户被归档后，下一次读取会自愈到当前可用的支付宝余额或第一个账户。精确模式仍保留默认路径，切回便捷模式时可继续使用。
- 模型供应商配置仍走 `config.yaml` / 环境变量；`GET /ai/settings` 只返回是否已配置，不在客户端暴露 API Key。

### 7.10 账本共享 / 同步 `service/ledger.go` + `transaction.go`

- `LedgerService`：`ListForUser / Create(邀请码) / Join / Members / RemoveMember / Leave / Role / IsPersonal / PersonalLedgerID`。邀请码 `genInviteCode` 用 `crypto/rand` 生成 6 位（去掉易混字符）。
- **推送/同步**（`transaction.go`）：
  - `mapAccountToLedger` / `mapCategoryToLedger`：按**名称**把账户/分类映射到目标账本（账户找不到用第一个，分类找不到留空）。
  - `PushToLedger(srcLedgerID, srcID, targetLedgerID, userID)`：复制一份到家庭/旅游/礼账账本，`SourceID=srcID`，防重复（目标已有该 source 则拒绝）。支持收入、支出和转账/还款；转账会分别映射目标账本中的转出、转入账户，保持账户类型和两端不同，仍不计入收支统计。
  - `PushMonth(srcLedgerID, month, targetLedgerID, userID)`：遍历该月 income/expense/transfer 流水逐笔 PushToLedger，返回 `{pushed, skipped}`。
  - `syncCopies(src)`：改源后同步所有副本（重映射账户/分类 + 撤销旧余额+应用新余额）。
  - `deleteCopies(srcID)`：删源后级联删副本。
- 副本记账人 UserID = 执行推送的人 → 家庭账本明细可显示「谁同步的」。

---

## 8. 前端要点

- `**common/request.js**`：统一封装 uni.request，注入 JWT、处理 401、剥 `{data}`。游客请求收到 401 时只返回 `AUTH_REQUIRED`，**不得自动跳登录**；仅已有 token 失效时才导航到重新登录。**账本上下文按调用显式传 `options.ledgerId`**（→ 头 `X-Ledger-Id`），不传=个人账本。read 类 api（overview/trend/categoryStats/monthly/transactions/budgets/categories/setBudget/deleteBudget）都有可选 `ledgerId` 末参。**不再有全局"当前账本"注入**。
- `**common/config.js**`：`BASE_URL`（H5 空走 vite 代理，小程序/生产填后端地址）、`TOKEN_KEY`、`USER_KEY`、`LEDGER_KEY`（已基本弃用）。
- **页面**（`pages.json` 5 个 tab + 登录 + 账户/分类管理页）：
  - `chat` AI 记账：文字/语音 + 多轮追问（请求带近期 `history`）；一段话可多笔草稿；确认卡片可逐笔或全部保存。
  - `record` 记一笔：支持支出/收入/转账还款；账户弹层按“支付宝、微信、直接账户”分组，支付渠道内再选择余额或关联卡；功能行统一使用 `static/ui/*.svg` 线性图标。
  - `list` 明细：**按月折叠 + 按日期分组**，月份头显示笔数/收支；分类图标使用统一圆形底；每笔提供可见的更多操作，不再只依赖长按。
  - `report` 报表：总览采用“本月结余 + 2×2 收入/支出/转账/净资产”；月度日历热力图支持支出/收入/结余三种口径，点击日期按需加载当天明细；每日趋势同时显示收入和支出，分类占比使用饼图和百分比。
  - `profile` 我的：保留用户、账本、加入共享账本与管理入口；创建时可选择家庭/旅游/礼账类型；“AI 记账方式”可切换便捷/精确模式，并选择支付宝/微信余额、关联卡或直接账户作为默认支付路径。
  - `account` 账户管理（非 tab 页）：按支付渠道/资产/负债分组，展示总资产、信用卡欠款和净资产，负责账户关联与归档。
  - `category` 分类管理（非 tab 页）：独立管理一级/二级分类、折叠和图标选择。
  - `login`：微信小程序登录 / H5 登录注册；提供“暂不登录，继续浏览”，登录后返回触发登录的主功能页。
- **游客浏览与按需登录（审核硬约束）**：5 个 tab 页不得在 `onLoad/onShow` 调用 `checkLogin()`，未登录时不得请求个人数据，也不得自动调用 `uni.login`。AI 首页展示功能说明，记一笔展示表单，明细展示隐私空状态，报表展示零数据结构，我的展示游客卡片；只有用户主动发送、保存、查看个人数据、导出或进入管理功能时，才通过 `requireLogin()` 提供“去登录 / 继续浏览”选择。`wx.login` 只能由登录页的“微信登录”按钮触发。
- 新账本默认账户顺序（seed）：**支付宝 → 微信 → 现金 → 储蓄卡 → 信用卡**（支付宝为默认；`service/seed.go`）。

### 8.1 UI 视觉规范（H5 / 微信小程序共用）

- 风格关键词：**清新、简约、可信、低干扰**；业务布局优先，避免玻璃拟态、重渐变和装饰性插画。
- 基础色：页面背景 `#F7F9F7`，卡片 `#FFFFFF`，主文字 `#18312B`，次文字 `#71817A`，弱文字 `#87968F`。
- 品牌色：薄荷绿 `#42C39C`（强调态可用 `#36B88F`）；支出/危险态统一珊瑚红 `#F06F64`；避免重新引入系统蓝 `#007AFF`。
- 卡片：默认圆角 `18px`、边框 `#EDF1EE`、轻阴影 `0 8px 28px rgba(24,49,43,.04)`；弹层圆角 `20px`。
- 按钮：主按钮薄荷绿、圆角 `14-16px`、高度至少 `44px`；次按钮用淡薄荷灰底，不使用纯灰系统按钮。
- 页面节奏：左右边距 `16px`，区块间距 `14-22px`，遵循 8px 间距体系；标题 27px/700，卡片标题 16-17px/700，正文 14-15px。
- 金额：金额是记账页面第一视觉层级；收入用薄荷绿，支出用珊瑚红，正文不要用颜色表达过多状态。
- 图标：tabBar 使用 `frontend/src/static/tab-src/` 中的统一线性 SVG 源文件，构建使用 `static/tab/*.png`；禁止用 1×1 占位图或混用 Emoji 作为主导航图标。替换图标内容后应同步更换文件名（如 `*-v2.png`），避免 H5/微信开发者工具继续使用同 URL 的旧缓存。
- H5 响应式：`>=900px` 隐藏原生顶部栏和底部 tabBar，使用全局 `WebSidebar` 左侧导航；业务内容最大宽度 `880px`。登录页没有侧栏，必须抵消 220px 桌面内容偏移并保持视口居中。
- 功能图标与分类图标分离：导航/钱包/日期/备注等功能使用统一 SVG；用户分类可使用 Emoji，但必须放进固定尺寸、淡色背景的图标容器。
- AI 首页：空状态保留欢迎语和三条快捷指令；底部输入区必须使用统一的圆角悬浮输入卡，不铺满成突兀的白色工具条。文字/语音切换使用功能 SVG 图标，发送使用圆形状态按钮；两种模式共用同一容器和高度，避免切换时布局跳变。语音维持微信式「按住说话、松开结束、上滑取消」。
- 记一笔：采用「收支切换 → 大金额 → 分类/账户/日期/备注 → 保存」固定层级，后续功能不得把金额降为普通表单项。
- 新增页面或组件时，先复用 `App.vue` 中的 `.card / .btn / .btn-primary / .btn-secondary / .input`，不要单独发明新色值。

---

## 9. ⚠️ 踩坑与纠正记录（重点，务必对齐，避免重复犯错）

### 后端

1. **SQLite 无法给已有数据的表加 `NOT NULL` 且无默认值的列**。所有新增的 `ledger_id` gorm tag 必须带 `default:0`，AutoMigrate 才能加列（旧行填 0），再由迁移回填。
2. **迁移必须幂等 + `Unscoped`**：回填 ledger_id 要独立于「是否新建个人账本」（否则第二次启动的孤儿行漏掉）；用 `db.Unscoped()` 连软删除行一起回填。
3. **时区/按天分组 −1 天**：纯日期按 `time.Local` 存午夜，sqlite 以 UTC 存储，`strftime` 按天分组会偏前一天。手动记账只有日期时仍补成当天 `12:00`；AI 记账会补当前时分秒。两者都避免存午夜导致日期偏移。MySQL(`loc=Local`) 基本不受影响，主要是 SQLite 现象。
4. **流水筛选日期不能让 Gin 自动绑定 `time.Time`**：前端账户明细和报表日历传 `start/end=YYYY-MM-DD`，Gin 会默认按 RFC3339 解析并提前返回 `parsing time ... cannot parse ... as T`。`service.ListRequest.Start/End` 必须使用 `form:"-"`，由 `handler/transaction.go` 的 `parseTime` 手动解析；分页参数要用 `omitempty`，否则默认值逻辑执行前就会被校验拦截。
4. **gin 路由冲突**：`/transactions/:id/push`（param）与 `/transactions/push-month`（静态）在同一段会 panic。批量推送用**独立路径 `/push-month`**。
5. **GORM 查询默认带软删除作用域**（`deleted_at IS NULL`）；迁移回填软删除行需 `Unscoped()`；空结果 slice 序列化为 `null`，需在 service 里初始化为 `[]T{}`（如 `CategoryBreakdown`）。
6. **环境变量优先级高于 config.yaml**——dev.sh 曾自动注入 `DASHSCOPE_API_KEY` 覆盖了 config.yaml 的对话 key，导致 key 与 base_url 不匹配 401。已移除 dev.sh 的注入；排查 401 先查是否有 env 覆盖。
7. **百炼 MaaS 专属部署路径是 `/compatible-mode/v1`**，不是 `/api/v1`（后者 404）。
8. **语音识别不能用 Whisper `/audio/transcriptions`**（百炼 404），必须走 `chat/completions` + `qwen-audio-3.0-asr-flash` + `input_audio` data URI（见 §7.8）。

### 前端（多为 uni-app H5 的坑）

1. **flex 子项 `<input>` 撑破容器 / 横向溢出**：`flex:1` 的 input 默认 `min-width:auto`（约 170px，字号大更宽），会把卡片撑破、整页横向滚动，导致右对齐内容跑出屏幕。解法：给这类 input 加 `**min-width:0`**（record 的金额/备注）；顶层根容器 H5 加全局 `overflow-x:hidden`（在 `body/uni-page-body` 层，加在页面根 view 无效——**横向滚动发生在 body**）。
2. `**<scroll-view>` 在 flex 里会撑出自身宽度**、和面板配合时超界被裁。短列表（如预算设置、账户选择）改用**普通 `<view>` + `overflow-y:auto`**，并给列表/行加 `width:100%; box-sizing:border-box`。
3. **底部弹出弹层被 tabBar 遮挡**：选择/表单类弹层统一用**居中卡片**（`align-items:center; justify-content:center`，`max-height:80vh`，flex 列 + 头/按钮固定、中间可滚），不要用底部 sheet。
4. **uni `<picker>` 在 H5 弹层定位会跑偏**：账户/成员等选择改用**自定义弹层**（mask+panel），不用 uni picker。
5. **全屏 mask 会盖住按钮触发 `mouseleave` 误操作**：录音浮层等全屏遮罩加 `**pointer-events:none`**，否则鼠标"离开"按钮触发取消。
6. `**height:100vh` 在 tabBar 页面包含 tabBar 高度**，导致底部输入栏被顶出屏幕。解法：页面用 `height:100%` + 全局 `page, uni-page-body { height:100% }`（chat 页）。
7. **导航栏标题出现闪烁文本光标**：uni H5 标题可选中，鼠标悬停显示 I-beam。全局（#ifdef H5）给 `uni-page-head`* 设 `user-select:none; caret-color:transparent; cursor:default`（App.vue）。
8. **后端返回 `null` 列表前端 `.map/.length` 崩**：所有列表赋值加 `|| []` 兜底（report/list），后端也尽量返回 `[]`。
9. **直接给 uni `<input>` 设 `width` 有时不生效**：用固定宽度的**外层容器** + `input{width:100%}`（预算金额输入框）。
10. **查看账本状态要在 onShow 校验**：明细/报表的 `viewLedgerId` 若指向已退出/失效的账本要**回落个人账本**（`resetToPersonal`），否则退出家庭账本后卡在家庭视图、切不回个人。每次进入默认个人账本。
11. 发送按钮 loading 时文字被挤：`{{ sending ? '' : '发送' }}` 只显示转圈。

19.5. **uni-app `<input>` 在 H5 下高度异常，导致无法点击或文字向下错位**：`<input>` 组件在 H5 下被渲染为 `<uni-input>` 自定义元素，内部包裹 `<div class="uni-input-wrapper">` 和真实的 `<input>`。只设置其中一层会出现外层高度为 0/22px、内部高度 24px 并溢出的情况，表现为无法聚焦，或 placeholder、光标被边框裁切。
    - **现象**：`getBoundingClientRect().height === 0`，点击无反应
    - **关键结论**：`v-model` **本身是正常工作的**（值能正确绑定到 data、能提交到后端）。问题**只在 DOM 高度**——不要因为"输入框点不动"就怀疑 v-model、把它改成 `:value` + `@input`，那样反而会破坏绑定（uni-app 的 `@input` 事件 `e.detail.value` 在 H5 下不一定触发）。
    - **正确解法**：保留 `v-model`；CSS 同时给外层组件和 `.uni-input-wrapper` 明确相同高度并使用 flex 垂直居中，内层真实 input 固定为 24px。H5 若仍被运行时样式覆盖，再在 `mounted` 用 JS 兜底设置内层高度：
      ```css
      .my-input { height: 50px; }
      :deep(.my-input .uni-input-wrapper) {
        height: 50px;
        padding: 0 14px;
        display: flex;
        align-items: center;
        box-sizing: border-box;
      }
      :deep(.my-input .uni-input-input) {
        height: 24px;
        line-height: 24px;
      }
      ```
      ```js
      mounted() {
        this.$nextTick(() => setTimeout(() => this.fixInputHeight(), 100))
      },
      methods: {
        fixInputHeight() {
          document.querySelectorAll('.my-input input').forEach(input => {
            input.style.height = '24px'
            input.style.width = '100%'
            input.style.display = 'block'
            input.style.padding = '0'
            input.style.border = 'none'
            input.style.background = 'transparent'
          })
        }
      }
      ```
      边框/圆角/背景用 `:deep(.my-input .uni-input-wrapper)` 设置。不要用上下 padding 撑高度，也不要只给真实 input 设置高度；三层尺寸必须配套。
    - **密码框**：uni-app 的 `<input>` 用 `password` 布尔属性（`<input type="text" password>`），不要用 `type="password"`（type 只支持 text/number/idcard/digit 等）。
    - **切换显示**（如登录↔注册）后 input 重新渲染，要在 `v-if` 切换后再次调用 `fixInputHeight()`。
    - **影响范围**：所有用到 `<input>` 的页面（登录/注册、记账、搜索、弹层表单）。

19.6. **登录/注册接口 401 的错误提示**：后端登录失败返回 `401 + {"error":"用户名或密码错误"}`，但 `request.js` 默认把所有 401 当"token 过期"处理（跳登录页 + 显示"未登录或登录已过期"），导致用户看不到真实错误。解法：在 `request.js` 里判断 `url` 是否 `/auth/login` `/auth/register`，是则**只显示后端 error、不跳转、不清 token**。

### 后端补充

19.7. **个人账本 `invite_code` 空字符串触发 UNIQUE 冲突**：`ledgers.invite_code` 有 uniqueIndex，Go string 零值是 `""`（不是 NULL）。第一个用户注册时个人账本 invite_code="" 存入成功，**第二个用户注册时空字符串再次出现，UNIQUE 约束冲突，注册 400 失败**（user 已创建但 ledger 创建失败，留下无账本的孤儿用户）。**所有创建个人账本的地方都必须生成 invite_code**：
    - `service/auth.go` 的 `Register`：已修
    - `repository/db.go` 的 `migrateToLedgers`：**极易漏**！它给孤儿用户补建个人账本，若不设 invite_code，多个孤儿用户会全部冲突、补建失败，用户永远没有个人账本 → 前端报"未找到个人账本"。已修，用 `model.GenInviteCode()`
    - 邀请码生成函数统一放 `model.GenInviteCode()`（避免 `repository`→`service` 循环依赖），service 里的 `generateRandomString` 只是它的别名
    - `middleware.RequireLedger` 还要**兜底**：查不到个人账本时**自动创建**（个人账本默认就该有，不应报错），不要返回 400"未找到个人账本"。已修（`getOrCreatePersonalLedger`）
    - 或彻底解法：把 `InviteCode` 改成 `*string` 让空值为 NULL（NULL 不参与 UNIQUE），但改动大，推荐前者

19.8. **MySQL 驱动配置不能直接用结构体零值**：`go-sql-driver/mysql.Config{}` 会把 `AllowNativePasswords` 留为 `false`，RDS 账号使用 `mysql_native_password` 时会报 `could not use requested auth plugin`。构造 DSN 必须从 `mysql.NewConfig()` 开始再覆盖连接字段；`DB_PASSWORD="..."` 在 systemd `EnvironmentFile` 中的外层双引号会被正确去除，不是该错误的原因。`internal/config/config_test.go` 已覆盖认证默认值和特殊字符密码。

19.9. **RDS 库/表不能使用 `utf8mb3`**：分类图标包含 emoji，若 RDS 数据库创建时沿用了 `utf8mb3_general_ci`，图标同步会报 MySQL 3988。MySQL 建表统一指定 `utf8mb4_unicode_ci`；启动迁移会检测所有模型表，只转换仍为 utf8/utf8mb3 的表，已经是 utf8mb4 的表不会重复执行 `ALTER TABLE`。创建 RDS 数据库时仍应直接选择 `utf8mb4`，避免首次启动转换。

### 微信小程序（uni-app 编译到小程序的坑）

1. **域名必须 HTTPS + 已备案 + mp 后台配置为合法域名**：`request` / `uploadFile` / `downloadFile` 三类域名**分别配置**，漏一个对应功能就报错（`url not in domain list`）。
2. **录音授权 `scope.record`（已实现）**：H5 使用 `MediaRecorder`；微信小程序必须单独使用 `uni.getRecorderManager()`，不能只把 H5 代码放在条件编译中，否则按钮触摸事件会触发但没有任何录音行为。切换到语音模式时先通过 `uni.getSetting` / `uni.authorize` 申请权限，用户拒绝后引导 `uni.openSetting`。小程序录音固定为单声道 16kHz MP3，`onStop` 后用 `uni.getFileSystemManager().readFile(..., encoding:'base64')` 上传；支持按住录音、松开结束、上滑取消和 `touchcancel`。
3. **隐私协议必须前置（代码已实现）**：进入语音模式先 `getPrivacySetting`，未同意则展示用途弹层，并通过 `open-type="agreePrivacyAuthorization"` 授权后再申请麦克风。mp 后台仍要完成「小程序用户隐私保护指引」，声明麦克风、本地存储及服务端语音识别用途。
4. **内容安全审核**：AI 对话与备注属于用户输入，首版提交前应依据当前类目/审核要求确定微信内容安全 API 或阿里云内容安全方案；若审核反馈要求，优先在后端统一封装，不能把 AppSecret 放前端。
5. **小程序包体积**：当前构建约 540KB，明显低于主包限制，首版暂不分包。每次发布仍运行微信开发者工具「代码质量/依赖分析」；接近限制后再把非 tabBar 页面迁入分包（tabBar 页面本身必须在主包）。
6. **微信 openid 登录（当前启用）**：个人主体无手机号快速验证组件权限，登录页调用 `uni.login` 获取一次性 `code`，提交 `POST /auth/wechat {code}`；后端向微信换取可信 `openid`，按 openid 登录或自动注册，并创建个人账本与默认数据。`wx.login` 不会直接返回头像昵称；首次登录后使用 `<button open-type="chooseAvatar">` 与 `<input type="nickname">` 由用户主动选择，再分别调用 `PUT /me` 和 `POST /me/avatar` 保存。`users.wechat_open_id` 为可空唯一字段。手机号授权接口 `/auth/wechat-phone` 保留供未来主体认证后启用，但当前页面不调用；不得接受前端直接上传的明文手机号作为微信身份依据。H5 本地调试继续保留用户名密码登录。
7. **禁止启动即登录（已实现）**：小程序默认进入 AI 功能浏览页，游客可切换查看记一笔、明细、报表和我的；不得在启动、切 tab 或单纯浏览时调用 `wx.login`、索取手机号或跳转登录页。登录必须由用户主动使用需要账号的数据功能后选择，登录页必须保留“暂不登录，继续浏览”。每次提审前在构建产物中确认 `provider:"weixin"` 只出现在 `pages/login/index.js`。
8. **小程序 storage 有容量限制**：单个 key 上限未明确但总上限约 10MB，不要把大列表（如全量流水）存 storage；用 uni.setStorage 存 token/用户/当前账本即可。
9. **小程序 request 并发限制**：同时最多 10 个进行中的请求，超出的排队。明细页按月懒加载 + 报表页同时拉多个统计时注意别并发轰炸，`common/request.js` 可加并发控制。
10. **web-view 业务域名**：如果用 web-view 嵌入 H5 报表页，域名要在 mp 后台「业务域名」配置（与 request 合法域名分开），且域名需备案 + 放校验文件到根目录。
11. **小程序图片格式**：webp 支持但动图 webp 不推荐，base64 内联图片会增大包体积。**推荐**：图片统一走 OSS/CDN，不要放 `/static`。
12. **小程序真机与模拟器差异**：开发者工具正常 ≠ 真机正常。录音、蓝牙、文件、权限等**必须真机调试**。iOS 和 Android 行为也有差异（尤其录音格式：iOS 默认 aac，Android 默认 aac/mp3，要显式指定 `format: "mp3"`）。
13. **弹层禁止依赖 `@click.self`**：uni-app 当前编译到微信小程序后可能只生成外层 `bindtap`，丢失 self 判断；点击输入框、分类 Tab 等子元素会冒泡关闭弹层，表现为“秒退”或“输入没反应”。统一写成外层 `@tap="closeXxx"`、内容容器 `@tap.stop`，并确认生成 WXML 是外层 `bindtap` + 内容 `catchtap`。
14. **浏览器 API 必须条件编译**：`document/window/fetch/Blob/FileReader/MediaRecorder` 不得进入小程序产物。报表导出在 H5 使用 fetch + Blob，在小程序使用 `uni.downloadFile + uni.openDocument`；录音同理分为 H5 MediaRecorder 和小程序 RecorderManager。执行 `cd frontend && pnpm check:mp-weixin` 会构建并检查所有输入 `bindinput`、弹层 `catchtap`、浏览器 API 残留及个人主体误用手机号组件。

---

## 10. 端到端自测片段（新环境验证用）

用独立 sqlite 库起后端，覆盖核心链路：

```bash
cd backend
# 推荐：先 go build -o ./accounter-bin ./cmd/server
DB_DRIVER=sqlite SQLITE_PATH=./e2e-test.db JWT_SECRET=dev-secret SERVER_PORT=18080 ./accounter-bin &
bash e2e_test.sh
```

脚本覆盖：注册个人账本 → 创建/加入旅游账本 → 个人记账 → 目标账本直接记账被拒 → 单笔同步 → 重复同步拒绝 → 单独删除副本不影响源 → 重新同步 → 删源级联删副本 → 当月 overview。

---

## 11. 后续路线（分优先级）

### 上线前必做（P0，小程序 + 网页端首版）

- ~~**微信小程序账号体系**~~：已采用 `wx.login code → openid` 登录，首次登录自动创建个人账本，兼容个人主体。上线前仍需配置 AppID/AppSecret 并完成真机验证。
- ~~**小程序录音授权流程**~~：用途声明、隐私授权、`scope.record`、拒绝后 `openSetting` 已实现；仍需真机验收。
- ~~**小程序隐私协议弹窗代码**~~：已实现；后台隐私保护指引仍需人工填写。
- **内容安全策略确认**：依据提交时所选类目和微信审核要求，决定接入微信内容安全 API 或阿里云内容安全；若接入，统一放在后端，不能暴露 AppSecret。
- **生产配置脱敏**：API Key 改用环境变量，JWT_SECRET 强密码（见 §13.2）。
- ~~**补全缺失索引**~~：`source_id`、`(ledger_id, occurred_at)`、`(ledger_id, op_type)` 已在模型 tag 中（见 §14.3）。
- ~~**小程序包体**~~：当前约 540KB，无需为首版强行分包。

### 上线后短期优化（P1，首版后 1-2 月）

- 角色细分（管理员 admin / 只读访客 viewer）；账本内昵称 `alias`（字段已预留）。
- 允许清理家庭账本里「非同步的历史直接记账」（`source_id` 为空的遗留数据，目前家庭账本只读删不掉）。
- 余额并发安全加固：关键事务加 `SELECT FOR UPDATE`（见 §14.1）。
- 日志收集 + 监控告警：阿里云 SLS + CloudMonitor（AI 接口成功率、5xx 错误率）。
- 限流：`/auth/*` 和 `/ai/*` 加 IP 限流（见 §15）。

### 中长期按需（P2，根据用户反馈）

- 反向同步（改家庭副本回写个人）、推送时手动选账户/分类映射。
- 图片识别记账（qwen-vl）、定时/周期记账、预算超支推送。
- 净资产走势、支付宝/微信账单导入。
- 邮件推送与 SQL 备份到 OSS。

---

## 12. 约定

- 全部中文注释与中文回复。
- 金额一律 `decimal`（前端以字符串传金额，避免精度丢失）。
- 改动后：后端 `cd backend && go build ./...` 必须通过；前端保存即热更新。
- `config.yaml` 不入库；生产只使用 `/etc/accounter/env`。开发密钥若曾复制到聊天、日志或共享文件，上线前全部轮换。

---

## 13. 上线 Checklist（网页端 + 小程序，按顺序推进）

### 13.1 阿里云基础设施（前置，耗时最长的是域名备案）

- [ ] **域名购买 + ICP 备案**（国内服务器必需，阿里云备案需 7-20 个工作日，**最先启动**）
- [ ] **SSL 证书**：Let's Encrypt 免费证书（certbot 自动续签）或阿里云免费 DV 证书（1 年）
- [ ] **ECS 安全组**：开放 80/443，**3306 仅对内网开放**（白名单加 ECS 内网 IP）
- [ ] **RDS 白名单**：同上，只允许 ECS 内网 IP 访问
- [ ] **RDS 数据库与账号**：建 `accounter` 库（utf8mb4）+ 应用账号（非 root，最小权限）
- [ ] **OSS（可选）**：静态资源加速 / 语音文件暂存，按需开通

### 13.2 后端上线

- [ ] **生产配置脱敏**：复制 `deploy/env.example` 到 `/etc/accounter/env`，填值后权限设为 600
- [ ] **轮换现有密钥**：重新生成微信 AppSecret、百炼/ASR Key；不要沿用开发期已暴露的值
- [ ] **JWT_SECRET 强度**：`openssl rand -hex 32`，至少 32 字节
- [ ] **DB_DRIVER=mysql + RDS 配置**：确认走 RDS 内网地址；可用 DB_HOST/PORT/USER/PASSWORD/NAME，密码特殊字符会由 DSN 构造器正确处理
- [ ] **CORS 白名单**：同源部署留空；只有 H5 分域时设置 `CORS_ALLOWED_ORIGINS`
- [ ] **安全工具链**：使用项目锁定的 Go 1.26.5 或更高版本构建，禁止用 1.26.0 发布
- [ ] **systemd 服务文件**：`Restart=always`、`User=accounter`、日志输出到 journald
- [ ] **健康检查**：`GET /health` 验证 DB 连通 + AI 配置可用
- [ ] **日志收集**：推荐 journald + 阿里云 SLS，或简单的 logrotate
- [ ] **第一次启动**：验证 AutoMigrate 在 MySQL 上正常、种子数据（`service/seed.go`）正确写入

### 13.3 网页端（H5）上线

- [ ] **构建**：`cd frontend && pnpm install --frozen-lockfile && pnpm build:h5`
- [ ] **部署到 `/opt/accounter/web`**：Nginx 同源托管
- [ ] **HTTPS 强制跳转**：Nginx 80 → 443
- [ ] **H5 的 `BASE_URL`**：同源部署留空（`common/config.js` 空值走相对路径）
- [ ] **验证核心链路**：注册→登录→记账→查询→报表→AI 对话→语音

### 13.4 微信小程序上线（最繁琐，按清单逐项）

- [ ] **注册小程序 AppID**：mp.weixin.qq.com，选"企业"或"个体工商户"主体（个人主体类目受限）
- [ ] **小程序类目与资质**：以微信公众平台当前可选类目和页面提示为准，提交前确认个人主体可选范围、服务内容与类目一致，不在文档里写死可能变化的资质要求
- [ ] **服务器域名配置**（mp 后台 → 开发 → 开发管理 → 开发设置）：
  - `request 合法域名`：`https://accounter.yourdomain.com`
  - `downloadFile 合法域名`：同上（报表导出用）；当前语音通过 `request` 上传 base64，未来改用 `uploadFile` 时再配置对应域名
- [ ] **发布版本号**：仓库根目录 `VERSION` 是唯一版本记录；发布前按 `1.0.9 → 1.0.10` 递增，并同步 `frontend/package.json` 的 `version`、`frontend/src/manifest.json` 的 `versionName` 和数字版 `versionCode`。正式构建会校验三处，不一致直接失败
- [ ] **小程序正式构建**：`VITE_API_BASE_URL=https://你的域名 pnpm build:release:mp-weixin`；脚本会拒绝占位域名及版本记录不一致
- [ ] **录音授权 `scope.record`**：H5 用 MediaRecorder，小程序用 `uni.getRecorderManager`，需要在 mp 后台「隐私协议」中声明「麦克风」权限用途
- [ ] **隐私保护指引**：在 mp 后台如实填写收集清单和用途（包括麦克风、必要的本地存储及服务端语音识别），并与代码中的授权提示保持一致
- [ ] **内容安全策略**：根据当前类目、开放能力和审核反馈，选择微信内容安全 API 或阿里云内容安全；接入点放在后端，首版至少要有长度限制、错误处理和敏感日志脱敏
- [ ] **微信登录配置**：`frontend/src/manifest.json` 填小程序 AppID；ECS 环境变量配置 `WECHAT_APP_ID`、`WECHAT_APP_SECRET`（Secret 绝不能放前端或提交 Git）。当前个人主体使用 `POST /api/auth/wechat {code}`，不依赖手机号权限；未来完成非个人主体认证后可选启用 `/api/auth/wechat-phone`。
- [ ] **支付能力（按需）**：如果未来做会员/付费功能，需要开通微信支付商户号
- [ ] **真机联调**：微信开发者工具 → 真机调试（注意录音授权、网络请求 HTTPS、文件上传大小限制）
- [ ] **体验版内测**：上传代码 → 设为体验版 → 添加体验成员 → 走完整业务流程
- [ ] **提交审核并发布**：功能完整、无测试数据、隐私协议齐全、内容安全策略明确；审核通过后仍需在后台点击发布

### 13.5 数据与备份

- [ ] **RDS 自动备份**：保留 7 天
- [ ] **关键操作前手动快照**：升级/迁移前
- [ ] **定期 SQL 备份到 OSS**：`mysqldump` + cron，异地备份

---

## 14. 核心风险与对策（开发阶段就要考虑）

### 14.1 余额并发安全（**高优先级**）

**问题**：`applyBalance` 用 `UPDATE accounts SET balance = balance + ?` 方式累加，MySQL 的行锁可以保护单条记录。但如果同一账户在多端同时记账（比如用户手机+网页同时操作），`Create` 事务里 `applyBalance(+1)` 之间会串行化，**本身不会算错余额**。
**真正的风险**：`Update` 里 `applyBalance(old,-1)` → `Updates` → `applyBalance(next,+1)`，如果另一个事务在这三步之间也改了同一账户，可能看到不一致的中间状态。
**对策**：

- 用 `SELECT ... FOR UPDATE` 锁定账户行，或在事务开头 `db.Clauses(clause.Locking{Strength: "UPDATE"}).First(&account, accountID)`
- 或在 GORM 事务开头 `Set("gorm:query_option", "FOR UPDATE")`
- 关键：**先实现基础功能，上线前压测，发现竞态再加锁**（记账 QPS 低，初期风险可控）

### 14.2 同步级联性能

**问题**：`syncCopies(src)` 在改源后要遍历所有副本并逐一重算余额。如果一个家庭账本有 N 个成员各自同步过同一笔流水，修改源会触发 N 次 DB 事务。
**对策**：

- 当前 MVP 阶段家庭账本成员少（<10），性能可接受
- 后续优化：批量 `Updates` + 单事务处理所有副本
- 索引：`transactions.source_id`、`(ledger_id, occurred_at)`、`(ledger_id, op_type)` 已加（见下）

### 14.3 流水索引（已补）

模型 tag 已包含：

- `Transaction.SourceID` → `index`
- `Transaction.LedgerID + OccurredAt` → 复合索引 `idx_ledger_occurred`
- `Transaction.LedgerID + OpType` → 复合索引 `idx_ledger_op`
- `Transaction.AccountID` → `index`

### 14.4 LedgerID default:0 的校验漏洞

**问题**：为了 SQLite 迁移妥协的 `default:0`，意味着如果 service 层忘了设 LedgerID，数据会悄悄写入 ledger_id=0 而不是报错。
**对策**：

- 所有 service 方法入口加 `if ledgerID == 0 { return errors.New("ledgerID required") }`
- 或在 GORM `BeforeCreate` hook 里校验
- AutoMigrate 完成后，考虑迁移脚本把 default 去掉

### 14.5 AI 服务降级

**问题**：AI 记账依赖阿里云百炼 API，如果 API 故障或配额耗尽，核心功能不可用。
**对策**：

- 前端 AI 记账失败时，引导用户使用「记一笔」（手动记账）页面
- 后端 `/ai/chat` 返回明确错误码（503 + 重试提示），不要吞异常
- 监控 AI 接口成功率，配置阿里云 CloudMonitor 告警

### 14.6 小程序包体积

**现状**：当前微信小程序构建约 540KB，首版无需分包。
**对策**：

- 接近限制时再开启分包；tabBar 页面必须保留在主包，不要把 list/report/profile 直接迁走而破坏 tabBar
- 图片走 CDN 或后端托管，不要打进包里
- 使用小程序分析工具（mp 后台 → 工具 → 代码依赖分析）检查体积

### 14.7 时间/时区（开发期就要对齐）

**问题**：文档 §9.3 提到的"纯日期补成 12:00"是 SQLite 的 workaround。**生产用 MySQL 后，`loc=Local` 在 DSN 里，MySQL 会按服务器时区存储**。
**对策**：

- ECS 时区设置为 `Asia/Shanghai`：`timedatectl set-timezone Asia/Shanghai`
- Go 服务启动时 `time.Local = shanghai`（显式，不依赖系统）
- 前端传日期字符串 `"2026-07-15"`，后端统一按 `time.Date(y,m,d,12,0,0,0,shanghai)` 解析
- 按天统计用 `DATE(occurred_at)` 而不是 `strftime`（MySQL 方言，已在 `periodExpr` 处理）

---

## 15. 安全清单（上线前必须逐项检查）

- [ ] **config.yaml 不入 git**：`.gitignore` 加 `backend/configs/config.yaml`，提供 `config.yaml.example` 模板
- [ ] **API Key 轮换**：上线前轮换一次（开发期可能已泄露）
- [ ] **JWT_SECRET 强度**：≥ 32 字节随机
- [ ] **HTTPS 强制**：HTTP 301 → HTTPS（Nginx 配置）
- [ ] **RDS 仅内网**：公网不开放 3306
- [ ] **ECS SSH**：禁用密码登录，仅密钥；改默认端口；fail2ban
- [ ] **CORS 白名单**：不写 `*`，明确域名
- [ ] **限流**：`/auth/`* 和 `/ai/*` 加 IP 限流（防刷注册、防 AI 配额耗尽），推荐 golang.org/x/time/rate 或 gin-middleware/limit
- [ ] **输入校验**：所有 handler 用 `c.ShouldBindJSON` + struct tag `binding:"required"`，避免注入
- [ ] **AI 内容安全**：接阿里云绿网或微信内容安全 API（见 §13.4）
- [ ] **前端依赖审计**：当前 `pnpm audit --prod` 无 critical，剩余告警来自 uni-app/Vite 等上游构建依赖；生产 ECS 只部署预构建静态文件，不运行 Vite/Node 开发服务。每次升级 DCloud 依赖后重新审计，禁止把开发服务器暴露到公网
- [ ] **敏感日志脱敏**：不打 API Key、不打完整 JWT
- [ ] **定期更新系统包**：`unattended-upgrades`（Ubuntu）或 cron `yum update`

---

## 16. AI 试用与激活码（已实现）

### 16.1 产品规则

- 每个用户账号赠送 **7×24 小时** AI 试用，注册或登录时不开始计时；第一次成功完成 AI 对话或语音识别后才写入开始时间。
- 试用到期只关闭 AI 对话和语音识别，手动记账、账户、明细和报表继续可用。
- 激活码成功兑换后永久绑定当前用户；同一账号换设备登录仍有效，一个激活码只能绑定一个用户。
- 为控制异常成本，默认试用账号每天最多 50 次、已激活账号每天最多 500 次成功 AI 调用。语音识别与后续 AI 对话是两次独立调用，均计数。
- 权益状态只有 `not_started`、`trial`、`expired`、`activated` 四种，由时间字段推导，不在数据库保存容易失真的冗余状态。

### 16.2 数据与安全

- `ai_entitlements` 保存用户的试用开始、到期与永久激活时间。
- `ai_daily_usages` 按用户和自然日保存成功调用数；失败的模型请求不启动试用，也不扣次数。
- `activation_codes` 只保存以 `AI_ACTIVATION_SECRET` 计算的 HMAC-SHA256、末四位和管理信息，**不保存明文激活码**。
- `ai_activation_attempts` 保存脱敏 IP HMAC 与失败原因；默认同一用户每小时最多失败 5 次、同一 IP 每小时最多失败 30 次，记录保留 30 天。
- Nginx 是唯一可信代理，Gin 只信任 `127.0.0.1` 和 `::1` 的转发头，防止客户端伪造来源 IP 绕过限流。

### 16.3 生产环境变量

在 `/etc/accounter/env` 中增加：

```bash
# 使用 openssl rand -hex 32 生成；启用激活码后不得随意轮换
AI_ACTIVATION_SECRET=填写独立的随机值
AI_TRIAL_DAYS=7
AI_TRIAL_DAILY_LIMIT=50
AI_ACTIVATED_DAILY_LIMIT=500
AI_ACTIVATION_ATTEMPT_LIMIT=5
AI_ACTIVATION_IP_ATTEMPT_LIMIT=30
```

`AI_ACTIVATION_SECRET` 与 JWT 密钥用途不同，必须分别生成。该值若丢失或更换，数据库中尚未兑换的旧激活码将无法匹配；因此只应存放在 ECS 环境文件和安全备份中，禁止提交 Git。MySQL 生产模式启动时会拒绝短于 32 位或仍为占位值的密钥。

### 16.4 管理命令

发布后由 root 在 ECS 使用 `acli`（软链接到 `/opt/accounter/bin/accounter-admin`）：

```bash
# 查询注册用户总数，以及每个用户的账户数、流水数和收支汇总
sudo acli ops users --page 1 --page-size 50

# 按用户 ID、月份分页查询账单；默认隐藏备注
sudo acli ops bills \
  --user-id 12 --month 2026-08 --page 1 --page-size 50

# 确有排查需要时显示备注
sudo acli ops bills --user-id 12 --show-remark

# 生成一批激活码，明文只写入一次且文件权限为 600
sudo acli activation-code create \
  --count 20 --batch first-release --expires 2027-01-01 \
  --output /root/qingzhang-first-release.csv

# 查询未兑换、已兑换、已过期或已作废的记录
sudo acli activation-code list --status unused
sudo acli activation-code list --status redeemed

# 作废未兑换的码；已兑换的码不能通过此命令撤销
sudo acli activation-code revoke --id 123
```

`ops` 查询只读生产数据库，只统计个人账本中的原始流水；同步到旅游、礼账本等其他账本的副本不会重复计数。命令不会输出密码、微信 OpenID 等登录凭据，微信登录生成的内部用户名也会隐藏。分页上限为每页 200 条。

生成命令采用排他创建，不会覆盖已有 CSV。创建后只有 CSV 中存在完整明文；数据库和列表命令都无法恢复完整激活码。建议按渠道建立批次名，发放时记录对象，未发完的码可按 ID 作废。

### 16.5 接口与前端约定

- `GET /api/ai/entitlement`：读取当前用户权益和当日用量。
- `POST /api/ai/activate {"code":"QZ-..."}`：兑换一次性激活码。
- `/api/ai/chat` 与 `/api/ai/transcribe` 在调用模型前检查权益；到期返回 `403 / AI_ACCESS_EXPIRED`，每日额度耗尽返回 `429 / AI_DAILY_LIMIT_REACHED`。
- “AI 记账”页负责试用提醒和到期拦截；“我的 → AI 功能权益”负责显示状态、用量和输入激活码。

### 16.6 上线检查

- [ ] 发布前先做 RDS 快照；首次启动会自动创建四张 AI 权益相关表。
- [ ] ECS 已设置 `AI_ACTIVATION_SECRET`，再执行发布脚本；脚本会在构建前检查该配置。
- [ ] 用管理命令只生成 1 个测试码，确认 CSV 权限为 `600`、列表只显示末四位。
- [ ] 新用户首次进入显示“试用尚未开始”；第一次成功 AI 请求后显示 7 天倒计时。
- [ ] 测试码兑换后显示永久激活，并确认同一个码不能兑换到另一个用户。
- [ ] 确认 AI 到期或额度耗尽时，手动记账、明细和报表仍然可用。
