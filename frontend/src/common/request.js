import { BASE_URL, TOKEN_KEY, USER_KEY } from './config.js'

/**
 * 封装 uni.request
 * @param {string} url - 接口路径（不含 /api 前缀）
 * @param {string} method - 请求方法
 * @param {object} data - 请求数据
 * @param {object} options - 额外选项
 * @param {string} options.ledgerId - 账本 ID（会注入 X-Ledger-Id 头）
 * @param {boolean} options.silent - 是否静默失败（不显示 toast）
 */
export function request(url, method = 'GET', data = {}, options = {}) {
  return new Promise((resolve, reject) => {
    const token = uni.getStorageSync(TOKEN_KEY)
    const header = {
      'Content-Type': 'application/json'
    }
    if (token) {
      header['Authorization'] = `Bearer ${token}`
    }
    if (options.ledgerId) {
      header['X-Ledger-Id'] = options.ledgerId
    }

    uni.request({
      url: BASE_URL + '/api' + url,
      method,
      data,
      header,
      success: (res) => {
        if (res.statusCode === 401) {
          // 检查是否是登录/注册接口
          const isAuthUrl = url.startsWith('/auth/login') || url.startsWith('/auth/register') || url.startsWith('/auth/wechat')

          if (isAuthUrl) {
            // 登录/注册接口的 401，显示后端返回的错误信息
            const errMsg = res.data?.error || '登录失败，请重试'
            if (!options.silent) {
              uni.showToast({ title: errMsg, icon: 'none' })
            }
            const error = new Error(errMsg)
            error.code = res.data?.code || ''
            error.statusCode = res.statusCode
            reject(error)
            return
          }

          // 其他接口的 401，token 过期，跳登录
          uni.removeStorageSync(TOKEN_KEY)
          uni.removeStorageSync(USER_KEY)
          const currentPage = getCurrentPages().pop()
          if (currentPage && currentPage.route !== 'pages/login/index') {
            uni.redirectTo({ url: '/pages/login/index' })
            if (!options.silent) {
              uni.showToast({ title: '未登录或登录已过期', icon: 'none' })
            }
          }
          const error = new Error('未登录或登录已过期')
          error.code = res.data?.code || 'UNAUTHORIZED'
          error.statusCode = res.statusCode
          reject(error)
          return
        }
        if (res.statusCode >= 200 && res.statusCode < 300) {
          // 自动剥 data
          resolve(res.data?.data !== undefined ? res.data.data : res.data)
        } else {
          const errMsg = res.data?.error || `请求失败: ${res.statusCode}`
          if (!options.silent) {
            uni.showToast({ title: errMsg, icon: 'none' })
          }
          const error = new Error(errMsg)
          error.code = res.data?.code || ''
          error.statusCode = res.statusCode
          reject(error)
        }
      },
      fail: (err) => {
        if (!options.silent) {
          uni.showToast({ title: '网络错误', icon: 'none' })
        }
        reject(err)
      }
    })
  })
}

// 快捷方法
export const get = (url, data, options) => request(url, 'GET', data, options)
export const post = (url, data, options) => request(url, 'POST', data, options)
export const put = (url, data, options) => request(url, 'PUT', data, options)
export const del = (url, data, options) => request(url, 'DELETE', data, options)
