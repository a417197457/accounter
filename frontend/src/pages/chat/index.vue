<template>
  <view class="page">
    <!-- 对话历史 -->
    <scroll-view scroll-y class="messages" :scroll-top="scrollTop">
      <view class="msg" v-for="(m, i) in messages" :key="i" :class="m.role">
        <text class="msg-text" v-if="m.role === 'user'">{{ m.text }}</text>
        <view class="msg-ai" v-else>
          <text class="msg-text">{{ m.text }}</text>
          <view class="draft-card" v-if="m.draft">
            <view class="draft-row">
              <text class="draft-type" :class="m.draft.op_type">{{ opTypeLabel(m.draft.op_type) }}</text>
              <text class="draft-amount">¥{{ m.draft.amount }}</text>
            </view>
            <view class="draft-detail">
              <text>{{ m.draft.account_name }}</text>
              <text v-if="m.draft.category_name"> · {{ m.draft.category_name }}</text>
            </view>
            <view class="draft-detail" v-if="m.draft.remark"><text>备注：{{ m.draft.remark }}</text></view>
            <view class="draft-actions" v-if="!m.confirmed && !m.ignored">
              <button class="btn btn-primary mini" @click="confirmDraft(i)" :loading="m.loading">确认保存</button>
              <button class="btn btn-secondary mini" @click="ignoreDraft(i)">忽略</button>
            </view>
            <view class="draft-done" v-else-if="m.confirmed">✅ 已保存</view>
            <view class="draft-done" v-else>🗑️ 已忽略</view>
          </view>
        </view>
      </view>
      <view class="thinking" v-if="sending"><text class="msg-text">AI 思考中...</text></view>
    </scroll-view>

    <!-- 录音浮层 -->
    <view class="record-overlay" v-if="recording">
      <view class="record-box" :class="{ cancel: recordCancel }">
        <view class="record-icon">{{ recordCancel ? '🗑️' : '🎙️' }}</view>
        <text class="record-tip">{{ recordCancel ? '松开取消' : '松开结束，上滑取消' }}</text>
        <text class="record-time">{{ recordSecs }}s</text>
      </view>
    </view>

    <!-- 底部输入栏 -->
    <view class="input-bar">
      <!-- 文字/语音切换 -->
      <view class="mode-btn" @tap="toggleMode">{{ mode === 'text' ? '🎤' : '⌨️' }}</view>

      <!-- 文字模式 -->
      <view class="text-wrap" v-if="mode === 'text'">
        <input
          type="text"
          v-model="input"
          placeholder="说句话记账，如：今天打车花了30"
          class="chat-input"
          confirm-type="send"
          @confirm="send"
        />
      </view>
      <button class="btn btn-primary send-btn" v-if="mode === 'text'" @click="send" :loading="sending">
        {{ sending ? '' : '发送' }}
      </button>

      <!-- 语音模式：按住说话 -->
      <view
        class="voice-btn"
        v-if="mode === 'voice'"
        @touchstart.prevent="startRecord"
        @touchend.prevent="endRecord"
        @touchmove.prevent="moveRecord"
        @mousedown="startRecord"
        @mouseup="endRecord"
        @mouseleave="endRecord"
      >
        <text>{{ recording ? '松开 结束' : '按住 说话' }}</text>
      </view>
    </view>
  </view>
</template>

<script>
import { post } from '../../common/request.js'
import { checkLogin } from '../../common/auth.js'

export default {
  data() {
    return {
      input: '',
      messages: [],
      sending: false,
      scrollTop: 0,
      mode: 'text', // text | voice
      recording: false,
      recordCancel: false,
      recordSecs: 0,
      mediaRecorder: null,
      audioChunks: [],
      recordTimer: null,
      startY: 0,
      transcribing: false
    }
  },
  onLoad() {
    checkLogin()
  },
  onShow() {
    checkLogin()
    this.$nextTick(() => setTimeout(() => this.fixInputHeight(), 100))
  },
  methods: {
    fixInputHeight() {
      const inputs = document.querySelectorAll('.chat-input input')
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
      console.log('toggleMode called, before:', this.mode)
      this.mode = this.mode === 'text' ? 'voice' : 'text'
      console.log('toggleMode after:', this.mode)
      if (this.mode === 'text') {
        this.$nextTick(() => setTimeout(() => this.fixInputHeight(), 50))
      }
    },
    opTypeLabel(op) {
      return { expense: '支出', income: '收入', borrow_in: '借入', borrow_out: '借出', transfer: '转账' }[op] || op
    },
    async startRecord(e) {
      if (this.recording || this.transcribing) return
      // 记录起始位置（上滑取消用）
      const touch = e.touches?.[0] || e
      this.startY = touch.clientY || 0
      this.recordCancel = false
      this.recordSecs = 0
      this.audioChunks = []

      // H5: MediaRecorder
      // #ifdef H5
      try {
        const stream = await navigator.mediaDevices.getUserMedia({ audio: true })
        const mr = new MediaRecorder(stream)
        this.mediaRecorder = mr
        mr.ondataavailable = (e) => {
          if (e.data.size > 0) this.audioChunks.push(e.data)
        }
        mr.start()
        this.recording = true
        this.recordTimer = setInterval(() => { this.recordSecs++ }, 1000)
      } catch (err) {
        uni.showToast({ title: '无法访问麦克风：' + (err.message || err), icon: 'none' })
      }
      // #endif
    },
    moveRecord(e) {
      if (!this.recording) return
      const touch = e.touches?.[0] || e
      const y = touch.clientY || 0
      // 上滑超过 60px 标记取消
      this.recordCancel = (this.startY - y) > 60
    },
    endRecord() {
      if (!this.recording) return
      const cancelled = this.recordCancel
      const secs = this.recordSecs
      this.recording = false
      clearInterval(this.recordTimer)
      this.recordTimer = null

      // #ifdef H5
      const mr = this.mediaRecorder
      if (!mr) return
      if (cancelled) {
        // 取消：停止但不识别
        try { mr.stream.getTracks().forEach(t => t.stop()) } catch (e) {}
        this.mediaRecorder = null
        return
      }
      mr.onstop = async () => {
        if (secs < 1) {
          uni.showToast({ title: '录音太短', icon: 'none' })
          return
        }
        const blob = new Blob(this.audioChunks, { type: 'audio/webm' })
        await this.transcribeAndSend(blob, 'webm')
      }
      mr.stop()
      // #endif
    },
    blobToBase64(blob) {
      return new Promise((resolve) => {
        const reader = new FileReader()
        reader.onloadend = () => {
          const dataURI = reader.result // data:audio/webm;base64,xxx
          const base64 = dataURI.split(',')[1]
          resolve(base64)
        }
        reader.readAsDataURL(blob)
      })
    },
    async transcribeAndSend(blob, format) {
      this.transcribing = true
      uni.showLoading({ title: '识别中...' })
      try {
        const base64 = await this.blobToBase64(blob)
        const result = await post('/ai/transcribe', { audio: base64, format })
        const text = (result?.text || '').trim()
        if (!text) {
          uni.showToast({ title: '未识别到内容', icon: 'none' })
          return
        }
        // 识别成功，作为用户消息自动发送
        this.messages.push({ role: 'user', text })
        this.sending = true
        this.scrollToBottom()
        try {
          const res = await post('/ai/chat', { message: text })
          this.messages.push({
            role: 'ai',
            text: res.reply || '',
            draft: res.draft || null,
            confirmed: false,
            ignored: false,
            loading: false
          })
        } catch (err) {
          this.messages.push({ role: 'ai', text: err.message || 'AI 调用失败' })
        } finally {
          this.sending = false
        }
      } catch (err) {
        uni.showToast({ title: err.message || '语音识别失败', icon: 'none' })
      } finally {
        uni.hideLoading()
        this.transcribing = false
      }
    },
    async send() {
      const text = this.input.trim()
      if (!text || this.sending) return
      this.input = ''
      this.messages.push({ role: 'user', text })
      this.sending = true
      this.scrollToBottom()
      try {
        const result = await post('/ai/chat', { message: text })
        this.messages.push({
          role: 'ai',
          text: result.reply || '',
          draft: result.draft || null,
          confirmed: false,
          ignored: false,
          loading: false
        })
      } catch (err) {
        this.messages.push({ role: 'ai', text: err.message || 'AI 调用失败' })
      } finally {
        this.sending = false
        this.scrollToBottom()
        this.$nextTick(() => setTimeout(() => this.fixInputHeight(), 50))
      }
    },
    async confirmDraft(index) {
      const m = this.messages[index]
      if (!m.draft) return
      m.loading = true
      try {
        await post('/ai/confirm', { draft: m.draft })
        m.confirmed = true
        uni.showToast({ title: '记账成功', icon: 'success' })
      } catch (err) {
        uni.showToast({ title: err.message || '保存失败', icon: 'none' })
      } finally {
        m.loading = false
      }
    },
    ignoreDraft(index) {
      this.messages[index].ignored = true
    },
    scrollToBottom() {
      this.$nextTick(() => { this.scrollTop = 99999 })
    }
  }
}
</script>

<style scoped>
.page {
  display: flex;
  flex-direction: column;
  height: 100%;
  background: #f5f5f5;
}
.messages {
  flex: 1;
  min-height: 0;
  padding: 16px;
}
.msg { margin-bottom: 16px; display: flex; }
.msg.user { justify-content: flex-end; }
.msg-text { font-size: 15px; line-height: 1.5; }
.msg.user .msg-text {
  background: #007aff;
  color: #fff;
  padding: 10px 14px;
  border-radius: 14px 14px 4px 14px;
  max-width: 75%;
}
.msg-ai { max-width: 85%; }
.msg.ai .msg-text {
  display: block;
  background: #fff;
  padding: 10px 14px;
  border-radius: 14px 14px 14px 4px;
  margin-bottom: 8px;
}
.draft-card {
  background: #fff;
  border: 1px solid #007aff;
  border-radius: 12px;
  padding: 14px;
}
.draft-row { display: flex; justify-content: space-between; align-items: center; margin-bottom: 8px; }
.draft-type { font-size: 13px; padding: 2px 8px; border-radius: 4px; background: #f0f0f0; }
.draft-type.expense { background: #ffe5e5; color: #ff3b30; }
.draft-type.income { background: #e5f7e5; color: #34c759; }
.draft-amount { font-size: 20px; font-weight: bold; }
.draft-detail { font-size: 13px; color: #666; margin-bottom: 4px; }
.draft-actions { display: flex; gap: 8px; margin-top: 12px; }
.btn.mini { flex: 1; padding: 8px; font-size: 14px; text-align: center !important; line-height: 1.5 !important; }
.draft-done { margin-top: 8px; font-size: 13px; color: #999; text-align: center; }
.thinking { display: flex; }
.thinking .msg-text {
  background: #fff;
  padding: 10px 14px;
  border-radius: 14px 14px 14px 4px;
  color: #999;
  font-style: italic;
}
/* 录音浮层：pointer-events:none 避免遮挡触发 */
.record-overlay {
  position: fixed;
  top: 0; left: 0; right: 0; bottom: 0;
  display: flex;
  align-items: center;
  justify-content: center;
  pointer-events: none;
  z-index: 999;
}
.record-box {
  background: rgba(0,0,0,0.8);
  color: #fff;
  border-radius: 16px;
  padding: 24px 32px;
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 8px;
  min-width: 140px;
}
.record-box.cancel { background: rgba(255,59,48,0.85); }
.record-icon { font-size: 40px; }
.record-tip { font-size: 13px; }
.record-time { font-size: 20px; font-weight: bold; }
/* 输入栏 */
.input-bar {
  display: flex;
  align-items: center;
  gap: 8px;
  padding: 12px 16px;
  background: #fff;
  border-top: 1px solid #eee;
}
.mode-btn {
  font-size: 26px;
  width: 32px;
  text-align: center;
  cursor: pointer;
  flex-shrink: 0;
}
.text-wrap { flex: 1; min-width: 0; }
.chat-input { flex: 1; min-width: 0; }
:deep(.chat-input) { padding: 0 !important; border: none !important; }
:deep(.chat-input .uni-input-wrapper) {
  width: 100%;
  padding: 10px 12px;
  border: 1px solid #ddd;
  border-radius: 20px;
  background: #f8f8f8;
  box-sizing: border-box;
}
:deep(.chat-input input) {
  width: 100% !important; font-size: 15px !important; outline: none !important;
  background: transparent !important; user-select: auto !important; -webkit-user-select: auto !important;
}
.send-btn { flex-shrink: 0; padding: 8px 16px; font-size: 14px; text-align: center !important; line-height: 1.5 !important; }
.voice-btn {
  flex: 1;
  text-align: center;
  padding: 12px;
  background: #f0f0f0;
  border-radius: 20px;
  font-size: 15px;
  color: #333;
  cursor: pointer;
  user-select: none;
  -webkit-user-select: none;
}
.voice-btn:active { background: #e0e0e0; }
</style>
