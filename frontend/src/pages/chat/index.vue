<template>
  <view class="page">
    <WebSidebar />
    <!-- 对话历史 -->
    <scroll-view scroll-y class="messages" :scroll-top="scrollTop">
      <view
        class="entitlement-banner"
        :class="aiEntitlement?.status || 'loading'"
        v-if="aiNotice"
        @tap="aiNoticeActionable && goAIEntitlement()"
      >
        <view class="entitlement-banner-main">
          <text class="entitlement-banner-title">{{ aiNotice.title }}</text>
          <text class="entitlement-banner-copy">{{ aiNotice.copy }}</text>
        </view>
        <text class="entitlement-banner-action" v-if="aiNoticeActionable">{{ aiNotice.action }}</text>
      </view>
      <view class="welcome" v-if="messages.length === 0">
        <image class="ai-orb" src="/static/brand/zhangxiaoya-duck-app-icon.png" mode="aspectFill" />
        <text class="welcome-kicker">账小鸭 AI 记账助手</text>
        <text class="welcome-title">今天花了什么？</text>
        <text class="welcome-copy">可以说一大段话拆多笔；不确定时我会追问，你接着答就行</text>
        <view class="quick-list">
          <view class="quick-chip" @tap="useQuickPrompt('午餐花了25元')">午餐 25 元</view>
          <view class="quick-chip" @tap="useQuickPrompt('中午外卖35，下午打车22，晚上买菜58，都是支付宝')">一段话记三笔</view>
          <view class="quick-chip" @tap="useQuickPrompt('这个月花了多少')">本月花了多少</view>
        </view>
      </view>
      <view class="msg" v-for="(m, i) in messages" :key="i" :class="m.role">
        <text class="msg-text" v-if="m.role === 'user'">{{ m.text }}</text>
        <view class="msg-ai" v-else>
          <view class="ai-line">
            <image class="ai-avatar" src="/static/brand/zhangxiaoya-duck-app-icon.png" mode="aspectFill" />
            <text class="msg-text">{{ m.text }}</text>
          </view>
          <view class="warn-banner" v-if="m.warnings && m.warnings.length">
            <text v-for="(w, wi) in m.warnings" :key="wi" class="warn-line">{{ w }}</text>
          </view>
          <view class="drafts-wrap" v-if="m.drafts && m.drafts.length">
            <view
              class="draft-card"
              v-for="(d, di) in m.drafts"
              :key="di"
            >
              <view class="draft-row">
                <view>
                  <text class="draft-caption">第 {{ di + 1 }} / {{ m.drafts.length }} 笔</text>
                  <text class="draft-type" :class="d.op_type">{{ opTypeLabel(d.op_type) }}</text>
                </view>
                <text class="draft-amount">¥{{ d.amount }}</text>
              </view>
              <view class="draft-detail">
                <text class="default-account-badge" v-if="!d.account_specified">默认</text>
                <text v-if="d.payment_account_name">{{ d.payment_account_name }} · </text>
                <text>{{ d.account_name }}</text>
                <text v-if="d.to_account_name"> → {{ d.to_account_name }}</text>
                <text v-if="d.category_name"> · {{ d.category_name }}</text>
                <text v-if="d.occurred_at"> · {{ d.occurred_at }}</text>
              </view>
              <view class="draft-detail" v-if="d.remark"><text>备注：{{ d.remark }}</text></view>
              <view class="draft-actions" v-if="draftStatus(m, di) === 'pending'">
                <button class="btn btn-primary mini" @click="confirmOne(i, di)" :loading="m.itemLoading === di">确认</button>
                <button class="btn btn-secondary mini" @click="ignoreOne(i, di)">忽略</button>
              </view>
              <view class="draft-done" v-else-if="draftStatus(m, di) === 'confirmed'">已保存</view>
              <view class="draft-done" v-else>已忽略</view>
            </view>
            <view class="batch-actions" v-if="pendingCount(m) > 1">
              <button class="btn btn-primary" @click="confirmAll(i)" :loading="m.batchLoading">
                全部确认（{{ pendingCount(m) }} 笔）
              </button>
              <button class="btn btn-secondary" @click="ignoreAll(i)">全部忽略</button>
            </view>
          </view>
        </view>
      </view>
      <view class="thinking" v-if="sending"><text class="msg-text">AI 思考中...</text></view>
    </scroll-view>

    <!-- 录音浮层 -->
    <view class="record-overlay" v-if="recording">
      <view class="record-box" :class="{ cancel: recordCancel }">
        <view class="record-icon">{{ recordCancel ? '🗑️' : '🎙️' }}</view>
        <text class="record-tip">{{ recordCancel ? '松开取消' : (recordSecs >= MAX_RECORD_SECS ? '已达上限，正在识别' : '松开结束，上滑取消') }}</text>
        <text class="record-time">{{ Math.min(recordSecs, MAX_RECORD_SECS) }}/{{ MAX_RECORD_SECS }}s</text>
      </view>
    </view>

    <!-- #ifdef MP-WEIXIN -->
    <view class="privacy-modal" v-if="showPrivacy">
      <view class="privacy-card" @tap.stop>
        <text class="privacy-title">语音记账隐私说明</text>
        <text class="privacy-copy">语音记账需要使用麦克风。录音只用于识别本次账单内容，并发送到服务端完成语音转文字。</text>
        <text class="privacy-link" @tap="openMiniPrivacyContract">查看《小程序隐私保护指引》</text>
        <view class="privacy-actions">
          <button class="btn btn-secondary" @tap="rejectMiniPrivacy">暂不使用</button>
          <button
            id="agree-privacy-btn"
            class="btn btn-primary"
            open-type="agreePrivacyAuthorization"
            @agreeprivacyauthorization="agreeMiniPrivacy"
          >同意并继续</button>
        </view>
      </view>
    </view>
    <!-- #endif -->

    <!-- 底部输入卡 -->
    <view class="input-dock" v-if="aiAccessAllowed">
      <text class="input-hint" v-if="messages.length === 0">说一句，AI 会自动识别金额、分类和账户</text>
      <view class="input-bar" :class="{ 'voice-mode': mode === 'voice' }">
        <view class="mode-btn" @tap="toggleMode">
          <image class="mode-icon" :src="mode === 'text' ? '/static/ui/mic.svg' : '/static/ui/keyboard.svg'" mode="aspectFit" />
        </view>

        <view class="text-wrap" v-if="mode === 'text'">
          <input
            type="text"
            v-model="input"
            class="chat-input"
            confirm-type="send"
            placeholder="记一笔，例如：午餐 25 元"
            @confirm="send"
          />
        </view>
        <button
          class="send-btn"
          :class="{ ready: input.trim() && !sending }"
          v-if="mode === 'text'"
          @click="send"
          :loading="sending"
          :disabled="!input.trim() || sending"
        >
          <image v-if="!sending" class="send-icon" src="/static/ui/send.svg" mode="aspectFit" />
        </button>

        <view
          class="voice-btn"
          v-if="mode === 'voice'"
          @touchstart.prevent="startRecord"
          @touchend.prevent="endRecord"
          @touchmove.prevent="moveRecord"
          @touchcancel.prevent="endRecord"
          @mousedown="startRecord"
          @mouseup="endRecord"
          @mouseleave="endRecord"
        >
          <view class="voice-dot" :class="{ recording }"></view>
          <text>{{ recording ? '松开结束' : '按住说话' }}</text>
          <text class="voice-limit">最长 60 秒</text>
        </view>
      </view>
    </view>
    <view class="input-dock locked-dock" v-else @tap="goAIEntitlement">
      <view class="locked-input">
        <view class="locked-icon">AI</view>
        <view class="locked-main">
          <text class="locked-title">{{ aiLockTitle }}</text>
          <text class="locked-copy">{{ aiLockCopy }}</text>
        </view>
        <text class="locked-action">{{ aiLockAction }}</text>
      </view>
    </view>
  </view>
</template>

<script>
import { get, post } from '../../common/request.js'
import { checkLogin } from '../../common/auth.js'

const MAX_RECORD_SECS = 60
const MAX_CHAT_HISTORY = 12

function normalizeDrafts(res) {
  if (Array.isArray(res?.drafts) && res.drafts.length) return res.drafts
  if (res?.draft) return [res.draft]
  return []
}

function buildAIMessage(res) {
  const drafts = normalizeDrafts(res)
  return {
    role: 'ai',
    text: res.reply || '',
    warnings: Array.isArray(res.warnings) ? res.warnings : [],
    drafts,
    statuses: drafts.map(() => 'pending'), // pending | confirmed | ignored
    itemLoading: -1,
    batchLoading: false
  }
}

/** 从当前 messages 抽出不含本轮的近期历史（本轮走 message 字段） */
function buildHistory(messages) {
  const history = []
  for (const m of messages) {
    const content = (m.text || '').trim()
    if (!content) continue
    if (m.role === 'user') {
      history.push({ role: 'user', content })
    } else if (m.role === 'ai' || m.role === 'assistant') {
      history.push({ role: 'assistant', content })
    }
  }
  if (history.length > MAX_CHAT_HISTORY) {
    return history.slice(-MAX_CHAT_HISTORY)
  }
  return history
}

export default {
  data() {
    return {
      MAX_RECORD_SECS,
      input: '',
      messages: [],
      sending: false,
      scrollTop: 0,
      mode: 'text',
      recording: false,
      recordCancel: false,
      recordSecs: 0,
      mediaRecorder: null,
      audioChunks: [],
      recordTimer: null,
      recordStarting: false,
      pressing: false,
      startY: 0,
      transcribing: false,
      showPrivacy: false,
      aiEntitlement: null,
      entitlementLoading: false
    }
  },
  computed: {
    aiAccessAllowed() {
      if (!this.aiEntitlement) return true
      return this.aiEntitlement.access_allowed !== false
    },
    aiRemainingDays() {
      const seconds = Number(this.aiEntitlement?.remaining_seconds || 0)
      return seconds > 0 ? Math.max(1, Math.ceil(seconds / 86400)) : 0
    },
    aiNotice() {
      const entitlement = this.aiEntitlement
      if (!entitlement) return null
      if (entitlement.access_allowed === false && entitlement.status !== 'expired') {
        return {
          title: '今日 AI 使用次数已达上限',
          copy: `今日已使用 ${entitlement.daily_used || 0} / ${entitlement.daily_limit || 0} 次`,
          action: '查看'
        }
      }
      if (entitlement.status === 'expired') {
        return { title: 'AI 试用已结束', copy: '普通记账、明细和报表不受影响', action: '去激活' }
      }
      if (entitlement.status === 'not_started') {
        return {
          title: `赠送 ${entitlement.trial_days || 7} 天 AI 试用`,
          copy: '第一次成功使用 AI 后开始计时',
          action: ''
        }
      }
      if (entitlement.status === 'trial' && this.aiRemainingDays <= 3) {
        return {
          title: `AI 试用还剩 ${this.aiRemainingDays} 天`,
          copy: '激活后可持续使用文字和语音记账',
          action: '去激活'
        }
      }
      return null
    },
    aiNoticeActionable() {
      return Boolean(this.aiNotice?.action)
    },
    aiLockTitle() {
      if (this.aiEntitlement?.status === 'expired') return 'AI 试用已结束'
      return '今日 AI 使用次数已达上限'
    },
    aiLockCopy() {
      if (this.aiEntitlement?.status === 'expired') return '输入激活码后即可继续使用'
      return '普通记账功能仍可正常使用'
    },
    aiLockAction() {
      return this.aiEntitlement?.status === 'expired' ? '去激活' : '查看权益'
    }
  },
  onLoad() {
    checkLogin()
    this.loadAIEntitlement()
    // #ifdef MP-WEIXIN
    this.initMiniRecorder()
    // #endif
  },
  onShow() {
    checkLogin()
    this.loadAIEntitlement()
  },
  onHide() {
    // #ifdef MP-WEIXIN
    if (this.showPrivacy) this.rejectMiniPrivacy()
    // #endif
    if (this.recording || this.recordStarting) {
      this.recordCancel = true
      this.endRecord()
    }
  },
  methods: {
    async loadAIEntitlement() {
      if (this.entitlementLoading) return
      this.entitlementLoading = true
      try {
        this.aiEntitlement = await get('/ai/entitlement', {}, { silent: true })
      } catch (err) {
        // 网络异常时不在前端误拦截，实际请求仍由服务端校验权益。
      } finally {
        this.entitlementLoading = false
      }
    },
    goAIEntitlement() {
      uni.setStorageSync('open_ai_entitlement', '1')
      uni.switchTab({ url: '/pages/profile/index' })
    },
    ensureAIAccess() {
      if (this.aiAccessAllowed) return true
      uni.showToast({ title: this.aiLockTitle, icon: 'none' })
      return false
    },
    handleAIError(err) {
      if (err?.code === 'AI_ACCESS_EXPIRED' || err?.code === 'AI_DAILY_LIMIT_REACHED') {
        this.loadAIEntitlement()
        return err.message || this.aiLockTitle
      }
      return err?.message || 'AI 调用失败'
    },
    toggleMode() {
      if (!this.ensureAIAccess()) return
      this.mode = this.mode === 'text' ? 'voice' : 'text'
      // 在用户点击切换按钮时提前申请权限，避免第一次长按过程中弹授权窗。
      // #ifdef MP-WEIXIN
      if (this.mode === 'voice') this.ensureMiniRecordPermission()
      // #endif
    },
    useQuickPrompt(text) {
      if (this.sending || !this.ensureAIAccess()) return
      this.input = text
      this.send()
    },
    opTypeLabel(op) {
      return { expense: '支出', income: '收入', borrow_in: '借入', borrow_out: '借出', transfer: '转账' }[op] || op
    },
    draftStatus(m, di) {
      return (m.statuses && m.statuses[di]) || 'pending'
    },
    pendingCount(m) {
      if (!m.statuses) return 0
      return m.statuses.filter((s) => s === 'pending').length
    },
    async startRecord(e) {
      if (this.recording || this.recordStarting || this.transcribing || !this.ensureAIAccess()) return
      const touch = e.touches?.[0] || e
      this.startY = touch.clientY || 0
      this.recordCancel = false
      this.recordSecs = 0
      this.audioChunks = []
      this.pressing = true

      // #ifdef H5
      try {
        const stream = await navigator.mediaDevices.getUserMedia({ audio: true })
        const mr = new MediaRecorder(stream)
        this.mediaRecorder = mr
        mr.ondataavailable = (ev) => {
          if (ev.data.size > 0) this.audioChunks.push(ev.data)
        }
        mr.start()
        this.recording = true
        this.recordTimer = setInterval(() => {
          this.recordSecs++
          if (this.recordSecs >= MAX_RECORD_SECS) {
            uni.showToast({ title: '已达 60 秒上限', icon: 'none' })
            this.endRecord({ force: true })
          }
        }, 1000)
      } catch (err) {
        uni.showToast({ title: '无法访问麦克风：' + (err.message || err), icon: 'none' })
      }
      // #endif

      // #ifdef MP-WEIXIN
      const permitted = await this.ensureMiniRecordPermission()
      if (!permitted) {
        this.pressing = false
        return
      }
      // 首次授权期间用户可能已经松手，避免授权完成后意外持续录音。
      if (!this.pressing) {
        uni.showToast({ title: '麦克风已授权，请再次按住说话', icon: 'none' })
        return
      }
      this.recordStarting = true
      this._stopAfterStart = false
      this._recordCancelled = false
      this._recorderManager.start({
        duration: MAX_RECORD_SECS * 1000,
        sampleRate: 16000,
        numberOfChannels: 1,
        encodeBitRate: 48000,
        format: 'mp3'
      })
      // #endif
    },
    moveRecord(e) {
      if (!this.recording) return
      const touch = e.touches?.[0] || e
      const y = touch.clientY || 0
      this.recordCancel = (this.startY - y) > 60
    },
    endRecord(opts = {}) {
      this.pressing = false
      const cancelled = opts.force ? false : this.recordCancel
      const secs = Math.min(this.recordSecs, MAX_RECORD_SECS)

      // #ifdef MP-WEIXIN
      if (this.recordStarting && !this.recording) {
        this._stopAfterStart = true
        this._recordCancelled = cancelled
        return
      }
      if (!this.recording) return
      this._recordCancelled = cancelled
      this._recorderManager.stop()
      // #endif

      // #ifdef H5
      if (!this.recording) return
      this.recording = false
      clearInterval(this.recordTimer)
      this.recordTimer = null

      const mr = this.mediaRecorder
      if (!mr) return
      if (cancelled) {
        try { mr.stream.getTracks().forEach((t) => t.stop()) } catch (e) {}
        this.mediaRecorder = null
        return
      }
      mr.onstop = async () => {
        try { mr.stream.getTracks().forEach((t) => t.stop()) } catch (e) {}
        this.mediaRecorder = null
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
    // #ifdef MP-WEIXIN
    initMiniRecorder() {
      const manager = uni.getRecorderManager()
      this._recorderManager = manager
      manager.onStart(() => {
        this.recordStarting = false
        this.recording = true
        this.recordSecs = 0
        clearInterval(this.recordTimer)
        this.recordTimer = setInterval(() => {
          this.recordSecs = Math.min(this.recordSecs + 1, MAX_RECORD_SECS)
        }, 1000)
        if (this._stopAfterStart || !this.pressing) {
          this._recordCancelled = this._recordCancelled || !this.pressing
          manager.stop()
        }
      })
      manager.onStop(async (result) => {
        this.recordStarting = false
        this.recording = false
        this.pressing = false
        clearInterval(this.recordTimer)
        this.recordTimer = null
        if (this._recordCancelled) {
          this._recordCancelled = false
          return
        }
        if (!result.tempFilePath || Number(result.duration || 0) < 500) {
          uni.showToast({ title: '录音太短', icon: 'none' })
          return
        }
        try {
          const base64 = await this.readMiniFileBase64(result.tempFilePath)
          await this.transcribeBase64AndSend(base64, 'mp3')
        } catch (err) {
          uni.showToast({ title: err.message || '读取录音失败', icon: 'none' })
        }
      })
      manager.onError((err) => {
        this.recordStarting = false
        this.recording = false
        this.pressing = false
        clearInterval(this.recordTimer)
        this.recordTimer = null
        uni.showToast({ title: err.errMsg || '录音失败', icon: 'none' })
      })
    },
    async ensureMiniRecordPermission() {
      const privacyAllowed = await this.ensureMiniPrivacyAuthorization()
      if (!privacyAllowed) return false
      return new Promise((resolve) => {
        uni.getSetting({
          success: ({ authSetting = {} }) => {
            if (authSetting['scope.record'] === true) {
              resolve(true)
              return
            }
            if (authSetting['scope.record'] === false) {
              this.guideMiniRecordPermission()
              resolve(false)
              return
            }
            uni.authorize({
              scope: 'scope.record',
              success: () => resolve(true),
              fail: () => {
                this.guideMiniRecordPermission()
                resolve(false)
              }
            })
          },
          fail: () => {
            uni.showToast({ title: '无法检查麦克风权限', icon: 'none' })
            resolve(false)
          }
        })
      })
    },
    ensureMiniPrivacyAuthorization() {
      return new Promise((resolve) => {
        uni.getPrivacySetting({
          success: ({ needAuthorization }) => {
            if (!needAuthorization) {
              resolve(true)
              return
            }
            if (this._privacyPendingResolve) this._privacyPendingResolve(false)
            this._privacyPendingResolve = resolve
            this.showPrivacy = true
          },
          fail: () => {
            uni.showToast({ title: '无法检查隐私授权状态', icon: 'none' })
            resolve(false)
          }
        })
      })
    },
    agreeMiniPrivacy() {
      this.showPrivacy = false
      const resolve = this._privacyPendingResolve
      this._privacyPendingResolve = null
      if (resolve) resolve(true)
    },
    rejectMiniPrivacy() {
      this.showPrivacy = false
      const resolve = this._privacyPendingResolve
      this._privacyPendingResolve = null
      if (resolve) resolve(false)
    },
    openMiniPrivacyContract() {
      uni.openPrivacyContract({
        fail: () => uni.showToast({ title: '请先在小程序后台完善隐私保护指引', icon: 'none' })
      })
    },
    guideMiniRecordPermission() {
      uni.showModal({
        title: '需要麦克风权限',
        content: '语音记账需要使用麦克风，请在设置中允许麦克风权限。',
        confirmText: '去设置',
        success: ({ confirm }) => {
          if (confirm) uni.openSetting()
        }
      })
    },
    readMiniFileBase64(filePath) {
      return new Promise((resolve, reject) => {
        const fs = uni.getFileSystemManager()
        fs.readFile({
          filePath,
          encoding: 'base64',
          success: ({ data }) => resolve(data),
          fail: reject
        })
      })
    },
    // #endif
    // #ifdef H5
    blobToBase64(blob) {
      return new Promise((resolve) => {
        const reader = new FileReader()
        reader.onloadend = () => {
          const dataURI = reader.result
          resolve(dataURI.split(',')[1])
        }
        reader.readAsDataURL(blob)
      })
    },
    async transcribeAndSend(blob, format) {
      const base64 = await this.blobToBase64(blob)
      await this.transcribeBase64AndSend(base64, format)
    },
    // #endif
    async transcribeBase64AndSend(base64, format) {
      this.transcribing = true
      uni.showLoading({ title: '识别中...' })
      try {
        const result = await post('/ai/transcribe', { audio: base64, format }, { silent: true })
        this.loadAIEntitlement()
        const text = (result?.text || '').trim()
        if (!text) {
          uni.showToast({ title: '未识别到内容', icon: 'none' })
          return
        }
        const history = buildHistory(this.messages)
        this.messages.push({ role: 'user', text })
        this.sending = true
        this.scrollToBottom()
        try {
          const res = await post('/ai/chat', { message: text, history }, { silent: true })
          this.messages.push(buildAIMessage(res))
          this.loadAIEntitlement()
        } catch (err) {
          this.messages.push({ role: 'ai', text: this.handleAIError(err), drafts: [], statuses: [] })
        } finally {
          this.sending = false
          this.scrollToBottom()
        }
      } catch (err) {
        uni.showToast({ title: this.handleAIError(err) || '语音识别失败', icon: 'none' })
      } finally {
        uni.hideLoading()
        this.transcribing = false
      }
    },
    async send() {
      const text = this.input.trim()
      if (!text || this.sending || !this.ensureAIAccess()) return
      this.input = ''
      const history = buildHistory(this.messages)
      this.messages.push({ role: 'user', text })
      this.sending = true
      this.scrollToBottom()
      try {
        const result = await post('/ai/chat', { message: text, history }, { silent: true })
        this.messages.push(buildAIMessage(result))
        this.loadAIEntitlement()
      } catch (err) {
        this.messages.push({ role: 'ai', text: this.handleAIError(err), drafts: [], statuses: [] })
      } finally {
        this.sending = false
        this.scrollToBottom()
      }
    },
    async confirmOne(msgIndex, draftIndex) {
      const m = this.messages[msgIndex]
      if (!m?.drafts?.[draftIndex] || this.draftStatus(m, draftIndex) !== 'pending') return
      m.itemLoading = draftIndex
      try {
        await post('/ai/confirm', { draft: m.drafts[draftIndex] })
        m.statuses[draftIndex] = 'confirmed'
        uni.showToast({ title: '已保存', icon: 'success' })
      } catch (err) {
        uni.showToast({ title: err.message || '保存失败', icon: 'none' })
      } finally {
        m.itemLoading = -1
      }
    },
    ignoreOne(msgIndex, draftIndex) {
      const m = this.messages[msgIndex]
      if (!m?.statuses || this.draftStatus(m, draftIndex) !== 'pending') return
      m.statuses[draftIndex] = 'ignored'
    },
    async confirmAll(msgIndex) {
      const m = this.messages[msgIndex]
      if (!m?.drafts?.length) return
      const pendingIdx = []
      m.statuses.forEach((s, i) => { if (s === 'pending') pendingIdx.push(i) })
      if (!pendingIdx.length) return
      m.batchLoading = true
      try {
        const drafts = pendingIdx.map((i) => m.drafts[i])
        await post('/ai/confirm', { drafts })
        pendingIdx.forEach((i) => { m.statuses[i] = 'confirmed' })
        uni.showToast({ title: `已保存 ${drafts.length} 笔`, icon: 'success' })
      } catch (err) {
        uni.showToast({ title: err.message || '批量保存失败', icon: 'none' })
      } finally {
        m.batchLoading = false
      }
    },
    ignoreAll(msgIndex) {
      const m = this.messages[msgIndex]
      if (!m?.statuses) return
      m.statuses = m.statuses.map((s) => (s === 'pending' ? 'ignored' : s))
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
  background: #f7f9f7;
}
.messages {
  flex: 1;
  min-height: 0;
  padding: 18px 16px 18px;
  box-sizing: border-box;
}
.entitlement-banner {
  margin-bottom: 12px;
  padding: 12px 14px;
  display: flex;
  align-items: center;
  gap: 12px;
  background: #eef8f4;
  border: 1px solid #d6eee5;
  border-radius: 16px;
}
.entitlement-banner.expired {
  background: #fff5ed;
  border-color: #f2decc;
}
.entitlement-banner-main { flex: 1; min-width: 0; }
.entitlement-banner-title { display: block; color: #294c43; font-size: 13px; font-weight: 700; }
.entitlement-banner-copy { display: block; margin-top: 3px; color: #809089; font-size: 11px; }
.entitlement-banner-action { color: #2faf87; font-size: 12px; font-weight: 700; white-space: nowrap; }
.entitlement-banner.expired .entitlement-banner-action { color: #d17b42; }
.welcome {
  min-height: 390px;
  padding: 42px 4px 14px;
  display: flex;
  flex-direction: column;
  align-items: center;
  box-sizing: border-box;
}
.ai-orb {
  width: 70px;
  height: 70px;
  border-radius: 28px;
  display: block;
  background: #f7f9f7;
  box-shadow: 0 16px 34px rgba(67, 197, 158, 0.25);
}
.welcome-kicker {
  margin-top: 20px;
  color: #36b88f;
  font-size: 12px;
  font-weight: 600;
  letter-spacing: 2px;
}
.welcome-title {
  margin-top: 7px;
  color: #18312b;
  font-size: 26px;
  font-weight: 700;
}
.welcome-copy {
  margin-top: 9px;
  color: #87968f;
  font-size: 13px;
  text-align: center;
  padding: 0 12px;
}
.quick-list {
  margin-top: 24px;
  display: flex;
  justify-content: center;
  flex-wrap: wrap;
  gap: 10px;
}
.quick-chip {
  padding: 10px 15px;
  color: #31564c;
  font-size: 13px;
  background: #fff;
  border: 1px solid #cfe9df;
  border-radius: 999px;
  box-shadow: 0 6px 18px rgba(24, 49, 43, 0.035);
}
.msg { margin-bottom: 16px; display: flex; }
.msg.user { justify-content: flex-end; }
.msg-text { font-size: 15px; line-height: 1.55; white-space: pre-wrap; word-break: break-word; }
.default-account-badge {
  margin-right: 6px;
  padding: 2px 6px;
  color: #2cae86;
  background: #e8f7f1;
  border-radius: 999px;
  font-size: 10px;
}
.msg.user .msg-text {
  background: #43c59e;
  color: #fff;
  padding: 11px 15px;
  border-radius: 17px 17px 5px 17px;
  max-width: 75%;
}
.msg-ai { max-width: 88%; }
.warn-banner {
  margin: 0 0 8px 37px;
  padding: 10px 12px;
  background: #fff7ed;
  border: 1px solid #f3d9b8;
  border-radius: 14px;
}
.warn-line {
  display: block;
  color: #c47a1a;
  font-size: 12px;
  line-height: 1.45;
}
.warn-line + .warn-line { margin-top: 4px; }
.ai-line { display: flex; align-items: flex-start; gap: 9px; }
.ai-avatar {
  width: 28px;
  height: 28px;
  flex: 0 0 28px;
  display: block;
  background: #f7f9f7;
  border-radius: 10px;
}
.msg.ai .msg-text {
  display: block;
  background: #fff;
  padding: 11px 14px;
  border: 1px solid #edf1ee;
  border-radius: 5px 17px 17px 17px;
  margin-bottom: 8px;
  box-shadow: 0 6px 20px rgba(24, 49, 43, 0.04);
}
.drafts-wrap {
  margin-left: 37px;
  display: flex;
  flex-direction: column;
  gap: 10px;
}
.draft-card {
  background: #fff;
  border: 1px solid #d9eee6;
  border-radius: 18px;
  padding: 16px;
  box-shadow: 0 10px 28px rgba(24, 49, 43, 0.055);
}
.draft-row { display: flex; justify-content: space-between; align-items: center; margin-bottom: 8px; }
.draft-caption { display: block; margin-bottom: 5px; color: #93a19b; font-size: 11px; }
.draft-type { font-size: 12px; padding: 3px 8px; border-radius: 999px; background: #eef5f2; }
.draft-type.expense { background: #fff0ed; color: #f06f64; }
.draft-type.income { background: #e9f7f2; color: #36b88f; }
.draft-amount { color: #18312b; font-size: 24px; font-weight: 700; }
.draft-detail { font-size: 13px; color: #71817a; margin-bottom: 5px; }
.draft-actions { display: flex; gap: 8px; margin-top: 12px; }
.batch-actions {
  display: flex;
  flex-direction: column;
  gap: 8px;
  margin-top: 4px;
}
.batch-actions .btn { width: 100% !important; text-align: center !important; line-height: 1.5 !important; margin: 0; }
.btn.mini { flex: 1; padding: 8px; font-size: 14px; text-align: center !important; line-height: 1.5 !important; }
.draft-done { margin-top: 8px; font-size: 13px; color: #87968f; text-align: center; }
.thinking { display: flex; }
.thinking .msg-text {
  background: #fff;
  padding: 10px 14px;
  border-radius: 16px;
  color: #8a9993;
  font-style: italic;
}
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
  background: rgba(24,49,43,0.92);
  color: #fff;
  border-radius: 16px;
  padding: 24px 32px;
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 8px;
  min-width: 140px;
}
.record-box.cancel { background: rgba(240,111,100,0.92); }
.record-icon { font-size: 40px; }
.record-tip { font-size: 13px; }
.record-time { font-size: 20px; font-weight: bold; }
.privacy-modal {
  position: fixed;
  inset: 0;
  z-index: 1200;
  padding: 24px;
  display: flex;
  align-items: center;
  justify-content: center;
  background: rgba(24,49,43,.4);
  box-sizing: border-box;
}
.privacy-card {
  width: 100%;
  max-width: 390px;
  padding: 24px;
  background: #fff;
  border-radius: 22px;
  box-sizing: border-box;
}
.privacy-title { display: block; color: #18312b; font-size: 18px; font-weight: 700; }
.privacy-copy { display: block; margin-top: 12px; color: #71817a; font-size: 13px; line-height: 1.7; }
.privacy-link { display: block; margin-top: 12px; color: #2faf87; font-size: 12px; }
.privacy-actions { margin-top: 20px; display: flex; gap: 10px; }
.privacy-actions .btn { flex: 1; margin: 0; }
.input-dock {
  flex-shrink: 0;
  padding: 0 14px calc(12px + env(safe-area-inset-bottom));
  background: linear-gradient(180deg, rgba(247,249,247,0), #f7f9f7 24%);
}
.locked-dock { cursor: pointer; }
.locked-input {
  min-height: 62px;
  padding: 9px 12px;
  display: flex;
  align-items: center;
  gap: 11px;
  background: #fff;
  border: 1px solid #e4ebe7;
  border-radius: 22px;
  box-shadow: 0 12px 34px rgba(24, 49, 43, 0.08);
  box-sizing: border-box;
}
.locked-icon {
  width: 38px;
  height: 38px;
  flex: 0 0 38px;
  display: flex;
  align-items: center;
  justify-content: center;
  color: #fff;
  background: #9eaaa5;
  border-radius: 14px;
  font-size: 11px;
  font-weight: 700;
}
.locked-main { flex: 1; min-width: 0; }
.locked-title { display: block; color: #344c45; font-size: 13px; font-weight: 700; }
.locked-copy { display: block; margin-top: 3px; color: #95a19c; font-size: 11px; }
.locked-action { color: #2faf87; font-size: 12px; font-weight: 700; white-space: nowrap; }
.input-hint {
  display: block;
  margin: 0 0 8px 4px;
  color: #98a69f;
  font-size: 11px;
  line-height: 1.4;
}
.input-bar {
  display: flex;
  align-items: center;
  gap: 6px;
  min-height: 56px;
  padding: 6px 7px;
  background: #fff;
  border: 1px solid rgba(217, 232, 226, 0.9);
  border-radius: 22px;
  box-shadow: 0 12px 34px rgba(24, 49, 43, 0.09);
  box-sizing: border-box;
}
.mode-btn {
  width: 44px;
  height: 44px;
  display: flex;
  align-items: center;
  justify-content: center;
  color: #2faf87;
  background: #edf8f4;
  border-radius: 16px;
  cursor: pointer;
  flex-shrink: 0;
}
.mode-btn:active { background: #dff2eb; }
.mode-icon { width: 23px; height: 23px; }
.text-wrap { flex: 1; min-width: 0; }
.chat-input {
  width: 100%;
  height: 44px;
  padding: 0 8px;
  font-size: 15px;
  line-height: 44px;
  border: none;
  background: transparent;
  box-sizing: border-box;
}
:deep(.chat-input) { padding: 0 !important; border: none !important; background: transparent !important; }
:deep(.chat-input .uni-input-wrapper) {
  width: 100%;
  height: 44px;
  padding: 0 8px;
  display: flex;
  align-items: center;
  border: none;
  background: transparent;
  box-sizing: border-box;
}
:deep(.chat-input input) {
  width: 100% !important;
  height: 24px !important;
  line-height: 24px !important;
  padding: 0 8px !important;
  margin: 0 !important;
  font-size: 15px !important;
  background: transparent !important;
}
.send-btn {
  display: flex;
  align-items: center;
  justify-content: center;
  flex-shrink: 0;
  width: 44px !important;
  height: 44px;
  min-height: 44px;
  padding: 0;
  border: none;
  border-radius: 16px;
  color: #aebbb6;
  background: #eef2f0;
  box-shadow: none;
  margin: 0;
}
.send-btn::after { border: none; }
.send-btn.ready {
  color: #fff;
  background: #42c39c;
  box-shadow: 0 7px 16px rgba(66, 195, 156, 0.24);
}
.send-btn[disabled] { opacity: 1; }
.send-icon { width: 22px; height: 22px; }
.input-bar.voice-mode { gap: 9px; }
.voice-btn {
  flex: 1;
  height: 44px;
  display: flex;
  align-items: center;
  justify-content: center;
  gap: 8px;
  background: #f5f9f7;
  border-radius: 16px;
  font-size: 14px;
  color: #31564c;
  font-weight: 600;
  cursor: pointer;
  user-select: none;
  -webkit-user-select: none;
}
.voice-btn:active { background: #dff2eb; }
.voice-dot {
  width: 7px;
  height: 7px;
  border-radius: 50%;
  background: #42c39c;
}
.voice-dot.recording {
  background: #f06f64;
  box-shadow: 0 0 0 5px rgba(240,111,100,.12);
}
.voice-limit { color: #9aa7a1; font-size: 11px; font-weight: 400; }

@media (min-width: 900px) {
  .welcome { min-height: 360px; padding-top: 34px; }
  .input-dock { padding-left: 40px; padding-right: 40px; padding-bottom: 0; }
  .input-bar { min-height: 62px; padding: 8px 9px; border-radius: 24px; }
  .input-hint { margin-left: 8px; }
}
</style>
