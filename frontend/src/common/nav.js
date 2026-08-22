/** 子页返回：有上一页则返回，否则回「我的」 */
export function goBackOrProfile() {
  const pages = typeof getCurrentPages === 'function' ? getCurrentPages() : []
  if (pages && pages.length > 1) {
    uni.navigateBack({ fail: () => uni.switchTab({ url: '/pages/profile/index' }) })
    return
  }
  uni.switchTab({ url: '/pages/profile/index' })
}
