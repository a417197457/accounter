const baseURL = String(process.env.VITE_API_BASE_URL || '').replace(/\/$/, '')

if (!/^https:\/\/[^/]+/.test(baseURL) || /yourdomain|example\.com/i.test(baseURL)) {
  console.error('小程序正式构建失败：请设置真实 HTTPS API 地址，例如：')
  console.error('VITE_API_BASE_URL=https://你的已备案域名 pnpm build:release:mp-weixin')
  process.exit(1)
}

console.log(`正式环境 API：${baseURL}`)
