<template>
  <view class="page">
    <view class="login-shell">
      <image
        class="brand-mark"
        src="/static/brand/zhangxiaoya-duck-app-icon.png"
        mode="aspectFill"
      />
      <text class="brand-name">账小鸭</text>
      <text class="brand-copy">每一笔，都清清楚楚</text>

      <view class="card login-card">
      <!-- #ifdef MP-WEIXIN -->
      <view class="heading-row wechat-heading">
        <text class="title">微信快捷登录</text>
        <text class="heading-tip">使用微信身份即可开始记账</text>
      </view>
      <view class="wechat-login">
        <view class="phone-badge">微</view>
        <text class="wechat-title">使用微信登录</text>
        <text class="wechat-copy">首次登录会自动创建个人账本，登录后可选择微信头像和昵称。</text>
        <button
          class="btn btn-primary submit-btn wechat-btn"
          :loading="loading"
          :disabled="loading"
          @click="handleWechatLogin"
        >微信登录</button>
        <text class="privacy-copy">微信登录仅用于识别账号；使用语音前会单独征得授权</text>
      </view>
      <!-- #endif -->

      <!-- #ifdef H5 -->
      <view class="heading-row">
        <text class="title">{{ isLogin ? '欢迎回来' : '创建账号' }}</text>
        <text class="heading-tip">{{ isLogin ? '登录后继续轻松记账' : '开始你的清爽记账生活' }}</text>
      </view>

      <view class="form">
        <view class="field">
          <view class="label-row">
            <text class="label">用户名</text>
            <text v-if="!isLogin" class="label-hint">3–32 位，仅字母、数字或下划线</text>
          </view>
          <input
            type="text"
            v-model="form.username"
            maxlength="32"
            class="login-input"
          />
        </view>

        <view class="field">
          <view class="label-row">
            <text class="label">密码</text>
            <text v-if="!isLogin" class="label-hint">6–64 位</text>
          </view>
          <input
            type="text"
            password
            v-model="form.password"
            maxlength="64"
            class="login-input"
          />
        </view>

        <view class="field" v-if="!isLogin">
          <view class="label-row">
            <text class="label">昵称</text>
            <text class="label-hint">可选，最多 32 位</text>
          </view>
          <input
            type="text"
            v-model="form.nickname"
            maxlength="32"
            class="login-input"
          />
        </view>

        <button class="btn btn-primary submit-btn" @click="handleSubmit" :loading="loading">
          {{ isLogin ? '登录' : '注册' }}
        </button>

        <view class="switch">
          <text class="switch-text" @click="toggleMode">
            {{ isLogin ? '没有账号？去注册' : '已有账号？去登录' }}
          </text>
        </view>
      </view>
      <!-- #endif -->
      </view>
    </view>
  </view>
</template>

<script>
import { post } from '../../common/request.js'
import { TOKEN_KEY, USER_KEY } from '../../common/config.js'

export default {
  data() {
    return {
      isLogin: true,
      loading: false,
      form: {
        username: '',
        password: '',
        nickname: ''
      }
    }
  },
  onLoad() {
    // 清除可能存在的过期 token
    uni.removeStorageSync(TOKEN_KEY)
    uni.removeStorageSync(USER_KEY)
  },
  mounted() {
    // uni-app input 组件在 H5 下：样式只作用到外层 uni-input，内部真实 input 高度为 0。
    // 需要手动给内部 input 设置高度，否则无法点击/输入。
    this.$nextTick(() => {
      setTimeout(() => this.fixInputHeight(), 100)
    })
  },
  methods: {
    fixInputHeight() {
      // #ifdef H5
      const inputs = document.querySelectorAll('.login-input input')
      inputs.forEach(input => {
        input.style.height = '24px'
        input.style.lineHeight = '24px'
        input.style.padding = '0 8px'
        input.style.margin = '0'
        input.style.border = 'none'
        input.style.outline = 'none'
        input.style.background = 'transparent'
        input.style.width = '100%'
        input.style.display = 'block'
      })
      // #endif
    },
    toggleMode() {
      this.isLogin = !this.isLogin
      this.form = { username: '', password: '', nickname: '' }
      this.$nextTick(() => setTimeout(() => this.fixInputHeight(), 100))
    },
    async handleSubmit() {
      if (!this.form.username || !this.form.password) {
        uni.showToast({ title: '请填写用户名和密码', icon: 'none' })
        return
      }
      if (!this.isLogin && !this.form.username.match(/^[a-zA-Z0-9_]{3,32}$/)) {
        uni.showToast({ title: '用户名只能包含字母、数字、下划线，3-32位', icon: 'none' })
        return
      }
      if (this.form.password.length < 6) {
        uni.showToast({ title: '密码至少 6 位', icon: 'none' })
        return
      }

      this.loading = true
      try {
        const url = this.isLogin ? '/auth/login' : '/auth/register'
        const data = this.isLogin
          ? { username: this.form.username, password: this.form.password }
          : { username: this.form.username, password: this.form.password, nickname: this.form.nickname }

        const result = await post(url, data, { auth: true })
        this.completeLogin(result, this.isLogin ? '登录成功' : '注册成功')
      } catch (err) {
        // request.js 已经显示了 toast，这里不重复
      } finally {
        this.loading = false
      }
    },
    async handleWechatLogin() {
      if (this.loading) return
      this.loading = true
      try {
        const loginResult = await new Promise((resolve, reject) => {
          uni.login({ provider: 'weixin', success: resolve, fail: reject })
        })
        if (!loginResult.code) {
          uni.showToast({ title: '未获取到微信登录凭证', icon: 'none' })
          return
        }
        const result = await post('/auth/wechat', { code: loginResult.code }, { auth: true })
        this.completeLogin(result, '登录成功')
      } catch (err) {
        if (err?.errMsg) {
          uni.showToast({ title: '微信登录失败，请重试', icon: 'none' })
        }
      } finally {
        this.loading = false
      }
    },
    completeLogin(result, message) {
      const { token, user } = result
      uni.setStorageSync(TOKEN_KEY, token)
      uni.setStorageSync(USER_KEY, JSON.stringify(user))
      uni.showToast({ title: message, icon: 'success' })
      const needsWechatProfile = String(user?.username || '').startsWith('wx_') && (!user?.avatar_url || user?.nickname === '微信用户')
      if (needsWechatProfile) uni.setStorageSync('open_wechat_profile', '1')
      setTimeout(() => uni.switchTab({ url: needsWechatProfile ? '/pages/profile/index' : '/pages/chat/index' }), 500)
    }
  }
}
</script>

<style scoped>
.page {
  padding: 28px 20px;
  min-height: 100vh;
  background: #f7f9f7;
  display: flex;
  align-items: center;
  justify-content: center;
  box-sizing: border-box;
}
/* H5 宽屏登录页没有业务侧栏，抵消全局桌面内容偏移。 */
@media (min-width: 900px) {
  .page {
    width: calc(100% + 220px) !important;
    max-width: none !important;
    margin-left: -220px !important;
  }
}
.login-shell {
  width: 100%;
  max-width: 420px;
  display: flex;
  flex-direction: column;
  align-items: center;
}
.brand-mark {
  width: 64px;
  height: 64px;
  border-radius: 24px;
  display: block;
  background: #f7f9f7;
  box-shadow: 0 14px 30px rgba(67, 197, 158, 0.25);
}
.brand-name {
  margin-top: 16px;
  color: #18312b;
  font-size: 28px;
  font-weight: 700;
  letter-spacing: 2px;
}
.brand-copy {
  margin-top: 7px;
  margin-bottom: 26px;
  color: #83928c;
  font-size: 13px;
}
.login-card {
  width: 100%;
  padding: 28px 24px 24px;
  box-sizing: border-box;
}
.heading-row {
  margin-bottom: 24px;
}
.title {
  font-size: 23px;
  font-weight: 700;
  display: block;
  color: #18312b;
}
.heading-tip {
  display: block;
  margin-top: 7px;
  font-size: 13px;
  color: #8a9993;
}
.form {
  display: flex;
  flex-direction: column;
}
.field {
  margin-bottom: 18px;
}
.label-row {
  min-height: 20px;
  margin-bottom: 8px;
  display: flex;
  align-items: baseline;
  justify-content: space-between;
  gap: 12px;
}
.label {
  font-size: 14px;
  color: #4e655e;
  flex-shrink: 0;
  font-weight: 500;
}
.label-hint {
  color: #98a59f;
  font-size: 11px;
  line-height: 17px;
  text-align: right;
}
.login-input {
  display: block;
  width: 100%;
  height: 50px;
  box-sizing: border-box;
}
:deep(.login-input) {
  padding: 0 !important;
  border: none !important;
}
:deep(.login-input .uni-input-wrapper) {
  width: 100%;
  height: 50px;
  padding: 0 14px;
  display: flex;
  align-items: center;
  border: 1px solid #e2e9e6;
  border-radius: 13px;
  background: #fbfcfb;
  box-sizing: border-box;
}
:deep(.login-input input) {
  width: 100% !important;
  height: 24px !important;
  line-height: 24px !important;
  font-size: 16px !important;
  padding: 0 8px !important;
  outline: none !important;
  background: transparent !important;
  -webkit-appearance: none !important;
  appearance: none !important;
  user-select: auto !important;
  -webkit-user-select: auto !important;
}
.submit-btn {
  margin-top: 6px;
  width: 100% !important;
  min-height: 50px;
  height: 50px;
  font-size: 16px;
  border-radius: 15px;
  display: flex;
  align-items: center;
  justify-content: center;
  line-height: 1.4 !important;
}
.switch {
  margin-top: 18px;
  text-align: center;
}
.switch-text {
  font-size: 14px;
  color: #36b88f;
  cursor: pointer;
}
.wechat-heading {
  text-align: center;
}
.wechat-login {
  display: flex;
  flex-direction: column;
  align-items: center;
}
.phone-badge {
  width: 56px;
  height: 56px;
  border-radius: 20px;
  display: flex;
  align-items: center;
  justify-content: center;
  background: #edf9f4;
  color: #35b88d;
  font-size: 24px;
  font-weight: 700;
}
.wechat-title {
  margin-top: 16px;
  color: #29443c;
  font-size: 16px;
  font-weight: 600;
}
.wechat-copy {
  margin-top: 8px;
  color: #8a9993;
  font-size: 12px;
  line-height: 20px;
  text-align: center;
}
.wechat-btn {
  margin-top: 24px;
}
.privacy-copy {
  margin-top: 14px;
  color: #a3afa9;
  font-size: 11px;
}
</style>
