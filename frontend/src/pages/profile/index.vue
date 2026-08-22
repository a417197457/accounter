<template>
  <view class="page">
    <WebSidebar />
    <view class="card user-card" v-if="user">
      <view class="avatar">{{ (user.nickname || user.username || '我').slice(0, 1) }}</view>
      <view class="user-info">
        <text class="nickname">{{ user.nickname || user.username }}</text>
        <text class="username">@{{ user.username }}</text>
      </view>
      <text class="logout-link" @click="logout">退出</text>
    </view>

    <view class="section">
      <view class="section-header">
        <text class="section-title">我的账本</text>
        <text class="section-action" @click="showCreateModal">+ 创建</text>
      </view>

      <view class="card ledger-item" v-for="ledger in ledgers" :key="ledger.id">
        <view class="ledger-main">
          <view class="ledger-name-row">
            <text class="ledger-name">{{ ledger.name }}</text>
            <text class="ledger-badge" v-if="ledger.is_personal">个人</text>
            <text class="ledger-badge owner" v-if="ledger.role === 'owner' && !ledger.is_personal">创建者</text>
          </view>
          <text class="ledger-meta">{{ ledger.member_count }} 位成员</text>
          <text class="ledger-tip" v-if="ledger.is_personal">记账与 AI 默认写入此账本</text>
          <text class="ledger-tip" v-else>通过明细页将个人账单同步到这里</text>
        </view>
        <view class="ledger-actions">
          <text
            class="action-link"
            @click="showInviteModal(ledger)"
            v-if="ledger.role === 'owner' && !ledger.is_personal"
          >邀请码</text>
          <text class="action-link" @click="showMembers(ledger)" v-if="!ledger.is_personal">成员</text>
          <text class="action-link danger" @click="leaveLedger(ledger)" v-if="ledger.role === 'member'">退出</text>
        </view>
      </view>

      <view class="card empty" v-if="ledgers.length === 0">
        <text>暂无账本</text>
      </view>
    </view>

    <view class="section">
      <view class="section-header">
        <text class="section-title">加入家庭账本</text>
      </view>
      <view class="card">
        <view class="join-row">
          <input class="input join-input" v-model="joinCode" placeholder="输入 6 位邀请码" maxlength="6" />
          <button class="btn btn-primary join-btn" @click="joinLedger" :loading="joinLoading">加入</button>
        </view>
      </view>
    </view>

    <view class="section">
      <view class="section-header"><text class="section-title">管理工具</text></view>
      <view class="manage-grid">
        <view class="manage-card" @click="goAccountManagement">
          <view class="manage-icon mint">账</view>
          <view class="manage-main">
            <text class="manage-title">账户管理</text>
            <text class="manage-copy">余额、信用卡与支付渠道</text>
          </view>
          <text class="manage-chevron">›</text>
        </view>
        <view class="manage-card" @click="goCategoryManagement">
          <view class="manage-icon amber">类</view>
          <view class="manage-main">
            <text class="manage-title">分类管理</text>
            <text class="manage-copy">一级、二级分类与图标</text>
          </view>
          <text class="manage-chevron">›</text>
        </view>
        <view class="manage-card" @click="openAIEntitlement">
          <view class="manage-icon violet">权</view>
          <view class="manage-main">
            <text class="manage-title">AI 功能权益</text>
            <text class="manage-copy">{{ aiEntitlementCopy }}</text>
          </view>
          <text class="manage-chevron">›</text>
        </view>
        <view class="manage-card" @click="openBookkeepingSettings">
          <view class="manage-icon blue">AI</view>
          <view class="manage-main">
            <text class="manage-title">AI 记账方式</text>
            <text class="manage-copy" v-if="bookkeepingMode === 'strict'">精确模式 · 未说明账户时会追问</text>
            <text class="manage-copy" v-else>便捷模式 · 默认 {{ defaultAccountLabel || '账户' }}</text>
          </view>
          <text class="manage-chevron">›</text>
        </view>
      </view>
    </view>

    <!-- 个人账本账户 -->
    <view class="section" v-if="false">
      <view class="section-header">
        <text class="section-title">我的账户</text>
        <text class="section-action" @click="showAddAccount">+ 新增</text>
      </view>
      <view class="card account-item" v-for="acc in accounts" :key="acc.id">
        <view class="account-main">
          <text class="account-name">{{ acc.name }}</text>
          <text class="account-meta">{{ accountTypeLabel(acc.type) }} · {{ accountMoneyText(acc) }}</text>
          <text class="account-links" v-if="acc.linked_accounts?.length">关联：{{ acc.linked_accounts.map(item => item.name).join('、') }}</text>
          <text class="account-archived" v-if="acc.is_archived">已归档</text>
        </view>
        <view class="ledger-actions">
          <text class="action-link" @click="showEditAccount(acc)">编辑</text>
          <text class="action-link" v-if="!acc.is_archived" @click="archiveAccount(acc)">归档</text>
          <text class="action-link" v-else @click="unarchiveAccount(acc)">恢复</text>
          <text class="action-link danger" @click="deleteAccount(acc)">删除</text>
        </view>
      </view>
      <view class="card empty" v-if="accounts.length === 0">
        <text>暂无账户</text>
      </view>
    </view>

    <view class="section" v-if="false">
      <view class="section-header">
        <text class="section-title">记账分类</text>
        <text class="section-action" @click="openCategoryModal(null)">+ 一级分类</text>
      </view>
      <view class="category-kind-tabs">
        <text :class="{ active: categoryKind === 'expense' }" @click="switchCategoryKind('expense')">支出</text>
        <text :class="{ active: categoryKind === 'income' }" @click="switchCategoryKind('income')">收入</text>
      </view>
      <view class="card category-group" v-for="group in visibleCategories" :key="group.id">
        <view class="category-parent-row" @click="toggleCategoryGroup(group.id)">
          <view class="category-name-wrap">
            <text class="category-chevron" :class="{ open: isCategoryExpanded(group.id) }">›</text>
            <text class="category-icon">{{ group.icon || '📦' }}</text>
            <text class="category-name">{{ group.name }}</text>
            <text class="category-count" v-if="(group.children || []).length">{{ group.children.length }}</text>
          </view>
          <view class="category-actions" @click.stop>
            <text @click="openCategoryModal(group)">+ 子类</text>
            <text @click="editCategory(group)">编辑</text>
            <text class="danger" @click="deleteCategory(group)">删除</text>
          </view>
        </view>
        <view v-if="isCategoryExpanded(group.id)">
          <view class="category-child-row" v-for="child in group.children || []" :key="child.id">
            <text>{{ child.icon || '·' }} {{ child.name }}</text>
            <view class="category-actions">
              <text @click="editCategory(child)">编辑</text>
              <text class="danger" @click="deleteCategory(child)">删除</text>
            </view>
          </view>
          <view class="category-child-empty" v-if="!(group.children || []).length">暂无二级分类，点「+ 子类」添加</view>
        </view>
      </view>
      <view class="card empty" v-if="visibleCategories.length === 0">暂无分类</view>
    </view>

    <view class="modal" v-if="showCreate" @tap="showCreate = false">
      <view class="modal-content" @tap.stop>
        <text class="modal-title">创建家庭账本</text>
        <input class="input create-input" v-model="newLedgerName" placeholder="账本名称，如：我家" maxlength="64" />
        <view class="modal-actions">
          <button class="btn btn-secondary" @click="showCreate = false">取消</button>
          <button class="btn btn-primary" @click="createLedger" :loading="createLoading">创建</button>
        </view>
      </view>
    </view>

    <view class="modal" v-if="showInvite" @tap="showInvite = false">
      <view class="modal-content" @tap.stop>
        <text class="modal-title">邀请码</text>
        <text class="invite-code">{{ currentLedger?.invite_code }}</text>
        <text class="invite-tip">分享邀请码给家人即可加入账本</text>
        <button class="btn btn-primary" @click="copyInvite">复制邀请码</button>
      </view>
    </view>

    <view class="modal" v-if="showMember" @tap="showMember = false">
      <view class="modal-content" @tap.stop>
        <text class="modal-title">成员列表</text>
        <view class="member-list">
          <view class="member-item" v-for="m in members" :key="m.user_id">
            <view>
              <text class="member-name">{{ m.nickname || m.username }}</text>
              <text class="member-role">{{ m.role === 'owner' ? '创建者' : '成员' }}</text>
            </view>
            <text
              class="action-link danger"
              v-if="currentLedger?.role === 'owner' && m.role !== 'owner'"
              @click="removeMember(m)"
            >移除</text>
          </view>
        </view>
        <button class="btn btn-secondary" @click="showMember = false">关闭</button>
      </view>
    </view>

    <view class="modal" v-if="showBookkeepingSettings" @tap="showBookkeepingSettings = false">
      <view class="modal-content preference-modal" @tap.stop>
        <text class="modal-title">AI 记账方式</text>
        <view class="preference-toggle-row">
          <view class="preference-toggle-copy">
            <text class="preference-toggle-title">未说明时使用默认账户</text>
            <text class="preference-toggle-tip">开启为便捷模式，关闭为精确模式</text>
          </view>
          <switch
            color="#42c39c"
            :checked="bookkeepingMode === 'default'"
            @change="onBookkeepingModeChange"
          />
        </view>

        <view class="preference-note" :class="bookkeepingMode">
          <text class="preference-note-title">{{ bookkeepingMode === 'default' ? '便捷模式' : '精确模式' }}</text>
          <text v-if="bookkeepingMode === 'default'">没说支付方式时，自动使用下面选定的账户；明确说明时仍以本次说明为准。</text>
          <text v-else>必须说明余额或具体银行卡，否则 AI 会先追问，不生成可确认的记账草稿。</text>
        </view>

        <view v-if="bookkeepingMode === 'default'" class="default-account-section">
          <text class="default-account-title">默认支付方式</text>
          <text class="default-account-current">当前：{{ defaultAccountLabel || '请选择' }}</text>
          <view class="default-account-group" v-for="group in defaultAccountGroups" :key="group.key">
            <view class="default-account-group-head">
              <text>{{ group.title }}</text>
              <text>{{ group.copy }}</text>
            </view>
            <view
              class="default-account-option"
              :class="{ selected: isDefaultAccountOption(option) }"
              v-for="option in group.options"
              :key="`${option.paymentAccount?.id || 0}-${option.account.id}`"
              @tap="selectDefaultAccount(option)"
            >
              <view class="default-check">{{ isDefaultAccountOption(option) ? '✓' : '' }}</view>
              <view class="default-account-main">
                <text>{{ option.label }}</text>
                <text v-if="option.paymentAccount">实际计入 {{ option.account.name }}</text>
              </view>
            </view>
          </view>
        </view>

        <view class="modal-actions">
          <button class="btn btn-secondary" @click="showBookkeepingSettings = false">取消</button>
          <button class="btn btn-primary" @click="saveBookkeepingPreference" :loading="bookkeepingLoading">保存</button>
        </view>
      </view>
    </view>

    <view class="modal" v-if="showAIEntitlement" @tap="showAIEntitlement = false">
      <view class="modal-content entitlement-modal" @tap.stop>
        <text class="modal-title">AI 功能权益</text>
        <view class="entitlement-state" :class="aiEntitlement?.status || 'loading'">
          <view class="entitlement-state-icon">{{ aiEntitlement?.status === 'activated' ? '✓' : 'AI' }}</view>
          <view class="entitlement-state-main">
            <text class="entitlement-state-title">{{ aiEntitlementTitle }}</text>
            <text class="entitlement-state-copy">{{ aiEntitlementDetail }}</text>
          </view>
        </view>
        <view class="entitlement-usage" v-if="aiEntitlement">
          <text>今日使用</text>
          <text>{{ aiEntitlement.daily_used || 0 }} / {{ aiEntitlement.daily_limit || 0 }} 次</text>
        </view>
        <view class="activation-section entitlement-loading" v-if="!aiEntitlement">
          <text class="activation-tip">正在读取权益状态，请稍候...</text>
        </view>
        <view class="activation-section" v-else-if="aiEntitlement.status !== 'activated'">
          <text class="activation-title">永久激活</text>
          <text class="activation-tip">输入一次性激活码，成功后当前账号可持续使用 AI 功能。</text>
          <input
            class="input create-input activation-input"
            v-model="activationCode"
            placeholder="QZ-XXXX-XXXX-XXXX-XXXX"
            maxlength="29"
          />
          <button class="btn btn-primary activation-btn" @click="activateAI" :loading="activationLoading">立即激活</button>
        </view>
        <view class="entitlement-permanent" v-else>
          <text>此权益绑定当前账号，在其他设备登录后同样有效。</text>
        </view>
        <button class="btn btn-secondary entitlement-close" @click="showAIEntitlement = false">关闭</button>
      </view>
    </view>

    <view class="modal" v-if="showAccount" @tap="showAccount = false">
      <view class="modal-content" @tap.stop>
        <text class="modal-title">{{ editingAccount ? '编辑账户' : '新增账户' }}</text>
        <input class="input create-input" v-model="newAccountName" placeholder="账户名称，如：招商储蓄卡" maxlength="64" />
        <view class="type-row">
          <text
            class="type-chip"
            :class="{ active: newAccountType === t.value }"
            v-for="t in accountTypes"
            :key="t.value"
            @click="!editingAccount && (newAccountType = t.value)"
          >{{ t.label }}</text>
        </view>
        <view class="link-section" v-if="newAccountType === 'alipay' || newAccountType === 'wechat'">
          <text class="link-title">关联储蓄卡 / 信用卡</text>
          <text class="link-tip">记账时可以选择余额，或选择这里关联的卡</text>
          <view
            class="link-option"
            :class="{ selected: linkedAccountIDs.includes(candidate.id) }"
            v-for="candidate in linkCandidates"
            :key="candidate.id"
            @click="toggleLinkedAccount(candidate.id)"
          >
            <text>{{ linkedAccountIDs.includes(candidate.id) ? '✓' : '○' }}</text>
            <text>{{ candidate.name }} · {{ accountTypeLabel(candidate.type) }}</text>
          </view>
          <text class="link-empty" v-if="linkCandidates.length === 0">请先新增储蓄卡或信用卡</text>
        </view>
        <view class="modal-actions">
          <button class="btn btn-secondary" @click="showAccount = false">取消</button>
          <button class="btn btn-primary" @click="createAccount" :loading="accountLoading">{{ editingAccount ? '保存' : '创建' }}</button>
        </view>
      </view>
    </view>

    <view class="modal" v-if="showCategoryEditor" @tap="showCategoryEditor = false">
      <view class="modal-content" @tap.stop>
        <text class="modal-title">{{ editingCategory ? '编辑分类' : (categoryParent ? '新增二级分类' : '新增一级分类') }}</text>
        <text class="category-parent-tip" v-if="categoryParent">所属一级分类：{{ categoryParent.icon }} {{ categoryParent.name }}</text>
        <input class="input create-input category-input" v-model="categoryForm.name" placeholder="分类名称" maxlength="64" />
        <text class="icon-picker-title">选择图标</text>
        <view class="icon-picker">
          <text
            v-for="icon in categoryIcons"
            :key="icon"
            :class="{ active: categoryForm.icon === icon }"
            @click="categoryForm.icon = icon"
          >{{ icon }}</text>
        </view>
        <view class="modal-actions">
          <button class="btn btn-secondary" @click="showCategoryEditor = false">取消</button>
          <button class="btn btn-primary" @click="saveCategory" :loading="categoryLoading">保存</button>
        </view>
      </view>
    </view>
  </view>
</template>

<script>
import { get, post, put, del } from '../../common/request.js'
import { TOKEN_KEY, USER_KEY } from '../../common/config.js'
import { accountMoneyText } from '../../common/money.js'

export default {
  data() {
    return {
      user: null,
      ledgers: [],
      accounts: [],
      categories: { expense: [], income: [] },
      categoryKind: 'expense',
      showCategoryEditor: false,
      categoryLoading: false,
      editingCategory: null,
      categoryParent: null,
      categoryForm: { name: '', icon: '🍽️' },
      expandedCategoryIds: {},
      categoryIcons: ['🍽️', '🥐', '🍱', '🍜', '🥤', '🚗', '🚇', '🚕', '🛒', '🧴', '👕', '💻', '🛍️', '🏠', '🔑', '🎮', '🎬', '✈️', '💊', '📚', '📱', '🎁', '💰', '📈', '💼', '🧧', '📦', '💡', '🔧', '🩺'],
      joinCode: '',
      joinLoading: false,
      showCreate: false,
      newLedgerName: '',
      createLoading: false,
      showInvite: false,
      showMember: false,
      showAccount: false,
      accountLoading: false,
      newAccountName: '',
      newAccountType: 'savings',
      linkedAccountIDs: [],
      editingAccount: null,
      accountTypes: [
        { value: 'alipay', label: '支付宝' },
        { value: 'wechat', label: '微信' },
        { value: 'savings', label: '储蓄卡' },
        { value: 'cash', label: '现金' },
        { value: 'credit', label: '信用卡' }
      ],
      currentLedger: null,
      members: [],
      showBookkeepingSettings: false,
      bookkeepingLoading: false,
      bookkeepingMode: 'default',
      defaultAccountID: null,
      defaultPaymentAccountID: null,
      aiEntitlement: null,
      showAIEntitlement: false,
      activationCode: '',
      activationLoading: false,
    }
  },
  computed: {
    linkCandidates() {
      return this.accounts.filter(account => !account.is_archived && (account.type === 'savings' || account.type === 'credit'))
    },
    visibleCategories() {
      return this.categories[this.categoryKind] || []
    },
    defaultAccountGroups() {
      const active = this.accounts.filter(account => !account.is_archived)
      const groups = active
        .filter(account => account.type === 'alipay' || account.type === 'wechat')
        .map(payment => ({
          key: `payment-${payment.id}`,
          title: payment.name,
          copy: '余额或关联银行卡',
          options: [
            { account: payment, paymentAccount: null, label: `${payment.name}余额` },
            ...(payment.linked_accounts || []).filter(item => !item.is_archived).map(account => ({
              account,
              paymentAccount: payment,
              label: account.name
            }))
          ]
        }))
      const direct = active.filter(account => account.type !== 'alipay' && account.type !== 'wechat')
      if (direct.length) {
        groups.push({
          key: 'direct',
          title: '直接使用账户',
          copy: '不经过支付渠道',
          options: direct.map(account => ({ account, paymentAccount: null, label: account.name }))
        })
      }
      return groups
    },
    defaultAccountLabel() {
      const account = this.accounts.find(item => item.id === this.defaultAccountID)
      if (!account) return ''
      const payment = this.accounts.find(item => item.id === this.defaultPaymentAccountID)
      if (payment) return `${payment.name} · ${account.name}`
      if (account.type === 'alipay' || account.type === 'wechat') return `${account.name}余额`
      return account.name
    },
    aiRemainingDays() {
      const seconds = Number(this.aiEntitlement?.remaining_seconds || 0)
      return seconds > 0 ? Math.max(1, Math.ceil(seconds / 86400)) : 0
    },
    aiEntitlementCopy() {
      const status = this.aiEntitlement?.status
      if (status === 'activated') return '已永久激活'
      if (status === 'trial') return `试用中 · 剩余 ${this.aiRemainingDays} 天`
      if (status === 'expired') return '试用已结束 · 输入激活码继续使用'
      if (status === 'not_started') return `${this.aiEntitlement?.trial_days || 7} 天试用 · 首次使用时开始`
      return '正在读取权益状态'
    },
    aiEntitlementTitle() {
      const status = this.aiEntitlement?.status
      if (status === 'activated') return 'AI 功能已永久激活'
      if (status === 'trial') return `AI 试用中，剩余 ${this.aiRemainingDays} 天`
      if (status === 'expired') return 'AI 试用已结束'
      if (status === 'not_started') return `${this.aiEntitlement?.trial_days || 7} 天试用尚未开始`
      return '正在读取'
    },
    aiEntitlementDetail() {
      const status = this.aiEntitlement?.status
      if (status === 'activated') return '文字记账与语音识别均可继续使用'
      if (status === 'trial') return `有效期至 ${this.formatEntitlementTime(this.aiEntitlement?.trial_expires_at)}`
      if (status === 'expired') return '普通记账、明细与报表不受影响'
      if (status === 'not_started') return '第一次成功调用 AI 后开始计算 7×24 小时'
      return '请稍候'
    }
  },
  onShow() {
    this.loadUser()
    this.loadLedgers()
    this.loadAccounts()
    this.loadBookkeepingPreference()
    this.loadAIEntitlement().then(() => {
      if (uni.getStorageSync('open_ai_entitlement') === '1') {
        uni.removeStorageSync('open_ai_entitlement')
        this.openAIEntitlement()
      }
    })
    this.$nextTick(() => setTimeout(() => this.fixInputs(), 100))
  },
  methods: {
    goAccountManagement() {
      uni.navigateTo({ url: '/pages/account/index' })
    },
    goCategoryManagement() {
      uni.navigateTo({ url: '/pages/category/index' })
    },
    formatMoney(v) {
      const n = parseFloat(v)
      return Number.isFinite(n) ? n.toFixed(2) : '0.00'
    },
    accountMoneyText,
    accountTypeLabel(type) {
      return ({ alipay: '支付宝', wechat: '微信', cash: '现金', savings: '储蓄卡', credit: '信用卡', bank: '储蓄卡' })[type] || '其他'
    },
    fixInputs() {
      // #ifdef H5
      const inputs = document.querySelectorAll('.join-input input, .create-input input, .category-input input, .activation-input input')
      inputs.forEach(input => {
        input.style.height = '24px'
        input.style.lineHeight = '24px'
        input.style.width = '100%'
        input.style.padding = '0 8px'
        input.style.display = 'block'
        input.style.background = 'transparent'
        input.style.border = 'none'
        input.style.outline = 'none'
      })
      // #endif
    },
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
    async loadAccounts() {
      try {
        // 不传 ledgerId → 个人账本账户
        const result = await get('/accounts')
        this.accounts = result || []
		this.ensureDefaultAccount()
      } catch (err) {
        console.error(err)
      }
    },
    async loadBookkeepingPreference() {
      try {
        const result = await get('/ai/account-preference')
        this.bookkeepingMode = result?.account_mode === 'strict' ? 'strict' : 'default'
        this.defaultAccountID = result?.default_account_id || null
        this.defaultPaymentAccountID = result?.default_payment_account_id || null
      } catch (err) {
        console.error(err)
      }
    },
    async loadAIEntitlement() {
      try {
        this.aiEntitlement = await get('/ai/entitlement', {}, { silent: true })
      } catch (err) {
        console.error(err)
      }
    },
    async openAIEntitlement() {
      await this.loadAIEntitlement()
      this.activationCode = ''
      this.showAIEntitlement = true
      this.$nextTick(() => setTimeout(() => this.fixInputs(), 100))
    },
    formatEntitlementTime(value) {
      if (!value) return '-'
      const date = new Date(value)
      if (Number.isNaN(date.getTime())) return '-'
      const pad = (number) => String(number).padStart(2, '0')
      return `${date.getFullYear()}-${pad(date.getMonth() + 1)}-${pad(date.getDate())} ${pad(date.getHours())}:${pad(date.getMinutes())}`
    },
    async activateAI() {
      const code = this.activationCode.trim().toUpperCase()
      if (!code) {
        uni.showToast({ title: '请填写激活码', icon: 'none' })
        return
      }
      this.activationLoading = true
      try {
        this.aiEntitlement = await post('/ai/activate', { code })
        this.activationCode = ''
        uni.showToast({ title: 'AI 功能已永久激活', icon: 'success' })
      } catch (err) {
        // request.js 已显示后端的明确错误信息。
      } finally {
        this.activationLoading = false
      }
    },
    ensureDefaultAccount() {
      if (this.defaultAccountID && this.accounts.some(item => item.id === this.defaultAccountID && !item.is_archived)) return
      const preferred = this.accounts.find(item => !item.is_archived && item.type === 'alipay')
        || this.accounts.find(item => !item.is_archived)
      if (!preferred) return
      this.defaultAccountID = preferred.id
      this.defaultPaymentAccountID = null
    },
    async openBookkeepingSettings() {
      if (!this.accounts.length) await this.loadAccounts()
      this.ensureDefaultAccount()
      this.showBookkeepingSettings = true
    },
    onBookkeepingModeChange(event) {
      this.bookkeepingMode = event.detail.value ? 'default' : 'strict'
      if (this.bookkeepingMode === 'default') this.ensureDefaultAccount()
    },
    isDefaultAccountOption(option) {
      return this.defaultAccountID === option.account.id
        && (this.defaultPaymentAccountID || null) === (option.paymentAccount?.id || null)
    },
    selectDefaultAccount(option) {
      this.defaultAccountID = option.account.id
      this.defaultPaymentAccountID = option.paymentAccount?.id || null
    },
    async saveBookkeepingPreference() {
      if (!this.defaultAccountID) {
        uni.showToast({ title: '请选择默认账户', icon: 'none' })
        return
      }
      this.bookkeepingLoading = true
      try {
        const result = await put('/ai/account-preference', {
          account_mode: this.bookkeepingMode,
          default_account_id: this.defaultAccountID,
          default_payment_account_id: this.defaultPaymentAccountID
        })
        this.bookkeepingMode = result?.account_mode === 'strict' ? 'strict' : 'default'
        this.defaultAccountID = result?.default_account_id || this.defaultAccountID
        this.defaultPaymentAccountID = result?.default_payment_account_id || null
        this.showBookkeepingSettings = false
        uni.showToast({ title: '记账方式已保存', icon: 'success' })
      } catch (err) {
        uni.showToast({ title: err.message || '保存失败', icon: 'none' })
      } finally {
        this.bookkeepingLoading = false
      }
    },
    async loadCategories() {
      try {
        const [expense, income] = await Promise.all([
          get('/categories/tree', { kind: 'expense' }),
          get('/categories/tree', { kind: 'income' })
        ])
        this.categories.expense = (expense || []).map(item => ({ ...item, children: item.children || [] }))
        this.categories.income = (income || []).map(item => ({ ...item, children: item.children || [] }))
        // 一级默认折叠：不预设 expandedCategoryIds
      } catch (err) {
        console.error(err)
      }
    },
    switchCategoryKind(kind) {
      this.categoryKind = kind
      this.expandedCategoryIds = {}
    },
    isCategoryExpanded(id) {
      return !!this.expandedCategoryIds[id]
    },
    toggleCategoryGroup(id) {
      this.expandedCategoryIds = {
        ...this.expandedCategoryIds,
        [id]: !this.expandedCategoryIds[id]
      }
    },
    openCategoryModal(parent) {
      this.editingCategory = null
      this.categoryParent = parent
      this.categoryForm = { name: '', icon: parent ? '🛍️' : '📦' }
      if (parent) {
        this.expandedCategoryIds = { ...this.expandedCategoryIds, [parent.id]: true }
      }
      this.showCategoryEditor = true
      this.$nextTick(() => setTimeout(() => this.fixInputs(), 50))
    },
    editCategory(category) {
      this.editingCategory = category
      this.categoryParent = category.parent_id
        ? this.visibleCategories.find(group => group.id === category.parent_id) || null
        : null
      this.categoryForm = {
        name: category.name,
        icon: category.icon || (this.categoryParent ? '🛍️' : '📦')
      }
      if (this.categoryParent) {
        this.expandedCategoryIds = { ...this.expandedCategoryIds, [this.categoryParent.id]: true }
      }
      this.showCategoryEditor = true
      this.$nextTick(() => setTimeout(() => this.fixInputs(), 50))
    },
    async saveCategory() {
      const name = this.categoryForm.name.trim()
      if (!name) {
        uni.showToast({ title: '请输入分类名称', icon: 'none' })
        return
      }
      this.categoryLoading = true
      try {
        if (this.editingCategory) {
          await put(`/categories/${this.editingCategory.id}`, {
            name,
            icon: this.categoryForm.icon,
            sort: this.editingCategory.sort || 0
          })
        } else {
          await post('/categories', {
            name,
            icon: this.categoryForm.icon,
            kind: this.categoryKind,
            parent_id: this.categoryParent?.id || null,
            sort: this.categoryParent ? (this.categoryParent.children?.length || 0) + 1 : this.visibleCategories.length + 1
          })
        }
        this.showCategoryEditor = false
        uni.showToast({ title: '已保存', icon: 'success' })
        await this.loadCategories()
      } catch (err) {
        uni.showToast({ title: err.message || '保存失败', icon: 'none' })
      } finally {
        this.categoryLoading = false
      }
    },
    deleteCategory(category) {
      uni.showModal({
        title: '删除分类',
        content: `确定删除「${category.name}」吗？存在子分类或流水时不能删除。`,
        success: async (res) => {
          if (!res.confirm) return
          try {
            await del(`/categories/${category.id}`)
            uni.showToast({ title: '已删除', icon: 'success' })
            await this.loadCategories()
          } catch (err) {
            uni.showToast({ title: err.message || '删除失败', icon: 'none' })
          }
        }
      })
    },
    showAddAccount() {
      this.newAccountName = ''
      this.newAccountType = 'savings'
      this.linkedAccountIDs = []
      this.editingAccount = null
      this.showAccount = true
      this.$nextTick(() => setTimeout(() => this.fixInputs(), 50))
    },
    showEditAccount(acc) {
      this.editingAccount = acc
      this.newAccountName = acc.name
      this.newAccountType = acc.type === 'bank' ? 'savings' : acc.type
      this.linkedAccountIDs = (acc.linked_accounts || []).map(item => item.id)
      this.showAccount = true
      this.$nextTick(() => setTimeout(() => this.fixInputs(), 50))
    },
    toggleLinkedAccount(id) {
      const index = this.linkedAccountIDs.indexOf(id)
      if (index >= 0) this.linkedAccountIDs.splice(index, 1)
      else this.linkedAccountIDs.push(id)
    },
    async createAccount() {
      if (!this.newAccountName.trim()) {
        uni.showToast({ title: '请输入账户名称', icon: 'none' })
        return
      }
      this.accountLoading = true
      try {
        const payload = {
          name: this.newAccountName.trim(),
          type: this.newAccountType,
          sort: this.editingAccount?.sort || (this.accounts.length + 1) * 10,
          is_archived: this.editingAccount?.is_archived || false,
          linked_account_ids: (this.newAccountType === 'alipay' || this.newAccountType === 'wechat') ? this.linkedAccountIDs : []
        }
        if (this.editingAccount) await put(`/accounts/${this.editingAccount.id}`, payload)
        else await post('/accounts', payload)
        uni.showToast({ title: this.editingAccount ? '已保存' : '已创建', icon: 'success' })
        this.showAccount = false
        this.loadAccounts()
      } catch (err) {
        uni.showToast({ title: err.message || '创建失败', icon: 'none' })
      } finally {
        this.accountLoading = false
      }
    },
    async archiveAccount(acc) {
      try {
        await put(`/accounts/${acc.id}`, {
          name: acc.name,
          sort: acc.sort,
          is_archived: true
        })
        uni.showToast({ title: '已归档', icon: 'success' })
        this.loadAccounts()
      } catch (err) {
        uni.showToast({ title: err.message || '操作失败', icon: 'none' })
      }
    },
    async unarchiveAccount(acc) {
      try {
        await put(`/accounts/${acc.id}`, {
          name: acc.name,
          sort: acc.sort,
          is_archived: false
        })
        uni.showToast({ title: '已恢复', icon: 'success' })
        this.loadAccounts()
      } catch (err) {
        uni.showToast({ title: err.message || '操作失败', icon: 'none' })
      }
    },
    async deleteAccount(acc) {
      uni.showModal({
        title: '删除账户',
        content: `确定删除「${acc.name}」吗？有流水的账户无法删除，可先归档。`,
        success: async (res) => {
          if (!res.confirm) return
          try {
            await del(`/accounts/${acc.id}`)
            uni.showToast({ title: '已删除', icon: 'success' })
            this.loadAccounts()
          } catch (err) {
            uni.showToast({ title: err.message || '删除失败', icon: 'none' })
          }
        }
      })
    },
    logout() {
      uni.removeStorageSync(TOKEN_KEY)
      uni.removeStorageSync(USER_KEY)
      uni.redirectTo({ url: '/pages/login/index' })
    },
    showCreateModal() {
      this.newLedgerName = ''
      this.showCreate = true
      this.$nextTick(() => setTimeout(() => this.fixInputs(), 50))
    },
    async createLedger() {
      if (!this.newLedgerName.trim()) {
        uni.showToast({ title: '请输入账本名称', icon: 'none' })
        return
      }
      this.createLoading = true
      try {
        const result = await post('/ledgers', { name: this.newLedgerName.trim() })
        uni.showToast({ title: '创建成功', icon: 'success' })
        this.showCreate = false
        this.currentLedger = result.ledger || {}
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
    copyInvite() {
      const code = this.currentLedger?.invite_code || ''
      if (!code) return
      // #ifdef H5
      if (navigator.clipboard) {
        navigator.clipboard.writeText(code)
      }
      // #endif
      uni.setClipboardData({
        data: code,
        success: () => uni.showToast({ title: '已复制', icon: 'success' })
      })
      this.showInvite = false
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
    async removeMember(m) {
      uni.showModal({
        title: '移除成员',
        content: `确定移除「${m.nickname || m.username}」吗？`,
        success: async (res) => {
          if (!res.confirm) return
          try {
            await del(`/ledgers/${this.currentLedger.id}/members/${m.user_id}`)
            uni.showToast({ title: '已移除', icon: 'success' })
            this.showMembers(this.currentLedger)
            this.loadLedgers()
          } catch (err) {
            uni.showToast({ title: err.message || '移除失败', icon: 'none' })
          }
        }
      })
    },
    async joinLedger() {
      const code = (this.joinCode || '').trim().toUpperCase()
      if (code.length !== 6) {
        uni.showToast({ title: '请输入 6 位邀请码', icon: 'none' })
        return
      }
      this.joinLoading = true
      try {
        await post('/ledgers/join', { code })
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
          if (!res.confirm) return
          try {
            await post(`/ledgers/${ledger.id}/leave`)
            uni.showToast({ title: '已退出', icon: 'success' })
            this.loadLedgers()
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
.page {
  padding: 22px 16px 30px;
  min-height: 100%;
  background: #f7f9f7;
  box-sizing: border-box;
}
.user-card {
  display: flex;
  align-items: center;
  padding: 20px;
  background: #e8f7f1;
  border-color: #d6eee5;
}
.avatar {
  width: 52px;
  height: 52px;
  flex: 0 0 52px;
  display: flex;
  align-items: center;
  justify-content: center;
  margin-right: 13px;
  color: #fff;
  background: #43c59e;
  border-radius: 18px;
  font-size: 21px;
  font-weight: 700;
  box-shadow: 0 10px 20px rgba(67,197,158,.2);
}
.user-info {
  flex: 1;
  display: flex;
  flex-direction: column;
}
.nickname {
  color: #18312b;
  font-size: 19px;
  font-weight: 700;
  margin-bottom: 4px;
}
.username {
  font-size: 13px;
  color: #789087;
}
.logout-link {
  padding: 7px 10px;
  color: #71817a;
  background: rgba(255,255,255,.7);
  border-radius: 10px;
  font-size: 12px;
}
.section {
  margin-top: 22px;
}
.section-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  margin-bottom: 8px;
  gap: 12px;
}
.section-title {
  flex: 1;
  min-width: 0;
  color: #263f38;
  font-size: 17px;
  font-weight: 700;
}
.section-action {
  flex-shrink: 0;
  padding: 4px 0 4px 8px;
  color: #36b88f;
  font-size: 13px;
  font-weight: 600;
  line-height: 1.4;
  white-space: nowrap;
  cursor: pointer;
}
.manage-grid { display: grid; grid-template-columns: 1fr; gap: 10px; }
.manage-card {
  min-height: 76px;
  padding: 14px 16px;
  display: flex;
  align-items: center;
  gap: 12px;
  background: #fff;
  border: 1px solid #edf1ee;
  border-radius: 17px;
  box-shadow: 0 7px 24px rgba(24,49,43,.035);
}
.manage-icon {
  width: 40px; height: 40px; flex: 0 0 40px;
  display: flex; align-items: center; justify-content: center;
  border-radius: 13px; font-size: 13px; font-weight: 700;
}
.manage-icon.mint { color: #2cae86; background: #e7f7f1; }
.manage-icon.amber { color: #df9220; background: #fff4df; }
.manage-icon.blue { color: #4b7ea8; background: #eaf3f9; }
.manage-icon.violet { color: #7662b2; background: #f0edfb; }
.manage-main { flex: 1; min-width: 0; }
.manage-title { display: block; color: #263f38; font-size: 15px; font-weight: 650; }
.manage-copy { display: block; margin-top: 4px; color: #87968f; font-size: 11px; }
.manage-chevron { color: #aab5b0; font-size: 23px; }
@media (min-width: 600px) { .manage-grid { grid-template-columns: 1fr 1fr; } }
.ledger-item {
  display: flex;
  justify-content: space-between;
  align-items: flex-start;
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
  color: #263f38;
  font-weight: 600;
}
.ledger-badge {
  font-size: 11px;
  padding: 3px 7px;
  border-radius: 999px;
  background: #e8f7f1;
  color: #36b88f;
}
.ledger-badge.owner {
  background: #fff4e6;
  color: #d98b22;
}
.ledger-meta {
  font-size: 12px;
  color: #999;
  display: block;
}
.ledger-tip {
  font-size: 11px;
  color: #bbb;
  margin-top: 4px;
  display: block;
}
.ledger-actions {
  display: flex;
  gap: 12px;
  margin-left: 12px;
  flex-shrink: 0;
}
.action-link {
  font-size: 13px;
  color: #36b88f;
}
.action-link.danger {
  color: #f06f64;
}
.empty {
  text-align: center;
  color: #999;
  padding: 24px;
}
.join-row {
  display: flex;
  align-items: center;
  gap: 8px;
}
.join-btn {
  width: auto !important;
  flex-shrink: 0;
  min-height: 40px;
  margin: 0;
  padding: 8px 16px;
  white-space: nowrap;
}
.input {
  flex: 1;
  min-width: 0;
}
:deep(.join-input .uni-input-wrapper),
:deep(.create-input .uni-input-wrapper) {
  width: 100%;
  height: 48px;
  padding: 0 13px;
  display: flex;
  align-items: center;
  border: 1px solid #e1e9e5;
  border-radius: 13px;
  background: #fbfcfb;
  box-sizing: border-box;
}
.join-input,
.create-input {
  height: 48px;
  padding: 0 !important;
  box-sizing: border-box;
}
:deep(.join-input input),
:deep(.create-input input) {
  height: 24px !important;
  line-height: 24px !important;
  padding: 0 8px !important;
  margin: 0 !important;
}
.modal {
  position: fixed;
  top: 0; left: 0; right: 0; bottom: 0;
  background: rgba(24, 49, 43, 0.32);
  display: flex;
  align-items: center;
  justify-content: center;
  z-index: 999;
}
.modal-content {
  background: #fff;
  border-radius: 20px;
  padding: 24px;
  width: 80%;
  max-width: 360px;
}
.preference-modal {
  max-height: 82vh;
  overflow-y: auto;
  box-sizing: border-box;
}
.entitlement-modal { box-sizing: border-box; }
.entitlement-state {
  padding: 14px;
  display: flex;
  align-items: center;
  gap: 12px;
  color: #6c5a31;
  background: #fff8e8;
  border: 1px solid #f3e5c4;
  border-radius: 16px;
}
.entitlement-state.activated { color: #277c63; background: #ecf9f4; border-color: #d3eee4; }
.entitlement-state.expired { color: #956153; background: #fff3ef; border-color: #f3ddd6; }
.entitlement-state-icon {
  width: 40px;
  height: 40px;
  flex: 0 0 40px;
  display: flex;
  align-items: center;
  justify-content: center;
  color: #fff;
  background: #d7a84f;
  border-radius: 13px;
  font-size: 12px;
  font-weight: 750;
}
.entitlement-state.activated .entitlement-state-icon { background: #42c39c; font-size: 18px; }
.entitlement-state.expired .entitlement-state-icon { background: #d78b74; }
.entitlement-state-main { flex: 1; min-width: 0; }
.entitlement-state-title { display: block; font-size: 14px; font-weight: 700; }
.entitlement-state-copy { display: block; margin-top: 4px; font-size: 11px; line-height: 17px; opacity: .8; }
.entitlement-usage {
  margin-top: 10px;
  padding: 9px 12px;
  display: flex;
  justify-content: space-between;
  color: #71817a;
  background: #f7f9f7;
  border-radius: 11px;
  font-size: 11px;
}
.activation-section { margin-top: 18px; }
.entitlement-loading { padding: 8px 0; }
.activation-title { display: block; color: #263f38; font-size: 14px; font-weight: 700; }
.activation-tip { display: block; margin: 5px 0 10px; color: #8b9892; font-size: 11px; line-height: 18px; }
.activation-input { width: 100%; letter-spacing: 1px; }
.activation-btn { width: 100% !important; margin-top: 12px; }
.entitlement-permanent {
  margin-top: 14px;
  padding: 11px 13px;
  color: #5b7068;
  background: #f3f8f6;
  border-radius: 12px;
  font-size: 11px;
  line-height: 18px;
}
.entitlement-close { width: 100% !important; margin-top: 12px; }
.preference-toggle-row {
  min-height: 62px;
  padding: 12px 14px;
  display: flex;
  align-items: center;
  gap: 12px;
  background: #f7faf8;
  border-radius: 15px;
  box-sizing: border-box;
}
.preference-toggle-copy { flex: 1; min-width: 0; }
.preference-toggle-title { display: block; color: #263f38; font-size: 14px; font-weight: 650; }
.preference-toggle-tip { display: block; margin-top: 4px; color: #87968f; font-size: 11px; }
.preference-note {
  margin-top: 12px;
  padding: 12px 14px;
  display: flex;
  flex-direction: column;
  gap: 5px;
  color: #60746c;
  background: #eff8f4;
  border: 1px solid #dcefe7;
  border-radius: 14px;
  font-size: 11px;
  line-height: 1.55;
}
.preference-note.strict { color: #775f3d; background: #fff8eb; border-color: #f2e3c4; }
.preference-note-title { color: #2cae86; font-size: 13px; font-weight: 700; }
.preference-note.strict .preference-note-title { color: #c1842b; }
.default-account-section { margin-top: 18px; }
.default-account-title { display: block; color: #263f38; font-size: 14px; font-weight: 700; }
.default-account-current { display: block; margin-top: 4px; color: #42b990; font-size: 11px; }
.default-account-group { margin-top: 14px; }
.default-account-group-head {
  display: flex;
  justify-content: space-between;
  margin-bottom: 6px;
  color: #526860;
  font-size: 12px;
  font-weight: 650;
}
.default-account-group-head text:last-child { color: #9aa7a1; font-size: 10px; font-weight: 400; }
.default-account-option {
  min-height: 48px;
  padding: 8px 10px;
  display: flex;
  align-items: center;
  gap: 10px;
  border: 1px solid #e6ece9;
  border-radius: 12px;
  margin-top: 7px;
  box-sizing: border-box;
}
.default-account-option.selected { border-color: #42c39c; background: #f0faf6; }
.default-check {
  width: 20px;
  height: 20px;
  flex: 0 0 20px;
  display: flex;
  align-items: center;
  justify-content: center;
  color: #fff;
  background: #dfe7e3;
  border-radius: 7px;
  font-size: 12px;
}
.default-account-option.selected .default-check { background: #42c39c; }
.default-account-main { display: flex; flex-direction: column; color: #354d46; font-size: 13px; }
.default-account-main text:last-child:not(:first-child) { margin-top: 2px; color: #94a19b; font-size: 10px; }
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
  width: auto !important;
  margin: 0;
}
.modal-content > .btn {
  width: 100% !important;
  margin: 0;
}
.invite-code {
  font-size: 32px;
  font-weight: bold;
  color: #36b88f;
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
  margin-bottom: 12px;
}
.member-item {
  display: flex;
  justify-content: space-between;
  align-items: center;
  padding: 12px 0;
  border-bottom: 1px solid #f0f0f0;
}
.member-name {
  font-size: 15px;
  margin-right: 8px;
}
.member-role {
  font-size: 12px;
  color: #999;
}
.account-item {
  display: flex;
  justify-content: space-between;
  align-items: flex-start;
}
.account-main {
  flex: 1;
  min-width: 0;
}
.account-name {
  font-size: 15px;
  font-weight: 500;
  display: block;
  margin-bottom: 4px;
}
.account-meta {
  font-size: 12px;
  color: #999;
  display: block;
}
.account-links {
  margin-top: 4px;
  color: #36b88f;
  font-size: 12px;
  display: block;
}
.account-archived {
  font-size: 11px;
  color: #ff9500;
  margin-top: 4px;
  display: block;
}
.type-row {
  display: flex;
  flex-wrap: wrap;
  gap: 8px;
  margin: 16px 0 8px;
}
.type-chip {
  padding: 6px 12px;
  border: 1px solid #dce6e2;
  border-radius: 16px;
  font-size: 13px;
  color: #666;
}
.type-chip.active {
  background: #42c39c;
  color: #fff;
  border-color: #42c39c;
}
.link-section {
  margin-top: 16px;
  padding: 14px;
  border-radius: 14px;
  background: #f5faf8;
}
.link-title { display: block; color: #263f38; font-size: 14px; font-weight: 600; }
.link-tip { display: block; margin: 4px 0 10px; color: #87968f; font-size: 11px; }
.link-option {
  display: flex;
  gap: 8px;
  padding: 10px 0;
  color: #71817a;
  font-size: 13px;
  border-bottom: 1px solid #e6efeb;
}
.link-option.selected { color: #36b88f; }
.link-empty { color: #9aa6a1; font-size: 12px; }
.category-kind-tabs {
  display: flex;
  gap: 8px;
  margin-bottom: 10px;
}
.category-kind-tabs text {
  flex: 1;
  padding: 9px 0;
  border-radius: 12px;
  color: #71817a;
  background: #edf1ef;
  text-align: center;
  font-size: 13px;
}
.category-kind-tabs text.active { color: #fff; background: #42c39c; }
.category-group { padding: 4px 16px; }
.category-chevron {
  width: 16px;
  color: #87968f;
  font-size: 16px;
  transition: transform .15s ease;
  display: inline-block;
}
.category-chevron.open { transform: rotate(90deg); }
.category-count {
  margin-left: 6px;
  font-size: 11px;
  color: #9aa6a1;
  background: #f2f7f5;
  border-radius: 999px;
  padding: 1px 7px;
}
.category-child-empty {
  padding: 10px 0 12px 38px;
  font-size: 12px;
  color: #9aa6a1;
}
.category-parent-row,
.category-child-row {
  min-height: 52px;
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
  border-bottom: 1px solid #edf1ee;
}
.category-child-row:last-child { border-bottom: 0; }
.category-child-row { padding-left: 38px; color: #526860; font-size: 13px; }
.category-name-wrap { display: flex; align-items: center; min-width: 0; }
.category-chevron {
  width: 16px;
  color: #87968f;
  font-size: 16px;
  transition: transform .15s ease;
  display: inline-block;
}
.category-chevron.open { transform: rotate(90deg); }
.category-count {
  margin-left: 6px;
  font-size: 11px;
  color: #9aa6a1;
  background: #f2f7f5;
  border-radius: 999px;
  padding: 1px 7px;
}
.category-child-empty {
  padding: 10px 0 12px 38px;
  font-size: 12px;
  color: #9aa6a1;
}
.category-icon { width: 30px; font-size: 20px; }
.category-name { color: #263f38; font-size: 15px; font-weight: 600; }
.category-actions { display: flex; gap: 10px; flex-shrink: 0; color: #36b88f; font-size: 12px; }
.category-actions .danger { color: #f06f64; }
.category-parent-tip { display: block; margin-bottom: 12px; color: #71817a; font-size: 13px; }
.icon-picker-title { display: block; margin: 16px 0 8px; color: #526860; font-size: 13px; }
.icon-picker { display: flex; flex-wrap: wrap; gap: 8px; }
.icon-picker text {
  width: 34px;
  height: 34px;
  display: flex;
  align-items: center;
  justify-content: center;
  border: 1px solid #e1e9e5;
  border-radius: 10px;
  background: #fbfcfb;
  font-size: 18px;
}
.icon-picker text.active { border-color: #42c39c; background: #e8f7f1; }
</style>
