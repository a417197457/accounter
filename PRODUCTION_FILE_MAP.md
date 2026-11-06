# 账小鸭线上环境文件位置

> 最后核对：2026-09-03（Asia/Shanghai）  
> ECS SSH 别名：`qingzhang-ecs`  
> 线上域名：`https://www.qingzhang.top`  
> 本文只记录路径、用途和非敏感配置，不记录密码、Token、AppSecret 或数据库地址。

## 1. 快速索引

| 内容 | ECS 实际位置 | 用途 |
| --- | --- | --- |
| 生产环境配置 | `/etc/accounter/env` | 数据库、JWT、微信、AI 等运行参数 |
| systemd 服务 | `/etc/systemd/system/accounter.service` | 启动和守护后端服务 |
| 后端主程序 | `/opt/accounter/bin/accounter` | Go API 服务 |
| 运维 CLI | `/opt/accounter/bin/accounter-admin` | 用户、账单和激活码管理 |
| `acli` 命令 | `/usr/local/bin/acli` | 指向 `accounter-admin` 的软链接 |
| H5 网站 | `/opt/accounter/web/` | Nginx 直接提供的前端静态文件 |
| 用户上传文件 | `/opt/accounter/uploads/` | 持久化上传内容 |
| 用户头像 | `/opt/accounter/uploads/avatars/` | 微信头像和用户上传头像 |
| 应用日志 | `/opt/accounter/logs/accounter.log` | API、业务审计和错误日志 |
| 发布备份 | `/opt/accounter/backups/<发布号>/` | 上一版后端、CLI 和 H5 |
| Nginx 配置 | `/etc/nginx/conf.d/accounter.conf` | HTTPS、静态站点和 API 反向代理 |
| HTTPS 证书 | `/etc/nginx/ssl/accounter/fullchain.pem` | 公钥证书链 |
| HTTPS 私钥 | `/etc/nginx/ssl/accounter/privkey.key` | TLS 私钥，禁止复制或泄露 |
| Nginx 访问日志 | `/var/log/nginx/accounter-access.log` | HTTP 请求记录 |
| Nginx 错误日志 | `/var/log/nginx/accounter-error.log` | 代理、证书和静态文件错误 |
| systemd 日志 | `/var/log/journal/` | journald 持久化日志的磁盘目录 |

## 2. 生产配置

### 2.1 主配置文件

生产配置文件是：

```text
/etc/accounter/env
```

实际权限为 `600`，所有者为 `root:root`。后端进程不能修改该文件，systemd 启动服务时读取它。

查看配置变量名但不输出值：

```bash
ssh qingzhang-ecs \
  "sudo awk -F= '/^[A-Za-z_][A-Za-z0-9_]*=/{print \$1}' /etc/accounter/env"
```

当前文件包含以下几类变量：

- 服务：`SERVER_PORT`、`CORS_ALLOWED_ORIGINS`
- RDS MySQL：`DB_DRIVER`、`DB_HOST`、`DB_PORT`、`DB_USER`、`DB_PASSWORD`、`DB_NAME`
- 登录：`JWT_SECRET`、`JWT_EXPIRE`
- 微信：`WECHAT_APP_ID`、`WECHAT_APP_SECRET`
- AI：`AI_BASE_URL`、`DASHSCOPE_API_KEY`、`AI_MODEL`、`AI_ASR_*`
- AI 权益：`AI_ACTIVATION_SECRET`、`AI_TRIAL_*`、`AI_ACTIVATED_*`、`AI_ACTIVATION_*`

当前 `DB_DRIVER=mysql`，业务数据位于外部 RDS，不存在 ECS 本地数据库文件。RDS 备份、快照和恢复在云控制台管理，不在 `/opt/accounter/backups` 中。

当前 `/etc/accounter/env` 没有显式配置 `LOG_*` 和 `UPLOAD_DIR`，所以使用程序的生产默认值：

| 配置 | 当前有效值 |
| --- | --- |
| `LOG_FILE` | `/opt/accounter/logs/accounter.log` |
| `LOG_MAX_SIZE_MB` | `20` |
| `LOG_MAX_BACKUPS` | `14` |
| `LOG_MAX_AGE_DAYS` | `30` |
| `LOG_COMPRESS` | `true` |
| `UPLOAD_DIR` | 工作目录下的 `./uploads`，实际为 `/opt/accounter/uploads` |

安全修改配置：

```bash
ssh qingzhang-ecs
sudoedit /etc/accounter/env
sudo systemctl restart accounter
sudo systemctl status accounter --no-pager
curl -fsS https://www.qingzhang.top/health
```

不要把该文件复制到 Git、聊天记录或普通用户目录。发布脚本不会覆盖或备份它。

### 2.2 systemd 服务配置

```text
/etc/systemd/system/accounter.service
```

当前关键运行参数：

- 服务用户：`accounter:accounter`
- 工作目录：`/opt/accounter`
- 配置文件：`/etc/accounter/env`
- 启动程序：`/opt/accounter/bin/accounter`
- 异常退出后自动重启：开启

修改 service 文件后执行：

```bash
sudo systemctl daemon-reload
sudo systemctl restart accounter
```

## 3. 应用程序和静态文件

### 3.1 后端程序

```text
/opt/accounter/bin/accounter
```

这是正式 Go API 二进制。Nginx 通过 ECS 本机地址 `127.0.0.1:8080` 将请求转发给它。

### 3.2 运维 CLI

```text
/opt/accounter/bin/accounter-admin
/usr/local/bin/acli -> /opt/accounter/bin/accounter-admin
```

管理程序权限为 `750`、所有者为 `root:root`，通常使用 `sudo acli ...` 执行。它同样读取 `/etc/accounter/env` 并连接生产 RDS。

### 3.3 H5 前端

```text
/opt/accounter/web/index.html
/opt/accounter/web/assets/
```

Nginx 的网站根目录是 `/opt/accounter/web`。每次发布会整体替换该目录，不能直接在线修改构建后的 JS/CSS；正确方式是在本地修改源码后重新执行发布脚本。

### 3.4 小程序代码

微信小程序代码不部署在 ECS 文件系统。正式构建产物在本地：

```text
/Users/zhuoyu.yzy/GolangProject/accounter/frontend/dist/build/mp-weixin/
```

构建后通过微信开发者工具上传至微信公众平台。ECS 只提供小程序调用的 API 和上传文件。

## 4. 持久化文件

### 4.1 用户上传和头像

```text
/opt/accounter/uploads/
/opt/accounter/uploads/avatars/
```

这些目录归 `accounter:accounter` 所有、权限为 `750`。发布脚本不会替换它们，但发布备份也不会备份它们，应另行纳入 ECS 磁盘快照或文件备份。

仅查看目录占用：

```bash
ssh qingzhang-ecs 'sudo du -sh /opt/accounter/uploads'
```

### 4.2 数据库

生产使用 RDS MySQL，数据库连接信息仅保存在 `/etc/accounter/env`。ECS 上没有需要复制的 SQLite 或 MySQL 数据目录。

需要做数据备份时，应创建 RDS 快照或使用受控数据库备份工具；不要把数据库密码写进命令历史或文档。

## 5. 日志位置和轮转

### 5.1 应用日志

```text
/opt/accounter/logs/accounter.log
```

当前日志约 `1.8MB`，权限为 `640`，所有者为 `accounter:accounter`。程序会同时把日志写到该文件和 stdout，因此同一内容也能在 journald 中查看。

日志由应用自身轮转，不使用 `/etc/logrotate.d`：

- 单文件达到 `20MB` 时轮转；
- 最多保留 `14` 份；
- 最长保留 `30` 天；
- 历史日志使用 gzip 压缩；
- 历史文件格式约为 `/opt/accounter/logs/accounter-<时间>.log.gz`。

常用命令：

```bash
# 实时查看
ssh qingzhang-ecs 'sudo tail -f /opt/accounter/logs/accounter.log'

# 最近 200 行
ssh qingzhang-ecs 'sudo tail -n 200 /opt/accounter/logs/accounter.log'

# 查询成功创建的流水
ssh qingzhang-ecs \
  "sudo grep '\"event\":\"transaction_created\"' /opt/accounter/logs/accounter.log"

# 查询所有业务审计日志
ssh qingzhang-ecs \
  "sudo grep '\"log_type\":\"audit\"' /opt/accounter/logs/accounter.log"

# 查询轮转后的压缩日志
ssh qingzhang-ecs \
  "sudo zgrep '\"log_type\":\"audit\"' /opt/accounter/logs/accounter-*.log.gz"
```

### 5.2 systemd / journald 日志

日志查询入口：

```bash
sudo journalctl -u accounter
sudo journalctl -u accounter -f
sudo journalctl -u accounter -n 200 --no-pager
sudo journalctl -u accounter --since '2026-09-03 00:00:00'
```

持久化 journal 位于：

```text
/var/log/journal/
```

核对时整台 ECS 的 journal 总占用约 `3.9GB`，这个数字包含所有 systemd 服务，不只是账小鸭。查看占用：

```bash
sudo journalctl --disk-usage
```

### 5.3 Nginx 日志

项目专用日志：

```text
/var/log/nginx/accounter-access.log
/var/log/nginx/accounter-error.log
```

轮转配置：

```text
/etc/logrotate.d/nginx
```

当前规则为每天轮转、保留 `10` 份、压缩历史日志。常用命令：

```bash
sudo tail -f /var/log/nginx/accounter-access.log
sudo tail -f /var/log/nginx/accounter-error.log
sudo zgrep '关键字' /var/log/nginx/accounter-*.gz
```

`/var/log/nginx/access.log` 和 `/var/log/nginx/error.log` 是 Nginx 全局默认日志，不是账小鸭专用日志。

## 6. Nginx 和 HTTPS

### 6.1 Nginx 站点配置

```text
/etc/nginx/conf.d/accounter.conf
```

当前配置负责：

- `http://www.qingzhang.top` 跳转 HTTPS；
- `/opt/accounter/web` H5 静态文件；
- `/api/` 反向代理到 `127.0.0.1:8080`；
- `/health` 健康检查；
- 单次请求体上限 `12MB`。

修改后必须先检查再重载：

```bash
sudo nginx -t
sudo systemctl reload nginx
```

### 6.2 HTTPS 证书

```text
/etc/nginx/ssl/accounter/fullchain.pem
/etc/nginx/ssl/accounter/privkey.key
```

证书域名是 `www.qingzhang.top`，核对时有效期为：

```text
2026-08-08 00:00:00 GMT
至 2026-11-05 23:59:59 GMT
```

服务器上没有发现 certbot、acme.sh、systemd timer 或 root crontab 自动续期任务。因此必须在 `2026-11-05` 前手动更换证书或补充可靠的自动续期方案。

查看证书有效期：

```bash
sudo openssl x509 \
  -in /etc/nginx/ssl/accounter/fullchain.pem \
  -noout -subject -issuer -dates
```

私钥 `/etc/nginx/ssl/accounter/privkey.key` 权限为 `600`。不要输出、复制到 Git 或通过聊天工具传输私钥内容。

## 7. 发布和回滚备份

发布备份目录：

```text
/opt/accounter/backups/<发布号>/
```

发布号格式是 `YYYYMMDDHHMMSS`。目录通常包含：

```text
accounter
accounter-admin
web/
```

注意：以某次发布号命名的目录保存的是该次发布替换下来的上一版本，不是新版本副本。

核对时共有 `18` 个备份目录、约 `842MB`。当前发布脚本没有自动清理旧发布备份，后续应制定保留策略；删除前必须确认目标目录和回滚需求，不能直接清空整个 `/opt/accounter/backups`。

查看备份：

```bash
ssh qingzhang-ecs \
  'sudo du -sh /opt/accounter/backups && sudo ls -lah /opt/accounter/backups'
```

发布过程还会临时使用：

```text
/tmp/accounter-release-<发布号>/
/opt/accounter/web.new-<发布号>/
```

发布成功后临时目录会被脚本删除；健康检查失败时脚本使用对应备份自动回滚。

发布备份**不包含**以下内容：

- `/etc/accounter/env`
- `/etc/nginx/conf.d/accounter.conf`
- `/etc/nginx/ssl/accounter/`
- `/opt/accounter/uploads/`
- RDS 数据和快照

这些内容必须分别制定备份和恢复方案。

## 8. 本地发布相关文件

本地项目根目录：

```text
/Users/zhuoyu.yzy/GolangProject/accounter
```

| 本地文件 | 用途 |
| --- | --- |
| `deploy/release.sh` | 测试、构建、上传、替换、健康检查和自动回滚 |
| `deploy/accounter.service` | systemd 服务模板 |
| `deploy/env.example` | 生产环境变量模板，不包含真实密钥 |
| `deploy/nginx.conf.example` | Nginx 配置模板 |
| `RELEASE_GUIDE.md` | 手动发布流程 |
| `frontend/dist/build/h5/` | 本地 H5 构建产物 |
| `frontend/dist/build/mp-weixin/` | 本地微信小程序构建产物 |
| `dist/accounter` | 本地构建的 Linux 后端二进制 |
| `dist/accounter-admin` | 本地构建的 Linux 运维 CLI |
| `dist/accounter-h5-<发布号>.tar.gz` | 发布时生成的 H5 压缩包 |

正式发布命令：

```bash
cd /Users/zhuoyu.yzy/GolangProject/accounter
PUBLIC_BASE_URL=https://www.qingzhang.top \
  ./deploy/release.sh qingzhang-ecs
```

本地 `dist/` 文件是可重新生成的构建产物，不是生产数据备份。

## 9. 基础部署与运维方法

### 9.1 修改后端环境配置并生效

适用于修改 `/etc/accounter/env` 中的数据库、JWT、微信、AI、日志参数和服务端口等配置。

```bash
# 1. 登录 ECS
ssh qingzhang-ecs

# 2. 编辑配置；sudoedit 会调用本机默认编辑器
sudoedit /etc/accounter/env

# 3. 重启后端读取新配置
sudo systemctl restart accounter

# 4. 确认服务状态
sudo systemctl status accounter --no-pager

# 5. 查看本次启动日志
sudo journalctl -u accounter -n 100 --no-pager

# 6. 检查本机和公网健康状态
curl -fsS http://127.0.0.1:8080/health
curl -fsS https://www.qingzhang.top/health
```

看到服务状态为 `active (running)`，并且健康接口返回 `"status":"ok"`，才表示配置已经生效。

注意事项：

- `/etc/accounter/env` 不是 shell 脚本，使用 `变量名=值`，等号两侧不要加空格。
- 修改 `SERVER_PORT` 时，还必须同步修改 `/etc/nginx/conf.d/accounter.conf` 中的代理端口。
- 修改 `WECHAT_APP_ID` 或 `WECHAT_APP_SECRET` 后必须重新测试小程序微信登录。
- 修改数据库配置前先创建 RDS 快照；配置错误会导致健康检查中的 `database` 变为 `false` 或服务启动失败。
- 修改 `JWT_SECRET` 会使现有用户登录态失效，除非明确需要，否则不要更换。
- 修改 `AI_ACTIVATION_SECRET` 会影响尚未兑换的激活码，投入使用后不要更换。

如果重启失败：

```bash
sudo systemctl status accounter --no-pager
sudo journalctl -u accounter -n 200 --no-pager
```

修正 `/etc/accounter/env` 后再次执行：

```bash
sudo systemctl restart accounter
```

### 9.2 后端服务常用命令

```bash
# 查看状态
sudo systemctl status accounter --no-pager

# 重启并读取最新配置/二进制
sudo systemctl restart accounter

# 停止服务
sudo systemctl stop accounter

# 启动服务
sudo systemctl start accounter

# 实时查看服务日志
sudo journalctl -u accounter -f

# 查看本次开机后的最近 200 行服务日志
sudo journalctl -u accounter -b -n 200 --no-pager
```

线上环境不要使用 `go run`、`nohup` 或手动启动第二个后端进程，统一交给 systemd 管理。

### 9.3 修改 Nginx 配置并生效

适用于修改域名、代理端口、上传大小、超时时间、静态目录或 HTTPS 配置。

```bash
ssh qingzhang-ecs
sudoedit /etc/nginx/conf.d/accounter.conf

# 必须先检查语法
sudo nginx -t

# 只有 nginx -t 成功后才重载
sudo systemctl reload nginx

# 检查状态和错误日志
sudo systemctl status nginx --no-pager
sudo tail -n 100 /var/log/nginx/accounter-error.log
curl -fsS https://www.qingzhang.top/health
```

通常使用 `reload` 即可，不需要 `restart`，这样可以减少连接中断。如果 `nginx -t` 失败，不要重载，先修复配置。

### 9.4 更换 HTTPS 证书并生效

证书文件：

```text
/etc/nginx/ssl/accounter/fullchain.pem
/etc/nginx/ssl/accounter/privkey.key
```

更换证书前先备份到一个明确的新目录，不要覆盖发布备份：

```bash
sudo install -d -m 700 /root/accounter-cert-backup-YYYYMMDD
sudo cp -p /etc/nginx/ssl/accounter/fullchain.pem \
  /root/accounter-cert-backup-YYYYMMDD/
sudo cp -p /etc/nginx/ssl/accounter/privkey.key \
  /root/accounter-cert-backup-YYYYMMDD/
```

上传并替换新证书后，修正权限并检查：

```bash
sudo chown root:root /etc/nginx/ssl/accounter/fullchain.pem \
  /etc/nginx/ssl/accounter/privkey.key
sudo chmod 644 /etc/nginx/ssl/accounter/fullchain.pem
sudo chmod 600 /etc/nginx/ssl/accounter/privkey.key

sudo openssl x509 \
  -in /etc/nginx/ssl/accounter/fullchain.pem \
  -noout -subject -issuer -dates
sudo nginx -t
sudo systemctl reload nginx
```

最后从公网检查实际加载的证书：

```bash
echo | openssl s_client \
  -connect www.qingzhang.top:443 \
  -servername www.qingzhang.top 2>/dev/null \
  | openssl x509 -noout -subject -issuer -dates
```

命令中的 `YYYYMMDD` 必须手动替换为实际日期，例如 `20260903`。不要把私钥内容打印到终端或保存进项目仓库。

### 9.5 发布后端和 H5

日常发布从本地项目根目录执行：

```bash
cd /Users/zhuoyu.yzy/GolangProject/accounter

# 确认本地改动和当前分支
git status
git branch --show-current

# 确认代码已提交并推送
git push origin master

# 测试、构建、上传、替换并健康检查
PUBLIC_BASE_URL=https://www.qingzhang.top \
  ./deploy/release.sh qingzhang-ecs
```

脚本依次执行：

1. 检查 ECS 架构和生产配置；
2. 运行后端测试和 `go vet`；
3. 构建 Linux 后端与 `accounter-admin`；
4. 构建 H5；
5. 上传到 ECS 临时目录；
6. 把旧版保存到 `/opt/accounter/backups/<发布号>/`；
7. 替换后端、CLI 和 H5；
8. 重启 `accounter`；
9. 检查本机及公网 `/health`；
10. 新版健康检查失败时自动回滚。

只有看到下面内容才算部署成功：

```text
ECS 本机健康检查通过
发布完成: <发布号>
```

后端和 H5 发布不会自动上传微信小程序。

### 9.6 构建和上传微信小程序

```bash
cd /Users/zhuoyu.yzy/GolangProject/accounter/frontend

VITE_API_BASE_URL=https://www.qingzhang.top \
  pnpm build:release:mp-weixin
```

将下面的目录导入或刷新到微信开发者工具：

```text
/Users/zhuoyu.yzy/GolangProject/accounter/frontend/dist/build/mp-weixin
```

然后执行：

1. 在微信开发者工具中真机预览；
2. 点击“上传”，填写未使用的新版本号；
3. 登录微信公众平台进入“版本管理”；
4. 将开发版本提交审核；
5. 审核通过后点击“发布”。

前端小程序代码只有在微信公众平台审核并发布后，普通用户才会收到更新。只部署 ECS 不会更新小程序界面。

### 9.7 手动回滚原则

发布脚本在健康检查失败时会自动回滚。若部署成功后才发现业务问题，需要先确定目标备份目录，再手动恢复；不要直接复制“最新目录”，因为备份目录保存的是该次发布替换下来的上一版。

回滚前先查看：

```bash
sudo ls -lah /opt/accounter/backups
sudo find /opt/accounter/backups/<目标发布号> -maxdepth 2 -type f -ls
```

手动回滚涉及停止服务、替换二进制和整个 H5 目录，属于高风险操作。应先确认目标发布号、备份内容以及是否需要同时回滚数据库；生产数据库迁移通常不能仅靠替换程序回滚。优先使用发布脚本的自动回滚，无法确定时不要操作。

### 9.8 运维变更速查

| 改动内容 | 需要执行 |
| --- | --- |
| `/etc/accounter/env` | `sudo systemctl restart accounter` |
| 后端二进制 | 使用 `deploy/release.sh`，脚本会重启服务 |
| H5 前端 | 使用 `deploy/release.sh`，无需单独重启 Nginx |
| Nginx 配置 | `sudo nginx -t && sudo systemctl reload nginx` |
| HTTPS 证书 | 替换文件后执行 `nginx -t` 和 `reload nginx` |
| 微信小程序前端 | 构建、上传、提交审核、审核通过后发布 |
| RDS 连接配置 | 先做快照，再重启后端并检查 `/health` |
| systemd service 文件 | `daemon-reload` 后重启 `accounter` |

## 10. 快速排障顺序

```bash
# 1. 公网是否正常
curl -fsS https://www.qingzhang.top/health

# 2. 服务是否运行
ssh qingzhang-ecs 'sudo systemctl status accounter --no-pager'

# 3. 后端错误
ssh qingzhang-ecs 'sudo journalctl -u accounter -n 200 --no-pager'

# 4. Nginx 错误
ssh qingzhang-ecs 'sudo tail -n 200 /var/log/nginx/accounter-error.log'

# 5. 配置和语法
ssh qingzhang-ecs 'sudo nginx -t'

# 6. 磁盘空间
ssh qingzhang-ecs 'df -h && sudo du -sh /opt/accounter/*'
```

健康接口正常响应应类似：

```json
{"ai":true,"database":true,"status":"ok"}
```
