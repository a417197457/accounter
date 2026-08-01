# accounter 开发文档（供新环境 / 新模型接手）

> AI 中文记账 App：一句话记账 · 语音记账 · 智能查询 · 报表可视化 · 家庭共享账本。
> 本文档目标：让接手者**不看历史对话**也能完整理解现状、对齐所有功能与约定，并避免重复踩坑。
> **上线目标**：优先网页端 + 微信小程序；部署在**阿里云 ECS + RDS MySQL**。
> 仓库是 git 仓库，改动请走标准 git 工作流（commit / branch / PR）。

---

## 1. 技术栈与目录

| 层 | 技术 |
|---|---|
| 后端 | Go 1.24 + Gin + GORM。生产 MySQL；开发用纯 Go SQLite（`github.com/glebarez/sqlite`，无 CGO） |
| 前端 | uni-app（Vue3，Options API），一套代码出 H5 + 微信小程序；构建 vite；`@dcloudio/*` 为 alpha 版 |
| AI 对话 | 阿里云百炼 DashScope，OpenAI 兼容接口，`github.com/sashabaranov/go-openai` + Function Calling |
| AI 语音 | 百炼 `qwen3-asr-flash`，走 OpenAI 兼容 `chat/completions` + `input_audio`（**非** Whisper 接口） |
| 报表 | `github.com/xuri/excelize/v2` |
| 认证 | `github.com/golang-jwt/jwt/v5` |
| 金额 | `github.com/shopspring/decimal`（DB 用 `decimal(18,2)`） |
| 配置 | `github.com/spf13/viper` |

```
accounter/
├── backend/
│   ├── cmd/server/main.go          # 入口：装配 service/ai/handler，AI 配置来源，启动 gin
│   ├── configs/config.yaml         # 配置底座（含真实 key，注意脱敏）
│   └── internal/
│       ├── model/model.go          # 全部 GORM 模型 + AllModels()
│       ├── config/config.go        # viper 加载 + 环境变量覆盖 + 默认值
│       ├── repository/db.go        # NewDB + AutoMigrate + migrateToLedgers（账本迁移）
│       ├── middleware/middleware.go# JWT鉴权 + RequireLedger账本上下文 + CORS
│       ├── service/                # account/auth/category/transaction/stats/budget/ledger/setting/seed
│       ├── ai/                     # ai.go(对话) + asr.go(语音) + tools.go(function定义)
│       ├── report/excel.go         # excelize 导出
│       └── handler/                # HTTP 层 + router.go
├── frontend/src/
│   ├── pages/{chat,record,list,report,profile,login}/
│   ├── common/{request.js,config.js}
│   ├── pages.json                  # 路由 + tabBar
│   └── App.vue                     # 全局样式（含多个 H5 修复）
├── deploy/                         # Dockerfile / docker-compose.yml / .env
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
- `DB_DRIVER=sqlite`、`SQLITE_PATH=backend/dev.db`、`JWT_SECRET=dev-secret`
- **⚠️ 不再自动注入 `DASHSCOPE_API_KEY`**（历史上它读 `~/.bailian/config.json` 注入，会**覆盖 config.yaml 的对话 key** 导致 401，已移除，见 §9 踩坑）。AI 配置一律走 `backend/configs/config.yaml`。

**本地开发工作流**：
1. **后端热重载**（可选）：`cd backend && go run ./cmd/server` 或安装 `air`（`go install github.com/cosmtrek/air@latest` → `air` 自动监听文件变化重启）
2. **前端热重载**：`cd frontend && npm run dev:h5`（vite，自动刷新）
3. **数据库管理**：
   - 查看数据：`sqlite3 backend/dev.db`（命令行）或用 DB Browser for SQLite（GUI）
   - 重置数据：删除 `backend/dev.db` → 重启后端自动 AutoMigrate + 种子数据
   - 备份：`cp backend/dev.db backend/dev.db.bak`
4. **测试 AI 功能**：确保 `backend/configs/config.yaml` 中 `ai.api_key` 有效（阿里云百炼控制台申请）
5. **浏览器访问**：`http://localhost:5173`（vite 代理 `/api/*` 到 `:8080`）

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

### 生产（Docker）
`cd deploy && cp .env.example .env`（填 `DASHSCOPE_API_KEY`/`JWT_SECRET`）→ `docker compose up -d --build`（:8080）。
Dockerfile 三阶段：构建 H5 → 构建 Go → alpine 运行，`STATIC_DIR=/app/static` 让后端同源托管 H5。

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
    ├── backend/accounter            # Go 二进制
    ├── backend/configs/config.yaml  # 生产配置（权限 600，owner=accounter）
    ├── backend/static/              # H5 构建产物（前端 build 后拷贝过来）
    └── logs/                        # 应用日志
  /etc/nginx/sites-available/accounter.conf
  /etc/systemd/system/accounter.service
  ```

**RDS MySQL 侧**：
- 实例规格：起步 1C2G 够用（记账 QPS 低），按量付费或包年
- 白名单：只允许 ECS 内网 IP（同 VPC 走内网，免流量费、延迟 < 1ms）
- 账号：为应用单独建账号（非 root），只给目标库 DML + DDL 权限
- 数据库：`accounter`，字符集 `utf8mb4`（支持 emoji 备注）
- 备份：RDS 自动备份保留 7 天，关键变更前手动打快照

**config.yaml 生产版**（环境变量覆盖更推荐，见下）：
```yaml
server:
  port: 8080
db:
  driver: mysql
  dsn: "accounter:PASSWORD@tcp(rm-xxx.mysql.rds.aliyuncs.com:3306)/accounter?charset=utf8mb4&parseTime=True&loc=Local"
jwt:
  secret: "生产用 openssl rand -hex 32 生成"
  expire: 168h
ai:
  base_url: "..."
  api_key: "..."
  model: "qwen-max"
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
    root /opt/accounter/backend/static;
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

**CORS**：网页端同源部署后不需要跨域；小程序走微信服务器也不涉及浏览器 CORS。仅当 H5 独立部署到别的域名时，需在 `middleware.CORS` 加白名单。

### 配置系统（`config/config.go`）
**优先级：环境变量 > config.yaml > 代码默认值。** `applyEnvOverrides` 逐个 `os.Getenv` 覆盖。

`config.yaml` 的 `ai` 段（**对话与语音是两套独立接入**）：
```yaml
ai:
  base_url: "..."       # 对话/记账/查询模型
  api_key: "..."
  model: "qwen-max"
  asr:                  # 语音识别（可与对话不同的 base_url/api_key）
    base_url: ""        # 留空则复用对话模型的 base_url
    api_key: ""         # 留空则复用对话模型的 api_key
    model: "qwen3-asr-flash"
```
环境变量：`DB_DRIVER DB_DSN SQLITE_PATH DB_HOST/PORT/USER/PASSWORD/NAME JWT_SECRET SERVER_PORT STATIC_DIR DASHSCOPE_API_KEY AI_BASE_URL AI_MODEL AI_ASR_BASE_URL AI_ASR_API_KEY AI_ASR_MODEL`。
默认值：driver=mysql、port=8080、jwt.expire=168h、ai.model=qwen-max、asr.model=qwen3-asr-flash、asr.base_url/key 回落对话配置（在 `applyEnvOverrides` 末尾填充）。

---

## 3. 数据模型（`model/model.go`，全部进 `AllModels()` 走 AutoMigrate）

隔离维度：**业务数据（account/category/transaction/budget）按 `ledger_id` 隔离**，不是 user_id。

```go
User        { ID, Username(uniq), PasswordHash(json:"-"), Nickname, 软删除 }
Ledger      { ID, Name, OwnerID, IsPersonal bool, InviteCode(size:16), CreatedAt }   // 账本
Membership  { ID, LedgerID, UserID, Role(owner|member), Alias, uniqueIndex(ledger,user) } // 成员关系
Account     { ID, LedgerID, UserID(创建者), Name, Type, Balance decimal(18,2), Sort, IsArchived, 软删除 }
Category    { ID, LedgerID, UserID, Name, ParentID *uint(二级), Kind(income|expense), Icon, Sort, 软删除
              Children []Category `gorm:"-"` }  // 树在内存组装
Transaction { ID, LedgerID, UserID(记账/归属成员), AccountID, ToAccountID*(转账转入),
              CategoryID*, OpType, Amount decimal, Remark, OccurredAt, SourceID *uint, 软删除
              Account/Category/Member 关联(Preload 用) }
Budget      { ID, LedgerID, UserID, CategoryID *uint(nil=总预算), Amount decimal }  // 月度循环预算
Setting     { Key(pk), Value text }  // 全局 KV（AI 配置持久化用，见 §7.9）
```

- **OpType**（`ValidOpType` 校验）：`expense 支出 / income 收入 / borrow_in 借入 / borrow_out 借出 / transfer 转账`。
- **CategoryKind**：`income | expense`。
- **Role**：`owner 创建者 / member 成员`。
- `Transaction.SourceID`：非空表示这是从别的账本「同步」来的副本，指向源流水 id（防重复推送 + 单向镜像，见 §7.8）。
- 各 `LedgerID` 的 gorm tag 必须带 `default:0`（否则 SQLite 给已有数据表加 NOT NULL 列失败，见 §9）。

### 老数据迁移（`repository/db.go: migrateToLedgers`，每次启动跑，幂等）
1. 为每个「没有个人账本」的 user 建个人账本（`IsPersonal=true`）+ owner 成员关系。
2. **幂等回填**：把该 user 名下 `ledger_id=0 或 NULL` 的 account/category/transaction/budget 更新为其个人账本 id。用 **`db.Unscoped()`** 连软删除行一并回填（否则软删除的旧行 ledger_id 永远为 0）。
   - 关键：回填**独立于**「是否新建了个人账本」，不能只在新建分支里做，否则第二次启动产生的孤儿行不会被回填。

---

## 4. 认证与账本上下文（`middleware/middleware.go`）

- `Auth(secret)`：解析 `Authorization: Bearer <jwt>`，注入 `userID`（claim `uid`）。`NewToken` 签发，默认 168h。
- `RequireLedger(db)`：读请求头 **`X-Ledger-Id`**；为空则用该用户的**个人账本**（`owner_id=uid AND is_personal=true`）；非空则校验 `Membership(user,ledger)` 存在，否则 403「你不是该账本成员」。注入 `ledgerID`。
- `middleware.UserID(c)` / `middleware.LedgerID(c)` 取上下文。

**路由分两组**（`router.go`）：
- **账本管理组**（仅 `Auth`，不绑定具体账本）：`/ledgers*`。
- **业务组**（`Auth + RequireLedger`，按 ledger 隔离）：accounts / categories / transactions / stats / budgets / report / ai。

---

## 5. 完整 API 列表（均在 `/api` 下）

公开：
- `POST /auth/register {username,password,nickname?}` → `{token,user}`（注册即建个人账本+播种默认账户/分类）
- `POST /auth/login` → `{token,user}`
- `GET /health` → `{status,ai}`

账本管理（需登录）：
- `GET /ledgers` → 我加入的账本列表 `[{...ledger, role, member_count}]`（个人账本在前）
- `POST /ledgers {name}` → 创建家庭账本（生成邀请码 + 播种默认账户/分类）
- `POST /ledgers/join {code}` → 邀请码加入（幂等）
- `GET /ledgers/:id/members` → 成员列表 `[{user_id,username,nickname,alias,role}]`
- `DELETE /ledgers/:id/members/:uid` → owner 移除成员
- `POST /ledgers/:id/leave` → 成员退出（owner/个人账本不可退）

业务（需登录 + `X-Ledger-Id`，缺省=个人账本）：
- 账户 `GET/POST/PUT /accounts/:id/DELETE`
- 分类 `GET/POST/PUT /categories/:id/DELETE`（`?kind=income|expense`）
- 流水 `GET /transactions`（筛选 `start end account_id category_id member_id op_type keyword page page_size`）、`POST`、`PUT/:id`、`DELETE/:id`
  - `POST /transactions/:id/push {target_ledger_id}` → 单笔推送到目标账本
  - `POST /push-month {month:"YYYY-MM", target_ledger_id}` → 整月批量推送（**独立路径**，避开 `:id` 路由冲突）
- 统计 `GET /stats/overview|trend|category|monthly`（overview 返回 income/expense/balance/total_assets；trend `?granularity=day|month`；category `?kind=`；monthly `?months=`）
- 预算 `GET /budgets`（附本月已用 spent）、`PUT /budgets {category_id,amount}`、`DELETE /budgets/:id`
- 报表 `GET /report/monthly?month=YYYY-MM`、`GET /report/export?<筛选>`（返回 xlsx）
- AI `POST /ai/chat {message}`、`POST /ai/transcribe {audio(base64),format}`、`POST /ai/confirm <draft>`、`GET/PUT /ai/settings`

响应统一 `{data:...}`（`handler.ok`）或 `{error:...}`（`handler.fail`）；前端 `request.js` 自动剥 `data`。

---

## 6. 关键业务规则（务必对齐）

1. **记账只在个人账本**：`CreateTransaction` 和 `AIConfirm` 开头用 `LedgerService.IsPersonal(lid)` 校验，非个人账本直接记账一律拒绝「家庭账本不能直接记账，请在个人账本记账后同步过来」。家庭账本数据**只能通过同步**（`PushToLedger` 走内部 `Create`，不经该 handler，不受限）。
2. **前端无全局"当前账本"切换**：记账页（record/chat）请求不带 `X-Ledger-Id` → 后端默认个人账本。只有**明细/报表**页可切换查看账本（只读）。
3. **家庭账本明细只读**：查看家庭账本时不能删除/推送/整月同步，仅展示 + 显示记账成员。
4. **同步是单向镜像**：源（个人账本）→ 家庭副本。改源自动改副本，删源级联删副本；改副本不影响源。

---

## 7. 各功能模块细节

### 7.1 账户 `service/account.go`
按 `ledger_id` 隔离。`Create(ledgerID, userID, a)`（其余方法 `(ledgerID, ...)`）。删除前检查该账户下有无流水（`ledger_id AND (account_id OR to_account_id)`），有则拒绝（可归档）。列表 `Order("sort asc, id asc")`。

### 7.2 分类 `service/category.go`
二级树（`ParentID` 空为一级）。`ListTree` 内存组装 children；`ListFlat` 供 AI 上下文。`Create(ledgerID, userID, c)` 校验 parent 归属同账本，子类继承父类 kind。删除前检查有无流水 / 子分类。

### 7.3 流水与余额同步 `service/transaction.go`（核心）
- `balanceDelta(op,amount)`：income/borrow_in 为 +amount，expense/borrow_out/transfer 为 -amount。
- `applyBalance(tx, t, sign)`：在**数据库事务内**调整账户余额，`Where("id=? AND ledger_id=?", accountID, t.LedgerID)`；转账额外给 `ToAccountID` 加钱。
- `Create(ledgerID, userID, t)`：设 LedgerID/UserID → `validate`（账户/分类归属按 ledger 校验，金额>0，转账需不同转入账户）→ 事务内 Create + applyBalance(+1)。
- `Update(ledgerID, id, next)`：事务内 applyBalance(old,-1) → Updates → applyBalance(next,+1)；保留原 UserID/CreatedAt；**更新后调 `syncCopies(next)`**。
- `Delete(ledgerID, id)`：事务内 applyBalance(-1)+删除；**删除后调 `deleteCopies(id)`**。
- `List(ledgerID,f)`：`Preload("Account").Preload("Category").Preload("Member")`，分页，`Order("occurred_at desc, id desc")`。`TxFilter.MemberID` 支持按记账成员筛选。

### 7.4 统计 `service/stats.go`
全部按 `ledger_id`。`periodExpr` 按方言：sqlite `strftime`，mysql `DATE_FORMAT`。`CategoryBreakdown` 用 Raw SQL 把子类金额归并到一级分类，**无数据时返回 `[]CategorySlice{}` 而非 nil**（否则 JSON null，前端 `.map` 崩，见 §9）。`Monthly` 补齐缺月为 0。`Overview` 的 total_assets = 该账本未归档账户余额合计。

### 7.5 报表 `service/report/excel.go` + `handler/report.go`
复用 `TxFilter` 拉全量流水写 xlsx。导出走 `exportExcel(c, ledgerID, f, ...)`。前端 H5 用 fetch+blob 下载、小程序用 downloadFile+openDocument，**都要带 `X-Ledger-Id` 头**才能导出所查看账本。

### 7.6 预算 `service/budget.go` + `handler/budget.go`
按月循环（不绑定具体月份）。`Set(ledgerID, userID, categoryID, amount)` upsert（categoryID nil=总预算）。`ListBudgets` 返回每条 `{amount, spent}`，spent 用本月 `CategoryBreakdown`（总预算=本月总支出）。前端设置弹层批量 upsert/删除。

### 7.7 AI 对话记账 `ai/ai.go` + `ai/tools.go`
- `Service` 支持运行时热更新配置（读写锁）。**两套配置**：对话 `client/model/apiKey/baseURL` + 语音 `asrModel/asrAPIKey/asrBaseURL`。`Configure(base,key,model)` 配对话；`ConfigureASR(base,key,model)` 配语音（空则回落对话配置，故 NewService 里**先 Configure 再 ConfigureASR**）。
- `Chat(ctx, ledgerID, userID, msg)`：`buildSystemPrompt(ledgerID)` 把账户/分类/今天日期塞进 system prompt；两个 tool（`create_transaction` / `query_transactions`）。记账**不直接落库**，返回 `create_confirm` 草稿；查询走 stats 返回汇总。
- `ConfirmDraft(ledgerID, userID, draft)`：落库（handler 已拦非个人账本）。
- 相对时间：`normalizeTime` 把纯日期补成 `12:00`（见 §9 时区）。

### 7.8 语音识别 `ai/asr.go`
- **百炼不支持 Whisper 风格 `/audio/transcriptions`（404）**。走 OpenAI 兼容 `chat/completions`，body：`{model:qwen3-asr-flash, messages:[{role:user, content:[{type:"input_audio", input_audio:{data:"data:audio/<fmt>;base64,<b64>"}}]}]}`，返回 `choices[0].message.content` 即文本。
- `Transcribe(ctx, audioB64, format)` 用 `net/http` 直接调（**不经 go-openai**，因 data URI 结构特殊），复用 asr 那套 base_url/key/model；`audioMime()` 映射格式→MIME（mp3/wav/webm/m4a/aac/ogg…）。
- ASR 端点路径注意：百炼「专属部署/MaaS」域名（`llm-xxx.cn-beijing.maas.aliyuncs.com`）兼容路径是 **`/compatible-mode/v1`**，不是 `/api/v1`。

### 7.9 AI 设置 `handler/ai.go` + `service/setting.go`
`GET/PUT /ai/settings` 支持应用内改对话 base_url/model/api_key（存 Setting 表、热生效、脱敏返回）。**当前前端已移除该入口**（配置走 config.yaml），但后端接口与 `main.go` 里「DB Setting 覆盖」逻辑保留（想恢复页面配置时前端加回即可）。ASR 配置只走配置文件，不支持页面配置。

### 7.10 账本共享 / 同步 `service/ledger.go` + `transaction.go`
- `LedgerService`：`ListForUser / Create(邀请码) / Join / Members / RemoveMember / Leave / Role / IsPersonal / PersonalLedgerID`。邀请码 `genInviteCode` 用 `crypto/rand` 生成 6 位（去掉易混字符）。
- **推送/同步**（`transaction.go`）：
  - `mapAccountToLedger` / `mapCategoryToLedger`：按**名称**把账户/分类映射到目标账本（账户找不到用第一个，分类找不到留空）。
  - `PushToLedger(srcLedgerID, srcID, targetLedgerID, userID)`：复制一份到目标账本，`SourceID=srcID`，防重复（目标已有该 source 则拒绝）。转账不支持推送。
  - `PushMonth(srcLedgerID, month, targetLedgerID, userID)`：遍历该月 income/expense 流水逐笔 PushToLedger，返回 `{pushed, skipped}`。
  - `syncCopies(src)`：改源后同步所有副本（重映射账户/分类 + 撤销旧余额+应用新余额）。
  - `deleteCopies(srcID)`：删源后级联删副本。
- 副本记账人 UserID = 执行推送的人 → 家庭账本明细可显示「谁同步的」。

---

## 8. 前端要点

- **`common/request.js`**：统一封装 uni.request，注入 JWT、401 跳登录、剥 `{data}`。**账本上下文按调用显式传 `options.ledgerId`**（→ 头 `X-Ledger-Id`），不传=个人账本。read 类 api（overview/trend/categoryStats/monthly/transactions/budgets/categories/setBudget/deleteBudget）都有可选 `ledgerId` 末参。**不再有全局"当前账本"注入**。
- **`common/config.js`**：`BASE_URL`（H5 空走 vite 代理，小程序/生产填后端地址）、`TOKEN_KEY`、`USER_KEY`、`LEDGER_KEY`（已基本弃用）。
- **页面**（`pages.json` 5 个 tab + 登录）：
  - `chat` AI 记账：文字输入 + **微信式「按住说话」语音**（H5 MediaRecorder 录 webm / 小程序 getRecorderManager 录 mp3，上滑取消，识别后自动发送）；记账确认卡片有「确认保存/忽略(🗑️)」，保存后「✅ 已保存」。
  - `record` 记一笔：账户/转入账户用**自定义居中弹层**（非 uni picker）；日期用 mode=date picker。记账固定个人账本。
  - `list` 明细：**按月折叠**（顶部累计总览 + 每月卡片，点开懒加载该月流水）；顶部**查看账本 tab**（个人+家庭，默认个人）；个人账本视图长按流水→「推送到家庭/删除」、月卡「⇪同步」整月推送；家庭账本视图只读、显示记账成员+「同步」标记。
  - `report` 报表：月份切换 + 支出/收入 tab + 每日趋势竖条 + 分类占比 + 预算超支 + 近6月对比；顶部查看账本 tab；导出 Excel。
  - `profile` 我的：用户信息 + **我的账本**（列表/创建/邀请码加入/成员管理，无全局切换）+ 退出登录。**已移除账户管理**。
  - `login`：登录/注册，登录时 `removeStorageSync(LEDGER_KEY)`。
- 记账时账户默认顺序（seed）：**支付宝 → 银行卡 → 微信钱包 → 现金**（现金最后，第一个为默认；`service/seed.go`）。

---

## 9. ⚠️ 踩坑与纠正记录（重点，务必对齐，避免重复犯错）

### 后端
1. **SQLite 无法给已有数据的表加 `NOT NULL` 且无默认值的列**。所有新增的 `ledger_id` gorm tag 必须带 `default:0`，AutoMigrate 才能加列（旧行填 0），再由迁移回填。
2. **迁移必须幂等 + `Unscoped`**：回填 ledger_id 要独立于「是否新建个人账本」（否则第二次启动的孤儿行漏掉）；用 `db.Unscoped()` 连软删除行一起回填。
3. **时区/按天分组 −1 天**：纯日期按 `time.Local` 存午夜，sqlite 以 UTC 存储，`strftime` 按天分组会偏前一天。解法：**存储时把纯日期补成当天 12:00**（`handler/transaction.go` 的 `toModel` 和 `ai/ai.go` 的 `normalizeTime` 都这么做）。MySQL(loc=Local) 基本不受影响，主要是 sqlite 现象。
4. **gin 路由冲突**：`/transactions/:id/push`（param）与 `/transactions/push-month`（静态）在同一段会 panic。批量推送用**独立路径 `/push-month`**。
5. **GORM 查询默认带软删除作用域**（`deleted_at IS NULL`）；迁移回填软删除行需 `Unscoped()`；空结果 slice 序列化为 `null`，需在 service 里初始化为 `[]T{}`（如 `CategoryBreakdown`）。
6. **环境变量优先级高于 config.yaml**——dev.sh 曾自动注入 `DASHSCOPE_API_KEY` 覆盖了 config.yaml 的对话 key，导致 key 与 base_url 不匹配 401。已移除 dev.sh 的注入；排查 401 先查是否有 env 覆盖。
7. **百炼 MaaS 专属部署路径是 `/compatible-mode/v1`**，不是 `/api/v1`（后者 404）。
8. **语音识别不能用 Whisper `/audio/transcriptions`**（百炼 404），必须走 `chat/completions` + `qwen3-asr-flash` + `input_audio` data URI（见 §7.8）。

### 前端（多为 uni-app H5 的坑）
9. **flex 子项 `<input>` 撑破容器 / 横向溢出**：`flex:1` 的 input 默认 `min-width:auto`（约 170px，字号大更宽），会把卡片撑破、整页横向滚动，导致右对齐内容跑出屏幕。解法：给这类 input 加 **`min-width:0`**（record 的金额/备注）；顶层根容器 H5 加全局 `overflow-x:hidden`（在 `body/uni-page-body` 层，加在页面根 view 无效——**横向滚动发生在 body**）。
10. **`<scroll-view>` 在 flex 里会撑出自身宽度**、和面板配合时超界被裁。短列表（如预算设置、账户选择）改用**普通 `<view>` + `overflow-y:auto`**，并给列表/行加 `width:100%; box-sizing:border-box`。
11. **底部弹出弹层被 tabBar 遮挡**：选择/表单类弹层统一用**居中卡片**（`align-items:center; justify-content:center`，`max-height:80vh`，flex 列 + 头/按钮固定、中间可滚），不要用底部 sheet。
12. **uni `<picker>` 在 H5 弹层定位会跑偏**：账户/成员等选择改用**自定义弹层**（mask+panel），不用 uni picker。
13. **全屏 mask 会盖住按钮触发 `mouseleave` 误操作**：录音浮层等全屏遮罩加 **`pointer-events:none`**，否则鼠标"离开"按钮触发取消。
14. **`height:100vh` 在 tabBar 页面包含 tabBar 高度**，导致底部输入栏被顶出屏幕。解法：页面用 `height:100%` + 全局 `page, uni-page-body { height:100% }`（chat 页）。
15. **导航栏标题出现闪烁文本光标**：uni H5 标题可选中，鼠标悬停显示 I-beam。全局（#ifdef H5）给 `uni-page-head*` 设 `user-select:none; caret-color:transparent; cursor:default`（App.vue）。
16. **后端返回 `null` 列表前端 `.map/.length` 崩**：所有列表赋值加 `|| []` 兜底（report/list），后端也尽量返回 `[]`。
17. **直接给 uni `<input>` 设 `width` 有时不生效**：用固定宽度的**外层容器** + `input{width:100%}`（预算金额输入框）。
18. **查看账本状态要在 onShow 校验**：明细/报表的 `viewLedgerId` 若指向已退出/失效的账本要**回落个人账本**（`resetToPersonal`），否则退出家庭账本后卡在家庭视图、切不回个人。每次进入默认个人账本。
19. 发送按钮 loading 时文字被挤：`{{ sending ? '' : '发送' }}` 只显示转圈。
19.5. **uni-app `<input>` 在 H5 下高度为 0 导致无法点击**：`<input>` 组件在 H5 下被渲染为 `<uni-input>` 自定义元素，内部包裹 `<div class="uni-input-wrapper">` 和真实的 `<input>`。CSS 只作用到外层 `<uni-input>`，内部真实 `<input>` 高度为 0，导致无法点击/聚焦。
    - **现象**：`getBoundingClientRect().height === 0`，点击无反应
    - **关键结论**：`v-model` **本身是正常工作的**（值能正确绑定到 data、能提交到后端）。问题**只在 DOM 高度**——不要因为"输入框点不动"就怀疑 v-model、把它改成 `:value` + `@input`，那样反而会破坏绑定（uni-app 的 `@input` 事件 `e.detail.value` 在 H5 下不一定触发）。
    - **正确解法**：保留 `v-model`，在 `mounted` 用 JS 给内部 input 设置高度：
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
      边框/圆角/背景用 `:deep(.my-input .uni-input-wrapper)` 设置（外层），高度用 JS 设置（内层）。
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

### 微信小程序（uni-app 编译到小程序的坑）
20. **域名必须 HTTPS + 已备案 + mp 后台配置为合法域名**：`request` / `uploadFile` / `downloadFile` 三类域名**分别配置**，漏一个对应功能就报错（`url not in domain list`）。
21. **录音授权 `scope.record`**：`uni.getRecorderManager().start` 首次调用会弹授权窗，用户拒绝后**必须引导去设置页手动打开**（`uni.openSetting`），不能直接再调 start（静默失败）。录音开始前先 `uni.getSetting` 查一下，没授权则弹自定义引导。
22. **隐私协议必须前置**：小程序首次启动必须展示隐私协议弹窗，用户同意后才能调用录音/位置等敏感 API，否则审核不过。mp 后台「隐私协议」要填收集清单（麦克风、本地存储）。
23. **内容安全审核**：用户输入（AI 对话、备注）和 AI 输出**必须**过内容安全 API（`msgSecCheck` 文本 / `mediaCheckAsync` 图片音频），否则审核不过或被封禁。推荐统一封装 `/api/moderate` 接口。
24. **小程序包体积限制**：主包 2MB，总包 20MB。uni-app 默认构建产物 vendor.js 容易超 2MB。对策：**分包加载**（list/report/profile 放子包）、图片走 CDN、tree-shaking、开启「ES6 转 ES5」和「压缩代码」。
25. **wx.login 账号体系不同**：当前后端是 username+password，小程序推荐 `wx.login` 拿 code → 后端换 openid → 自动注册/绑定。需要新增 `POST /auth/wx-login {code}` 接口，User 表加 `openid` 字段（unique index，可空）。
26. **小程序 storage 有容量限制**：单个 key 上限未明确但总上限约 10MB，不要把大列表（如全量流水）存 storage；用 uni.setStorage 存 token/用户/当前账本即可。
27. **小程序 request 并发限制**：同时最多 10 个进行中的请求，超出的排队。明细页按月懒加载 + 报表页同时拉多个统计时注意别并发轰炸，`common/request.js` 可加并发控制。
28. **web-view 业务域名**：如果用 web-view 嵌入 H5 报表页，域名要在 mp 后台「业务域名」配置（与 request 合法域名分开），且域名需备案 + 放校验文件到根目录。
29. **小程序图片格式**：webp 支持但动图 webp 不推荐，base64 内联图片会增大包体积。**推荐**：图片统一走 OSS/CDN，不要放 `/static`。
30. **小程序真机与模拟器差异**：开发者工具正常 ≠ 真机正常。录音、蓝牙、文件、权限等**必须真机调试**。iOS 和 Android 行为也有差异（尤其录音格式：iOS 默认 aac，Android 默认 aac/mp3，要显式指定 `format: "mp3"`）。

---

## 10. 端到端自测片段（新环境验证用）

用独立 sqlite 库起后端，覆盖核心链路（示例，用 curl）：
```bash
cd backend && env -u DASHSCOPE_API_KEY DB_DRIVER=sqlite SQLITE_PATH=/tmp/t.db JWT_SECRET=x SERVER_PORT=8099 go run ./cmd/server &
# 1 注册→得个人账本；2 创建家庭账本→邀请码；3 另一账号 join；4 个人账本记账；
# 5 push/push-month 同步到家庭；6 家庭账本汇总=各成员合计、member_id 筛选；
# 7 家庭账本直接记账应 400；8 改源→副本跟随、删源→副本级联删；9 越权访问家庭账本 403
```
验证要点：家庭汇总正确、成员筛选正确、越权 403、个人/家庭隔离、直接记账被拒、同步镜像正确、每日趋势日期不偏移。

---

## 11. 后续路线（分优先级）

### 上线前必做（P0，小程序 + 网页端首版）
- **微信小程序账号体系**：`wx.login` → 后端换 openid → 自动注册/绑定；User 表加 `openid` 字段（unique，可空），新增 `POST /auth/wx-login {code}` 接口。
- **小程序录音授权流程**：开始前 `uni.getSetting` 检查，未授权则自定义引导 → `uni.openSetting`（见 §9.21）。
- **小程序隐私协议弹窗**：首次启动展示，同意后才能调用录音等敏感 API。
- **内容安全 API 接入**：AI 对话输入输出走 `msgSecCheck`（见 §13.4）。
- **生产配置脱敏**：API Key 改用环境变量，JWT_SECRET 强密码（见 §13.2）。
- **补全缺失索引**：`transactions.source_id`、`(ledger_id, occurred_at)`、`(ledger_id, op_type)`、`account_id`（见 §14.3）。
- **小程序分包**：主包控制在 2MB 内，list/report/profile 放子包（见 §9.24）。

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
- config.yaml 内含真实 key，分享/提交前务必脱敏并轮换。

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
- [ ] **生产配置脱敏**：config.yaml 中的 `ai.api_key`、`jwt.secret` 改用环境变量（`/etc/accounter/env`，权限 600）
- [ ] **JWT_SECRET 强度**：`openssl rand -hex 32`，至少 32 字节
- [ ] **DB_DRIVER=mysql + DB_DSN**：确认走 RDS 内网地址
- [ ] **CORS 白名单**：同源部署后只需放行小程序 `wx****.com`（如有 webview）
- [ ] **systemd 服务文件**：`Restart=always`、`User=accounter`、日志输出到 journald
- [ ] **健康检查**：`GET /health` 验证 DB 连通 + AI 配置可用
- [ ] **日志收集**：推荐 journald + 阿里云 SLS，或简单的 logrotate
- [ ] **第一次启动**：验证 AutoMigrate 在 MySQL 上正常、种子数据（`service/seed.go`）正确写入

### 13.3 网页端（H5）上线
- [ ] **构建**：`cd frontend && npm run build:h5`
- [ ] **部署到 `/opt/accounter/backend/static`**：Nginx 同源托管
- [ ] **HTTPS 强制跳转**：Nginx 80 → 443
- [ ] **H5 的 `BASE_URL`**：同源部署留空（`common/config.js` 空值走相对路径）
- [ ] **验证核心链路**：注册→登录→记账→查询→报表→AI 对话→语音

### 13.4 微信小程序上线（最繁琐，按清单逐项）
- [ ] **注册小程序 AppID**：mp.weixin.qq.com，选"企业"或"个体工商户"主体（个人主体类目受限）
- [ ] **小程序类目选择**：工具 → 记账属于「工具 > 记账」，可能需要「增值电信业务经营许可证」或「软件企业认定」，**提前确认资质**
- [ ] **服务器域名配置**（mp 后台 → 开发 → 开发管理 → 开发设置）：
  - `request 合法域名`：`https://accounter.yourdomain.com`
  - `uploadFile/downloadFile 合法域名`：同上（报表导出用）
- [ ] **小程序 `BASE_URL`**：`common/config.js` 在小程序构建时填生产地址（`#ifdef MP` 分支）
- [ ] **录音授权 `scope.record`**：H5 用 MediaRecorder，小程序用 `uni.getRecorderManager`，需要在 mp 后台「隐私协议」中声明「麦克风」权限用途
- [ ] **隐私协议**：mp 后台填写用户隐私收集清单（麦克风、本地存储），否则审核不过
- [ ] **内容安全**：AI 对话的输入输出建议走「微信内容安全 API」（msgSecCheck / mediaCheckAsync）或阿里云绿网，**否则小程序审核不过**
- [ ] **账号体系**：小程序推荐 `wx.login` → 后端换 openid → 绑定/自动注册。当前后端用 username+password，需要新增 `POST /auth/wx-login {code}` 接口
- [ ] **支付能力（按需）**：如果未来做会员/付费功能，需要开通微信支付商户号
- [ ] **真机联调**：微信开发者工具 → 真机调试（注意录音授权、网络请求 HTTPS、文件上传大小限制）
- [ ] **体验版内测**：上传代码 → 设为体验版 → 添加体验成员 → 走完整业务流程
- [ ] **提交审核**：功能完整、无测试数据、隐私协议齐全、内容安全合规，通常 1-7 天

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
- 加索引：`transactions.source_id` 必须加索引（当前缺失，见下）

### 14.3 缺失索引（**上线前必须补**）
当前 model 定义中缺失以下索引，上线前必须在 gorm tag 中补上：
- `Transaction.SourceID` → `index`（`syncCopies` / `deleteCopies` / 去重查询都依赖）
- `Transaction.LedgerID + OccurredAt` → 复合索引（明细列表、月度统计高频查询）
- `Transaction.LedgerID + OpType` → 复合索引（PushMonth 按 income/expense 筛选）
- `Transaction.AccountID` → 索引（账户流水关联查询）

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
**问题**：微信小程序主包限制 2MB，总包 20MB。uni-app 打包时 vendor 很容易超标。
**对策**：
- 开启 uni-app 的「分包加载」：把非 tabBar 页面（list/report/profile）放到子包
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
- [ ] **限流**：`/auth/*` 和 `/ai/*` 加 IP 限流（防刷注册、防 AI 配额耗尽），推荐 golang.org/x/time/rate 或 gin-middleware/limit
- [ ] **输入校验**：所有 handler 用 `c.ShouldBindJSON` + struct tag `binding:"required"`，避免注入
- [ ] **AI 内容安全**：接阿里云绿网或微信内容安全 API（见 §13.4）
- [ ] **敏感日志脱敏**：不打 API Key、不打完整 JWT
- [ ] **定期更新系统包**：`unattended-upgrades`（Ubuntu）或 cron `yum update`
