// 环境配置
const ENV = {
  dev: {
    BASE_URL: '' // H5 开发环境留空，走 vite 代理
  },
  prod: {
    BASE_URL: 'https://accounter.yourdomain.com' // 生产环境地址，上线前修改
  }
}

// 当前环境（根据编译模式自动判断）
// #ifdef H5
const currentEnv = 'dev'
// #endif
// #ifdef MP-WEIXIN
const currentEnv = 'prod'
// #endif

export const BASE_URL = ENV[currentEnv].BASE_URL
export const TOKEN_KEY = 'accounter_token'
export const USER_KEY = 'accounter_user'
export const LEDGER_KEY = 'accounter_ledger' // 已基本弃用，保留兼容
