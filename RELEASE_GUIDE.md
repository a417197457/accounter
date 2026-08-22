# 账小鸭手动发版指南

生产环境：

- ECS：`qingzhang-ecs`
- 域名：`https://www.qingzhang.top`
- 微信小程序 AppID：`wx6339d0862921ce0e`

## 1. 发布前检查

在项目根目录执行：

```bash
git status
git pull --ff-only origin master

cd backend && go test ./... && go vet ./...
cd ../frontend && pnpm build:h5 && pnpm check:mp-weixin
cd ..
```

确认测试通过、代码已提交并推送：

```bash
git push origin master
```

## 2. 发布 ECS 后端和 H5

在项目根目录执行：

```bash
PUBLIC_BASE_URL=https://www.qingzhang.top ./deploy/release.sh qingzhang-ecs
```

脚本会自动运行测试、构建、备份旧版本、上传文件、重启服务并检查健康状态。看到“发布完成”才算成功。

发布后验证：

```bash
curl https://www.qingzhang.top/health
ssh qingzhang-ecs 'systemctl is-active accounter'
```

正常结果应包含 `"status":"ok"`，服务状态应为 `active`。

## 3. 构建并上传微信小程序

正式构建：

```bash
cd frontend
VITE_API_BASE_URL=https://www.qingzhang.top pnpm build:release:mp-weixin
```

打开微信开发者工具，导入或刷新以下目录：

```text
frontend/dist/build/mp-weixin
```

点击“上传”，填写一个未使用的新版本号和简短更新说明，例如：

```text
版本号：1.0.7
说明：优化账本管理和记账体验
```

也可以使用开发者工具命令行上传：

```bash
/Applications/wechatwebdevtools.app/Contents/MacOS/cli upload \
  --project "$(pwd)/dist/build/mp-weixin" \
  --version 1.0.7 \
  --desc '优化账本管理和记账体验' \
  --lang zh
```

## 4. 提交审核并发布

登录微信公众平台：

1. 进入“版本管理”，找到刚上传的开发版本。
2. 点击“提交审核”，按页面要求填写功能说明。
3. 审核通过后点击“发布”。
4. 如果修改了名称或头像，先在“设置 → 基本设置”中单独修改；代码上传不会自动修改公众平台资料。

## 5. 发布后验收

- 打开 `https://www.qingzhang.top`，确认页面和登录正常。
- 用小程序体验版检查微信登录、AI 记账、手动记账、账本管理和 Excel 导出。
- 正式发布后再用手机打开线上小程序复查一次。

## 6. 出现问题

查看生产日志：

```bash
ssh qingzhang-ecs 'sudo journalctl -u accounter -n 100 --no-pager'
```

发布脚本健康检查失败时会自动回滚，并在输出中显示备份目录。不要删除 `/opt/accounter/uploads`，也不要覆盖 `/etc/accounter/env`。
