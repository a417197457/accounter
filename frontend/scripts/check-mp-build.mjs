import fs from 'node:fs'
import path from 'node:path'

const root = path.resolve('dist/build/mp-weixin')
const pagesRoot = path.join(root, 'pages')
const problems = []

function walk(dir, suffix) {
  if (!fs.existsSync(dir)) return []
  return fs.readdirSync(dir, { withFileTypes: true }).flatMap((entry) => {
    const full = path.join(dir, entry.name)
    if (entry.isDirectory()) return walk(full, suffix)
    return full.endsWith(suffix) ? [full] : []
  })
}

if (!fs.existsSync(pagesRoot)) {
  console.error('未找到微信小程序构建产物，请先运行 pnpm build:mp-weixin')
  process.exit(1)
}

const appConfig = JSON.parse(fs.readFileSync(path.join(root, 'app.json'), 'utf8'))
if (!appConfig.permission?.['scope.record']?.desc) {
  problems.push('app.json: 缺少 scope.record 麦克风用途声明')
}

const chatWXML = path.join(pagesRoot, 'chat/index.wxml')
if (!fs.existsSync(chatWXML) || !/open-type="agreePrivacyAuthorization"/.test(fs.readFileSync(chatWXML, 'utf8'))) {
  problems.push('AI 记账页缺少微信隐私授权按钮')
}

for (const file of walk(pagesRoot, '.wxml')) {
  const source = fs.readFileSync(file, 'utf8')
  for (const [tag] of source.matchAll(/(<input\b[^>]*>)/g)) {
    if (!tag.includes('bindinput')) problems.push(`${file}: input 缺少 bindinput`)
  }
  for (const [tag] of source.matchAll(/(<view\b[^>]*class="[^"]*modal-content[^"]*"[^>]*>)/g)) {
    if (!tag.includes('catchtap')) problems.push(`${file}: modal-content 缺少 catchtap，子元素点击会关闭弹层`)
  }
}

const forbiddenBrowserAPIs = [
  /\bdocument\./,
  /\bwindow\./,
  /\bnavigator\./,
  /\bfetch\s*\(/,
  /\bnew\s+Blob\b/,
  /\bnew\s+FileReader\b/,
  /\bnew\s+MediaRecorder\b/,
  /URL\.createObjectURL/
]

for (const file of walk(pagesRoot, '.js')) {
  const source = fs.readFileSync(file, 'utf8')
  for (const pattern of forbiddenBrowserAPIs) {
    if (pattern.test(source)) problems.push(`${file}: 小程序产物残留浏览器 API ${pattern}`)
  }
  if (/getPhoneNumber|wechat-phone/.test(source)) {
    problems.push(`${file}: 个人主体构建不应调用手机号授权组件`)
  }
  const normalized = file.split(path.sep).join('/')
  if (!normalized.endsWith('/pages/login/index.js') && /\.login\(\{provider:"weixin"/.test(source)) {
    problems.push(`${file}: wx.login 只能由登录页的用户点击触发`)
  }
}

for (const page of ['chat', 'record', 'list', 'report', 'profile']) {
  const file = path.join(pagesRoot, `${page}/index.js`)
  const source = fs.readFileSync(file, 'utf8')
  if (/\.checkLogin\(/.test(source) || source.includes('/pages/login/index')) {
    problems.push(`${file}: 主导航页不得在浏览阶段强制登录或跳转登录页`)
  }
}

if (problems.length) {
  console.error(`微信小程序构建检查失败（${problems.length} 项）：`)
  problems.forEach((problem) => console.error(`- ${problem}`))
  process.exit(1)
}

console.log('微信小程序构建检查通过：输入、弹层、平台 API 与登录能力均符合约束。')
