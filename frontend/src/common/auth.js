import { TOKEN_KEY } from './config.js'

export const LOGIN_RETURN_KEY = 'accounter_login_return'

const TAB_ROUTES = new Set([
  'pages/chat/index',
  'pages/record/index',
  'pages/list/index',
  'pages/report/index',
  'pages/profile/index'
])

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
 * 在用户主动使用需要账号的功能时再询问登录，不阻断游客浏览。
 * @returns {boolean} 当前是否已经登录
 */
export function requireLogin(message = '登录后即可使用该功能') {
  if (isLoggedIn()) return true
  const pages = getCurrentPages()
  const route = pages.length ? pages[pages.length - 1].route : 'pages/chat/index'
  uni.showModal({
    title: '登录后继续',
    content: message,
    confirmText: '去登录',
    cancelText: '继续浏览',
    success: ({ confirm }) => {
      if (!confirm) return
      uni.setStorageSync(LOGIN_RETURN_KEY, TAB_ROUTES.has(route) ? route : 'pages/profile/index')
      uni.navigateTo({ url: '/pages/login/index' })
    }
  })
  return false
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
