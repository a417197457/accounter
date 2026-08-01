<template>
  <view class="page">
    <view class="card">
      <text class="title">{{ isLogin ? '登录' : '注册' }}</text>

      <view class="form">
        <view class="field">
          <text class="label">用户名</text>
          <input
            type="text"
            v-model="form.username"
            placeholder="请输入用户名"
            maxlength="32"
            class="login-input"
          />
        </view>

        <view class="field">
          <text class="label">密码</text>
          <input
            type="text"
            password
            v-model="form.password"
            placeholder="请输入密码"
            maxlength="64"
            class="login-input"
          />
        </view>

        <view class="field" v-if="!isLogin">
          <text class="label">昵称（可选）</text>
          <input
            type="text"
            v-model="form.nickname"
            placeholder="请输入昵称"
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
      const inputs = document.querySelectorAll('.login-input input')
      inputs.forEach(input => {
        input.style.height = '24px'
        input.style.lineHeight = '24px'
        input.style.padding = '0'
        input.style.margin = '0'
        input.style.border = 'none'
        input.style.outline = 'none'
        input.style.background = 'transparent'
        input.style.width = '100%'
        input.style.display = 'block'
      })
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
        const { token, user } = result

        uni.setStorageSync(TOKEN_KEY, token)
        uni.setStorageSync(USER_KEY, JSON.stringify(user))

        uni.showToast({ title: this.isLogin ? '登录成功' : '注册成功', icon: 'success' })
        setTimeout(() => {
          uni.switchTab({ url: '/pages/chat/index' })
        }, 500)
      } catch (err) {
        // request.js 已经显示了 toast，这里不重复
      } finally {
        this.loading = false
      }
    }
  }
}
</script>

<style scoped>
.page {
  padding: 20px;
  min-height: 100vh;
  background: #f5f5f5;
  display: flex;
  align-items: center;
  justify-content: center;
}
.card {
  width: 100%;
  max-width: 400px;
  background: #fff;
  border-radius: 12px;
  padding: 32px 24px;
  box-shadow: 0 2px 8px rgba(0, 0, 0, 0.05);
}
.title {
  font-size: 24px;
  font-weight: bold;
  margin-bottom: 24px;
  display: block;
  text-align: center;
}
.form {
  display: flex;
  flex-direction: column;
}
.field {
  margin-bottom: 16px;
}
.label {
  font-size: 14px;
  color: #666;
  margin-bottom: 8px;
  display: block;
}
.login-input {
  display: block;
  width: 100%;
}
:deep(.login-input) {
  padding: 0 !important;
  border: none !important;
}
:deep(.login-input .uni-input-wrapper) {
  width: 100%;
  padding: 12px;
  border: 1px solid #ddd;
  border-radius: 8px;
  background: #fff;
  box-sizing: border-box;
}
:deep(.login-input input) {
  width: 100% !important;
  font-size: 16px !important;
  outline: none !important;
  background: transparent !important;
  -webkit-appearance: none !important;
  appearance: none !important;
  user-select: auto !important;
  -webkit-user-select: auto !important;
}
:deep(.login-input input::placeholder) {
  color: #bbb !important;
  font-size: 16px !important;
}
.submit-btn {
  margin-top: 8px;
  width: 100%;
  height: 48px;
  font-size: 16px;
  text-align: center !important;
  line-height: 48px !important;
}
.switch {
  margin-top: 16px;
  text-align: center;
}
.switch-text {
  font-size: 14px;
  color: #007aff;
  cursor: pointer;
}
</style>
