<template>
  <view class="page">
    <!-- 用户信息卡片 -->
    <view class="card user-card" v-if="user">
      <view class="user-info">
        <text class="nickname">{{ user.nickname || user.username }}</text>
        <text class="username">@{{ user.username }}</text>
      </view>
      <button class="btn btn-secondary" @click="logout">退出登录</button>
    </view>

    <!-- 我的账本 -->
    <view class="section">
      <view class="section-header">
        <text class="section-title">我的账本</text>
        <button class="btn btn-primary small" @click="showCreateModal">+ 创建</button>
      </view>

      <view class="card ledger-item" v-for="ledger in ledgers" :key="ledger.id">
        <view class="ledger-main">
          <view class="ledger-name-row">
            <text class="ledger-name">{{ ledger.name }}</text>
            <text class="ledger-badge" v-if="ledger.is_personal">个人</text>
            <text class="ledger-badge owner" v-if="ledger.role === 'owner' && !ledger.is_personal">创建者</text>
          </view>
          <text class="ledger-meta">{{ ledger.member_count }} 位成员</text>
        </view>
        <view class="ledger-actions">
          <text class="action-link" @click="showInviteModal(ledger)" v-if="ledger.role === 'owner' && !ledger.is_personal">邀请码</text>
          <text class="action-link" @click="showMembers(ledger)" v-if="!ledger.is_personal">成员</text>
          <text class="action-link danger" @click="leaveLedger(ledger)" v-if="ledger.role === 'member'">退出</text>
        </view>
      </view>

      <view class="card empty" v-if="ledgers.length === 0">
        <text>暂无账本</text>
      </view>
    </view>

    <!-- 加入账本 -->
    <view class="section">
      <view class="section-header">
        <text class="section-title">加入账本</text>
      </view>
      <view class="card">
        <view class="flex" style="gap: 8px;">
          <input class="input" v-model="joinCode" placeholder="输入 6 位邀请码" maxlength="6" />
          <button class="btn btn-primary" @click="joinLedger" :loading="joinLoading">加入</button>
        </view>
      </view>
    </view>

    <!-- 创建账本弹层 -->
    <view class="modal" v-if="showCreate" @click.self="showCreate = false">
      <view class="modal-content">
        <text class="modal-title">创建账本</text>
        <input class="input" v-model="newLedgerName" placeholder="账本名称" maxlength="64" />
        <view class="modal-actions">
          <button class="btn btn-secondary" @click="showCreate = false">取消</button>
          <button class="btn btn-primary" @click="createLedger" :loading="createLoading">创建</button>
        </view>
      </view>
    </view>

    <!-- 邀请码弹层 -->
    <view class="modal" v-if="showInvite" @click.self="showInvite = false">
      <view class="modal-content">
        <text class="modal-title">邀请码</text>
        <text class="invite-code">{{ currentLedger?.invite_code }}</text>
        <text class="invite-tip">分享邀请码给家人即可加入账本</text>
        <button class="btn btn-primary" @click="showInvite = false">知道了</button>
      </view>
    </view>

    <!-- 成员弹层 -->
    <view class="modal" v-if="showMember" @click.self="showMember = false">
      <view class="modal-content">
        <text class="modal-title">成员列表</text>
        <view class="member-list">
          <view class="member-item" v-for="m in members" :key="m.user_id">
            <text class="member-name">{{ m.nickname || m.username }}</text>
            <text class="member-role">{{ m.role === 'owner' ? '创建者' : '成员' }}</text>
          </view>
        </view>
        <button class="btn btn-secondary" @click="showMember = false">关闭</button>
      </view>
    </view>
  </view>
</template>

<script>
import { get, post, del } from '../../common/request.js'
import { TOKEN_KEY, USER_KEY } from '../../common/config.js'

export default {
  data() {
    return {
      user: null,
      ledgers: [],
      joinCode: '',
      joinLoading: false,
      showCreate: false,
      newLedgerName: '',
      createLoading: false,
      showInvite: false,
      showMember: false,
      currentLedger: null,
      members: []
    }
  },
  onShow() {
    this.loadUser()
    this.loadLedgers()
  },
  methods: {
    loadUser() {
      const userStr = uni.getStorageSync(USER_KEY)
      if (userStr) {
        try {
          this.user = JSON.parse(userStr)
        } catch (e) {
          this.user = null
        }
      }
    },
    async loadLedgers() {
      try {
        const result = await get('/ledgers')
        this.ledgers = result || []
      } catch (err) {
        if (err.message && err.message.includes('未登录')) {
          uni.redirectTo({ url: '/pages/login/index' })
        }
      }
    },
    logout() {
      uni.removeStorageSync(TOKEN_KEY)
      uni.removeStorageSync(USER_KEY)
      uni.redirectTo({ url: '/pages/login/index' })
    },
    showCreateModal() {
      this.newLedgerName = ''
      this.showCreate = true
    },
    async createLedger() {
      if (!this.newLedgerName.trim()) {
        uni.showToast({ title: '请输入账本名称', icon: 'none' })
        return
      }
      this.createLoading = true
      try {
        const result = await post('/ledgers', { name: this.newLedgerName })
        uni.showToast({ title: '创建成功', icon: 'success' })
        this.showCreate = false
        this.currentLedger = result.ledger
        this.currentLedger.invite_code = result.invite_code
        this.showInvite = true
        this.loadLedgers()
      } catch (err) {
        uni.showToast({ title: err.message || '创建失败', icon: 'none' })
      } finally {
        this.createLoading = false
      }
    },
    showInviteModal(ledger) {
      this.currentLedger = ledger
      this.showInvite = true
    },
    async showMembers(ledger) {
      this.currentLedger = ledger
      try {
        const result = await get(`/ledgers/${ledger.id}/members`)
        this.members = result || []
        this.showMember = true
      } catch (err) {
        uni.showToast({ title: err.message || '加载失败', icon: 'none' })
      }
    },
    async joinLedger() {
      if (!this.joinCode || this.joinCode.length !== 6) {
        uni.showToast({ title: '请输入 6 位邀请码', icon: 'none' })
        return
      }
      this.joinLoading = true
      try {
        await post('/ledgers/join', { code: this.joinCode })
        uni.showToast({ title: '加入成功', icon: 'success' })
        this.joinCode = ''
        this.loadLedgers()
      } catch (err) {
        uni.showToast({ title: err.message || '加入失败', icon: 'none' })
      } finally {
        this.joinLoading = false
      }
    },
    async leaveLedger(ledger) {
      uni.showModal({
        title: '确认退出',
        content: `确定要退出「${ledger.name}」吗？`,
        success: async (res) => {
          if (res.confirm) {
            try {
              await post(`/ledgers/${ledger.id}/leave`)
              uni.showToast({ title: '已退出', icon: 'success' })
              this.loadLedgers()
            } catch (err) {
              uni.showToast({ title: err.message || '退出失败', icon: 'none' })
            }
          }
        }
      })
    }
  }
}
</script>

<style scoped>
.page {
  padding: 16px;
  min-height: 100%;
  background: #f5f5f5;
}
.user-card {
  display: flex;
  align-items: center;
  justify-content: space-between;
}
.user-info {
  display: flex;
  flex-direction: column;
}
.nickname {
  font-size: 18px;
  font-weight: bold;
  margin-bottom: 4px;
}
.username {
  font-size: 13px;
  color: #999;
}
.section {
  margin-top: 16px;
}
.section-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  margin-bottom: 8px;
}
.section-title {
  font-size: 16px;
  font-weight: bold;
}
.ledger-item {
  display: flex;
  justify-content: space-between;
  align-items: center;
}
.ledger-main {
  flex: 1;
  min-width: 0;
}
.ledger-name-row {
  display: flex;
  align-items: center;
  gap: 8px;
  margin-bottom: 4px;
}
.ledger-name {
  font-size: 15px;
  font-weight: 500;
}
.ledger-badge {
  font-size: 11px;
  padding: 2px 6px;
  border-radius: 4px;
  background: #e6f0ff;
  color: #007aff;
}
.ledger-badge.owner {
  background: #fff4e6;
  color: #ff9500;
}
.ledger-meta {
  font-size: 12px;
  color: #999;
}
.ledger-actions {
  display: flex;
  gap: 12px;
  margin-left: 12px;
}
.action-link {
  font-size: 13px;
  color: #007aff;
  cursor: pointer;
}
.action-link.danger {
  color: #ff3b30;
}
.empty {
  text-align: center;
  color: #999;
  padding: 24px;
}
.btn.small {
  padding: 6px 12px;
  font-size: 13px;
}
.flex {
  display: flex;
}
.modal {
  position: fixed;
  top: 0;
  left: 0;
  right: 0;
  bottom: 0;
  background: rgba(0, 0, 0, 0.5);
  display: flex;
  align-items: center;
  justify-content: center;
  z-index: 999;
}
.modal-content {
  background: #fff;
  border-radius: 12px;
  padding: 24px;
  width: 80%;
  max-width: 360px;
}
.modal-title {
  font-size: 18px;
  font-weight: bold;
  margin-bottom: 16px;
  display: block;
  text-align: center;
}
.modal-actions {
  display: flex;
  gap: 12px;
  margin-top: 16px;
}
.modal-actions .btn {
  flex: 1;
}
.invite-code {
  font-size: 32px;
  font-weight: bold;
  color: #007aff;
  letter-spacing: 4px;
  display: block;
  text-align: center;
  margin: 16px 0 8px;
}
.invite-tip {
  font-size: 13px;
  color: #999;
  display: block;
  text-align: center;
  margin-bottom: 16px;
}
.member-list {
  max-height: 300px;
  overflow-y: auto;
}
.member-item {
  display: flex;
  justify-content: space-between;
  padding: 12px 0;
  border-bottom: 1px solid #f0f0f0;
}
.member-name {
  font-size: 15px;
}
.member-role {
  font-size: 13px;
  color: #999;
}
</style>
