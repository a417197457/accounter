import { TOKEN_KEY } from './config.js'

/**
 * 检查是否已登录
 */
export function isLoggedIn() {
  const token = uni.getStorageSync(TOKEN_KEY)
  return !!token
}

/**
 * 检查登录状态，未登录则跳转到登录页
 * @returns {boolean} 是否已登录
 */
export function checkLogin() {
  if (!isLoggedIn()) {
    uni.redirectTo({
      url: '/pages/login/index'
    })
    return false
  }
  return true
}

/**
 * 获取当前用户信息
 */
export function getCurrentUser() {
  const userStr = uni.getStorageSync('accounter_user')
  if (userStr) {
    try {
      return JSON.parse(userStr)
    } catch (e) {
      return null
    }
  }
  return null
}
