# 第一版部署模板（ECS + RDS MySQL）

生产架构采用 Nginx 同域托管 H5，并将 `/api` 反向代理到仅监听 ECS 本机的 Go 服务。小程序也请求同一个 HTTPS 域名。

1. RDS 创建 `utf8mb4` 数据库与非 root 应用账号，只把 ECS 内网 IP 加入白名单。
2. ECS 创建 `accounter` 系统用户及 `/opt/accounter/bin`、`/opt/accounter/web` 目录。
3. 使用项目锁定的 Go 1.26.5 或更高版本交叉编译后端与管理工具：`cd backend && CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -trimpath -ldflags="-s -w" -o ../dist/accounter ./cmd/server && CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -trimpath -ldflags="-s -w" -o ../dist/accounter-admin ./cmd/admin`。如果 ECS 是 ARM 实例，把 `GOARCH` 改为 `arm64`。
4. 构建网页端：`cd frontend && pnpm install --frozen-lockfile && pnpm build:h5`，上传 `dist/build/h5/` 到 `/opt/accounter/web/`。
5. 将 `deploy/env.example` 复制为 `/etc/accounter/env`，填真实值并设置权限 `600`。其中 `AI_ACTIVATION_SECRET` 使用 `openssl rand -hex 32` 单独生成，投入使用后不要更换，否则尚未兑换的激活码会失效。
6. 安装 `deploy/accounter.service`，执行 `systemctl daemon-reload && systemctl enable --now accounter`。
7. 替换 Nginx 模板中的域名和证书路径，检查后重载 Nginx。
8. 先验证 `https://域名/health`、网页注册/登录/记账/报表，再构建小程序正式包。
9. 小程序正式构建前先递增仓库根目录 `VERSION`，并同步 `frontend/package.json` 与 `frontend/src/manifest.json`；再执行 `VITE_API_BASE_URL=https://域名 pnpm build:release:mp-weixin`，导入 `frontend/dist/build/mp-weixin` 上传体验版。构建脚本会拒绝版本记录不一致。

## 后续一键发布

首次部署完成并配置好 SSH 密钥后，可在本地项目根目录执行：

```bash
PUBLIC_BASE_URL=https://www.qingzhang.top ./deploy/release.sh qingzhang-ecs
```

脚本会自动识别 ECS 的 amd64/arm64 架构，运行后端测试和静态检查，构建 Go 后端与 H5，上传并替换服务，然后检查 ECS 本机及公网 `/health`。`/etc/accounter/env`、Nginx 和 HTTPS 证书不会被脚本覆盖。上一版会保存在 `/opt/accounter/backups/<发布时间>/`，新版本健康检查失败时自动回滚。

生产日志同时写入 journald 和 `/opt/accounter/logs/accounter.log`。业务写操作会输出 JSON 审计日志，包含用户、账本、路由、结果和请求 ID，但不记录 Token、密码、备注、AI 对话或语音内容。日志默认达到 20MB 自动轮转，最多保留 14 个压缩备份且不超过 30 天；参数可通过 `LOG_*` 环境变量调整。

```bash
# 实时查看全部生产日志
sudo tail -f /opt/accounter/logs/accounter.log

# 只看成功创建的手动/AI 记账流水
sudo grep '"event":"transaction_created"' /opt/accounter/logs/accounter.log

# 查看轮转后的压缩日志
sudo zgrep '"log_type":"audit"' /opt/accounter/logs/accounter-*.log.gz
```

首次切换 MySQL 会执行 GORM 自动迁移；正式变更前先做 RDS 快照。详细上线清单见根目录 `DEV_GUIDE.md`。

## 运维查询与 AI 激活码管理

发布脚本会把仅管理员可执行的工具安装到 `/opt/accounter/bin/accounter-admin`，并创建 `/usr/local/bin/acli` 软链接。它默认读取 `/etc/accounter/env` 并连接生产 RDS。

运维查询是只读操作。用户列表会显示注册用户总数、每个用户的资金账户数、流水数和收支汇总；账单可按用户 ID 和月份分页查询。统计只包含个人账本的原始流水，不重复计算同步到旅游、礼账本等其他账本的副本。默认隐藏账单备注，也不会输出密码、微信 OpenID 等登录凭据。

```bash
# 查询注册用户和每个用户的流水汇总
sudo acli ops users --page 1 --page-size 50

# 查询指定用户在指定月份的账单
sudo acli ops bills \
  --user-id 12 --month 2026-08 --page 1 --page-size 50

# 确有排查需要时才显示账单备注
sudo acli ops bills --user-id 12 --show-remark
```

激活码数据库只保存 HMAC，明文只在创建时输出一次：

```bash
# 创建 20 个三个月激活码，权益从兑换时开始；2027-01-01 当天仍可兑换
sudo acli activation-code create \
  --count 20 --duration-months 3 --batch first-release --expires 2027-01-01 \
  --output /root/qingzhang-first-release.csv

# duration-months 支持 1-120；不传或传 0 时生成永久激活码

# 只看未使用码；列表仅显示末四位，不泄露完整激活码
sudo acli activation-code list \
  --status unused --batch first-release

# 作废尚未兑换的激活码
sudo acli activation-code revoke --id 123
```

CSV 发给用户后应从聊天记录、临时目录等位置及时清理。已经兑换的码不能作废；如需取消某个用户的权益，应通过后续的独立后台功能处理，不直接改激活码记录。
