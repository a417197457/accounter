import { readFileSync } from 'node:fs'

const readJSON = (relativeURL) => JSON.parse(readFileSync(new URL(relativeURL, import.meta.url), 'utf8'))
const releaseVersion = readFileSync(new URL('../../VERSION', import.meta.url), 'utf8').trim()
const packageJSON = readJSON('../package.json')
const manifest = readJSON('../src/manifest.json')
const expectedVersionCode = releaseVersion.replaceAll('.', '')

if (!/^\d+\.\d+\.\d+$/.test(releaseVersion)) {
  console.error(`小程序正式构建失败：VERSION 格式不正确：${releaseVersion}`)
  process.exit(1)
}

if (packageJSON.version !== releaseVersion || manifest.versionName !== releaseVersion || String(manifest.versionCode) !== expectedVersionCode) {
  console.error('小程序正式构建失败：版本记录不一致，请同时更新以下文件：')
  console.error(`VERSION=${releaseVersion}`)
  console.error(`frontend/package.json=${packageJSON.version}`)
  console.error(`frontend/src/manifest.json=${manifest.versionName} / ${manifest.versionCode}`)
  process.exit(1)
}

const baseURL = String(process.env.VITE_API_BASE_URL || '').replace(/\/$/, '')

if (!/^https:\/\/[^/]+/.test(baseURL) || /yourdomain|example\.com/i.test(baseURL)) {
  console.error('小程序正式构建失败：请设置真实 HTTPS API 地址，例如：')
  console.error('VITE_API_BASE_URL=https://你的已备案域名 pnpm build:release:mp-weixin')
  process.exit(1)
}

console.log(`正式版本：${releaseVersion}`)
console.log(`正式环境 API：${baseURL}`)
