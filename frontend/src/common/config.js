// 生产 API 地址通过构建环境变量注入：
// VITE_API_BASE_URL=https://你的已备案域名（不要带末尾 /）
const configuredBaseURL = String(import.meta.env.VITE_API_BASE_URL || '').replace(/\/$/, '')

// 环境配置
const ENV = {
  dev: {
    BASE_URL: configuredBaseURL // H5 开发默认留空走 vite 代理，也可显式覆盖
  },
  mpDev: {
    BASE_URL: configuredBaseURL || 'http://127.0.0.1:8080' // 开发者工具模拟器访问本机后端
  },
  h5Prod: {
    BASE_URL: configuredBaseURL // H5 与 API 同域部署时留空，Nginx 代理 /api
  },
  mpProd: {
    BASE_URL: configuredBaseURL || 'https://accounter.yourdomain.com' // 构建正式版时必须注入真实 HTTPS 域名
  }
}

// 当前环境（根据编译模式自动判断）
// #ifdef H5
const currentEnv = import.meta.env.DEV ? 'dev' : 'h5Prod'
// #endif
// #ifdef MP-WEIXIN
// 环境由构建模式决定；这样开发者工具导入正式包时也会真实访问生产 HTTPS。
const currentEnv = import.meta.env.DEV ? 'mpDev' : 'mpProd'
// #endif

export const BASE_URL = ENV[currentEnv].BASE_URL
export const TOKEN_KEY = 'accounter_token'
export const USER_KEY = 'accounter_user'
export const LEDGER_KEY = 'accounter_ledger' // 已基本弃用，保留兼容
