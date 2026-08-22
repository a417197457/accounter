<template>
  <view class="page">
    <WebSidebar />
    <view class="subpage-head">
      <text class="back-link" @click="goBack">‹ 返回</text>
      <view>
        <text class="page-title">我的账本</text>
        <text class="page-copy">集中管理个人、家庭、旅游和礼账账本</text>
      </view>
    </view>

    <view class="limit-card">
      <view>
        <text class="limit-title">我创建的账本</text>
        <text class="limit-copy">{{ ownedSharedCount }} / {{ ledgerLimit }} 个</text>
      </view>
      <button class="create-button" :class="{ disabled: !canCreate }" @click="showCreateModal">
        {{ canCreate ? '+ 创建账本' : '已达上限' }}
      </button>
    </view>

    <view class="ledger-list">
      <view class="ledger-card" v-for="ledger in ledgers" :key="ledger.id" @click="openLedger(ledger)">
        <view class="ledger-icon" :class="ledger.type">{{ ledgerTypeIcon(ledger) }}</view>
        <view class="ledger-main">
          <view class="ledger-title-row">
            <text class="ledger-name">{{ ledger.name }}</text>
            <text class="ledger-badge">{{ ledgerTypeLabel(ledger) }}</text>
            <text class="owner-badge" v-if="ledger.role === 'owner' && !ledger.is_personal">创建者</text>
          </view>
          <text class="ledger-meta" v-if="ledger.is_personal">默认记账账本 · 点击查看明细</text>
          <text class="ledger-meta" v-else>{{ ledger.member_count || 1 }} 位成员 · 点击查看同步明细</text>
          <view class="ledger-actions" v-if="!ledger.is_personal" @click.stop>
            <text v-if="ledger.role === 'owner'" @click="showInviteModal(ledger)">邀请码</text>
            <text @click="showMembers(ledger)">成员</text>
            <text class="danger" v-if="ledger.role === 'member'" @click="leaveLedger(ledger)">退出</text>
          </view>
        </view>
        <text class="chevron">›</text>
      </view>
      <view class="empty" v-if="!loading && ledgers.length === 0">暂无账本</view>
    </view>

    <view class="join-card">
      <text class="join-title">加入共享账本</text>
      <text class="join-copy">输入其他成员提供的 6 位邀请码</text>
      <view class="join-row">
        <input class="input join-input" v-model="joinCode" placeholder="请输入邀请码" maxlength="6" />
        <button class="join-button" @click="joinLedger" :loading="joinLoading">加入</button>
      </view>
    </view>

    <view class="modal" v-if="showCreate" @tap="showCreate = false">
      <view class="modal-content" @tap.stop>
        <text class="modal-title">创建新账本</text>
        <text class="field-label">账本类型</text>
        <view class="ledger-type-options">
          <view
            class="ledger-type-option"
            :class="{ selected: newLedgerType === item.value }"
            v-for="item in ledgerTypes"
            :key="item.value"
            @click="newLedgerType = item.value"
          >
            <text class="type-icon">{{ item.icon }}</text>
            <text>{{ item.label }}</text>
          </view>
        </view>
        <input class="input create-input" v-model="newLedgerName" :placeholder="newLedgerPlaceholder" maxlength="64" />
        <view class="modal-actions">
          <button class="btn secondary" @click="showCreate = false">取消</button>
          <button class="btn primary" @click="createLedger" :loading="createLoading">创建</button>
        </view>
      </view>
    </view>

    <view class="modal" v-if="showInvite" @tap="showInvite = false">
      <view class="modal-content" @tap.stop>
        <text class="modal-title">账本邀请码</text>
        <text class="invite-code">{{ currentLedger?.invite_code }}</text>
        <text class="invite-tip">分享给需要共同查看此账本的人</text>
        <button class="btn primary full" @click="copyInvite">复制邀请码</button>
      </view>
    </view>

    <view class="modal" v-if="showMember" @tap="showMember = false">
      <view class="modal-content" @tap.stop>
        <text class="modal-title">成员列表</text>
        <view class="member-list">
          <view class="member-item" v-for="member in members" :key="member.user_id">
            <view class="member-main">
              <view class="member-avatar">{{ (member.nickname || member.username || '成').slice(0, 1) }}</view>
              <view>
                <text class="member-name">{{ member.nickname || member.username }}</text>
                <text class="member-role">{{ member.role === 'owner' ? '创建者' : '成员' }}</text>
              </view>
            </view>
            <text
              class="remove-member"
              v-if="currentLedger?.role === 'owner' && member.role !== 'owner'"
              @click="removeMember(member)"
            >移除</text>
          </view>
        </view>
        <button class="btn secondary full" @click="showMember = false">关闭</button>
      </view>
    </view>
  </view>
</template>

<script>
import { get, post, del } from '../../common/request.js'
import { checkLogin } from '../../common/auth.js'
import { goBackOrProfile } from '../../common/nav.js'

const LEDGER_LIMIT = 10
const OPEN_LEDGER_KEY = 'open_ledger_id'

export default {
  data() {
    return {
      ledgers: [],
      loading: false,
      ledgerLimit: LEDGER_LIMIT,
      joinCode: '',
      joinLoading: false,
      showCreate: false,
      createLoading: false,
      newLedgerName: '',
      newLedgerType: 'family',
      ledgerTypes: [
        { value: 'family', label: '家庭', icon: '🏠', placeholder: '账本名称，如：我们家' },
        { value: 'travel', label: '旅游', icon: '✈️', placeholder: '账本名称，如：云南之旅' },
        { value: 'gift', label: '礼账', icon: '🧧', placeholder: '账本名称，如：婚礼礼账' }
      ],
      currentLedger: null,
      showInvite: false,
      showMember: false,
      members: []
    }
  },
  computed: {
    ownedSharedCount() {
      return this.ledgers.filter(item => !item.is_personal && item.role === 'owner').length
    },
    canCreate() {
      return this.ownedSharedCount < this.ledgerLimit
    },
    newLedgerPlaceholder() {
      return this.ledgerTypes.find(item => item.value === this.newLedgerType)?.placeholder || '请输入账本名称'
    }
  },
  onLoad() {
    checkLogin()
  },
  onShow() {
    this.loadLedgers()
  },
  methods: {
    goBack: goBackOrProfile,
    async loadLedgers() {
      this.loading = true
      try {
        this.ledgers = await get('/ledgers') || []
      } catch (err) {
        if (err.message?.includes('未登录')) uni.redirectTo({ url: '/pages/login/index' })
      } finally {
        this.loading = false
      }
    },
    ledgerTypeLabel(ledger) {
      return ({ personal: '个人', family: '家庭', travel: '旅游', gift: '礼账' })[ledger?.type] || '家庭'
    },
    ledgerTypeIcon(ledger) {
      return ({ personal: '账', family: '🏠', travel: '✈️', gift: '🧧' })[ledger?.type] || '账'
    },
    openLedger(ledger) {
      uni.setStorageSync(OPEN_LEDGER_KEY, String(ledger.id))
      uni.switchTab({ url: '/pages/list/index' })
    },
    showCreateModal() {
      if (!this.canCreate) {
        uni.showToast({ title: '最多只能创建 10 个账本', icon: 'none' })
        return
      }
      this.newLedgerName = ''
      this.newLedgerType = 'family'
      this.showCreate = true
    },
    async createLedger() {
      const name = this.newLedgerName.trim()
      if (!name) {
        uni.showToast({ title: '请输入账本名称', icon: 'none' })
        return
      }
      this.createLoading = true
      try {
        const result = await post('/ledgers', { name, type: this.newLedgerType })
        this.showCreate = false
        this.currentLedger = { ...(result.ledger || {}), role: 'owner', invite_code: result.invite_code }
        this.showInvite = true
        await this.loadLedgers()
        uni.showToast({ title: '创建成功', icon: 'success' })
      } catch (err) {
        // request 已展示服务端返回的明确错误。
      } finally {
        this.createLoading = false
      }
    },
    showInviteModal(ledger) {
      this.currentLedger = ledger
      this.showInvite = true
    },
    copyInvite() {
      const code = this.currentLedger?.invite_code || ''
      if (!code) return
      uni.setClipboardData({
        data: code,
        success: () => {
          this.showInvite = false
          uni.showToast({ title: '已复制', icon: 'success' })
        }
      })
    },
    async showMembers(ledger) {
      this.currentLedger = ledger
      try {
        this.members = await get(`/ledgers/${ledger.id}/members`) || []
        this.showMember = true
      } catch (err) {
        uni.showToast({ title: err.message || '加载成员失败', icon: 'none' })
      }
    },
    async removeMember(member) {
      uni.showModal({
        title: '移除成员',
        content: `确定移除「${member.nickname || member.username}」吗？`,
        success: async result => {
          if (!result.confirm) return
          try {
            await del(`/ledgers/${this.currentLedger.id}/members/${member.user_id}`)
            await this.showMembers(this.currentLedger)
            await this.loadLedgers()
            uni.showToast({ title: '已移除', icon: 'success' })
          } catch (err) {
            uni.showToast({ title: err.message || '移除失败', icon: 'none' })
          }
        }
      })
    },
    async joinLedger() {
      const code = this.joinCode.trim().toUpperCase()
      if (code.length !== 6) {
        uni.showToast({ title: '请输入 6 位邀请码', icon: 'none' })
        return
      }
      this.joinLoading = true
      try {
        await post('/ledgers/join', { code })
        this.joinCode = ''
        await this.loadLedgers()
        uni.showToast({ title: '加入成功', icon: 'success' })
      } catch (err) {
        // request 已展示错误。
      } finally {
        this.joinLoading = false
      }
    },
    leaveLedger(ledger) {
      uni.showModal({
        title: '确认退出',
        content: `确定退出「${ledger.name}」吗？`,
        success: async result => {
          if (!result.confirm) return
          try {
            await post(`/ledgers/${ledger.id}/leave`)
            await this.loadLedgers()
            uni.showToast({ title: '已退出', icon: 'success' })
          } catch (err) {
            uni.showToast({ title: err.message || '退出失败', icon: 'none' })
          }
        }
      })
    }
  }
}
</script>

<style scoped>
.page { min-height: 100vh; padding: 22px 16px 40px; box-sizing: border-box; background: #f7f9f7; }
.subpage-head { display: flex; align-items: flex-start; gap: 12px; margin-bottom: 18px; }
.back-link { padding: 5px 0; color: #42a987; font-size: 14px; }
.page-title { display: block; color: #18312b; font-size: 26px; font-weight: 750; }
.page-copy { display: block; margin-top: 5px; color: #87968f; font-size: 12px; }
.limit-card { display: flex; align-items: center; justify-content: space-between; padding: 18px; margin-bottom: 14px; border-radius: 18px; background: #eaf7f2; border: 1px solid #d7eee5; }
.limit-title { display: block; color: #31564c; font-size: 14px; font-weight: 700; }
.limit-copy { display: block; margin-top: 4px; color: #6c837a; font-size: 12px; }
.create-button, .join-button { margin: 0; border: 0; color: #fff; background: #42c39c; border-radius: 12px; font-size: 13px; line-height: 1.4; }
.create-button { padding: 10px 14px; }
.create-button.disabled { color: #93a19c; background: #dfe6e3; }
.ledger-list { display: flex; flex-direction: column; gap: 10px; }
.ledger-card { display: flex; align-items: center; gap: 13px; padding: 17px 15px; border: 1px solid #e8eeeb; border-radius: 18px; background: #fff; box-shadow: 0 7px 24px rgba(24,49,43,.035); }
.ledger-icon { width: 44px; height: 44px; flex: 0 0 44px; display: flex; align-items: center; justify-content: center; border-radius: 14px; color: #2c7f67; background: #eaf7f2; font-size: 20px; font-weight: 700; }
.ledger-icon.travel { background: #eaf2fb; }
.ledger-icon.gift { background: #fff1ed; }
.ledger-icon.family { background: #fff7df; }
.ledger-main { flex: 1; min-width: 0; }
.ledger-title-row { display: flex; align-items: center; flex-wrap: wrap; gap: 6px; }
.ledger-name { max-width: 100%; color: #263f38; font-size: 16px; font-weight: 700; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
.ledger-badge, .owner-badge { padding: 2px 7px; border-radius: 999px; color: #2fa680; background: #eaf7f2; font-size: 10px; }
.owner-badge { color: #a16c20; background: #fff4dc; }
.ledger-meta { display: block; margin-top: 6px; color: #8a9993; font-size: 12px; }
.ledger-actions { display: flex; gap: 16px; margin-top: 10px; color: #35a783; font-size: 12px; }
.ledger-actions .danger, .remove-member { color: #e06a62; }
.chevron { color: #a7b2ad; font-size: 25px; }
.empty { padding: 35px; text-align: center; color: #8a9993; background: #fff; border-radius: 18px; }
.join-card { margin-top: 18px; padding: 18px; border: 1px solid #e8eeeb; border-radius: 18px; background: #fff; }
.join-title { display: block; color: #263f38; font-size: 16px; font-weight: 700; }
.join-copy { display: block; margin-top: 4px; color: #8a9993; font-size: 12px; }
.join-row { display: flex; gap: 10px; margin-top: 14px; }
.input { height: 42px; padding: 0 12px; box-sizing: border-box; border: 1px solid #dfe8e4; border-radius: 12px; background: #fbfcfb; font-size: 14px; }
.join-input { flex: 1; min-width: 0; }
.join-button { width: 76px; padding: 0; }
.modal { position: fixed; inset: 0; z-index: 1200; display: flex; align-items: center; justify-content: center; background: rgba(24,49,43,.34); }
.modal-content { width: 82%; max-width: 380px; max-height: 78vh; overflow-y: auto; padding: 23px; box-sizing: border-box; border-radius: 20px; background: #fff; }
.modal-title { display: block; margin-bottom: 17px; text-align: center; color: #263f38; font-size: 18px; font-weight: 700; }
.field-label { display: block; margin-bottom: 9px; color: #667c73; font-size: 13px; }
.ledger-type-options { display: flex; gap: 8px; margin-bottom: 14px; }
.ledger-type-option { flex: 1; padding: 11px 5px; display: flex; flex-direction: column; align-items: center; gap: 4px; border: 1px solid #e2e9e6; border-radius: 12px; color: #71817a; font-size: 12px; }
.ledger-type-option.selected { color: #2fa680; border-color: #42c39c; background: #eaf7f2; }
.type-icon { font-size: 20px; }
.create-input { width: 100%; }
.modal-actions { display: flex; gap: 10px; margin-top: 18px; }
.btn { flex: 1; margin: 0; padding: 10px; border-radius: 12px; font-size: 14px; line-height: 1.4; }
.btn.primary { color: #fff; background: #42c39c; }
.btn.secondary { color: #60766e; background: #f0f4f2; }
.btn.full { width: 100%; margin-top: 16px; }
.invite-code { display: block; padding: 16px; text-align: center; color: #268f70; background: #eaf7f2; border-radius: 14px; font-size: 28px; font-weight: 750; letter-spacing: 4px; }
.invite-tip { display: block; margin-top: 10px; text-align: center; color: #8a9993; font-size: 12px; }
.member-list { max-height: 48vh; overflow-y: auto; }
.member-item { display: flex; justify-content: space-between; align-items: center; padding: 12px 0; border-bottom: 1px solid #edf1ee; }
.member-main { display: flex; align-items: center; gap: 10px; }
.member-avatar { width: 36px; height: 36px; display: flex; align-items: center; justify-content: center; color: #fff; background: #42c39c; border-radius: 12px; }
.member-name, .member-role { display: block; }
.member-name { color: #263f38; font-size: 14px; }
.member-role { margin-top: 3px; color: #95a29d; font-size: 11px; }
.remove-member { font-size: 12px; }
@media (min-width: 900px) { .page { padding: 32px 36px 50px 256px; } .subpage-head, .limit-card, .ledger-list, .join-card { max-width: 820px; margin-left: auto; margin-right: auto; } }
</style>
