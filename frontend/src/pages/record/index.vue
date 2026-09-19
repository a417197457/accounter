<template>
  <view class="page">
    <WebSidebar />
    <view class="record-head">
      <text class="eyebrow">PERSONAL LEDGER</text>
      <text class="title">{{ editingID ? '编辑流水' : '记一笔' }}</text>
      <text class="subtitle">{{ editingID ? '修改原流水后，已同步的副本也会更新' : '记录会默认保存到个人账本' }}</text>
    </view>

    <view class="type-switch">
      <view class="type-option" :class="{ active: form.op_type === 'expense' }" @tap="switchOpType('expense')">支出</view>
      <view class="type-option" :class="{ active: form.op_type === 'income' }" @tap="switchOpType('income')">收入</view>
      <view class="type-option" :class="{ active: form.op_type === 'transfer' }" @tap="switchOpType('transfer')">转账/还款</view>
      <view
        class="type-option"
        v-if="editingID && (form.op_type === 'borrow_in' || form.op_type === 'borrow_out')"
        :class="{ active: true }"
      >{{ form.op_type === 'borrow_in' ? '借入' : '借出' }}</view>
    </view>

    <view class="amount-panel">
      <text class="currency">¥</text>
      <view class="amount-input-box">
          <input
            class="record-input"
            type="digit"
            v-model="form.amount"
            placeholder="0.00"
          />
      </view>
    </view>

    <view class="card detail-card">
      <view class="detail-row" v-if="form.op_type !== 'transfer'" @click="showCategoryModal">
        <view class="detail-icon mint"><image src="/static/ui/category.svg" mode="aspectFit" /></view>
        <text class="detail-label">分类</text>
        <text class="detail-value">{{ categoryLabel || '请选择' }}</text>
        <text class="chevron">›</text>
      </view>
      <view class="detail-row" @click="showAccountModal">
        <view class="detail-icon blue"><image src="/static/ui/wallet.svg" mode="aspectFit" /></view>
        <text class="detail-label">{{ form.op_type === 'transfer' ? '转出' : '账户' }}</text>
        <text class="detail-value">{{ selectedAccountLabel || '请选择' }}</text>
        <text class="chevron">›</text>
      </view>
      <view class="detail-row" v-if="form.op_type === 'transfer'" @click="showTargetAccountModal">
        <view class="detail-icon mint"><image src="/static/ui/arrow-in.svg" mode="aspectFit" /></view>
        <text class="detail-label">转入</text>
        <text class="detail-value">{{ selectedTargetAccount?.name || '请选择' }}</text>
        <text class="chevron">›</text>
      </view>
      <view class="detail-row">
        <view class="detail-icon amber"><image src="/static/ui/calendar.svg" mode="aspectFit" /></view>
        <text class="detail-label">日期</text>
        <picker class="detail-picker" mode="date" :value="form.date" @change="onDateChange">
          <view class="picker-inner">
            <text class="detail-value">{{ form.date }}</text>
            <text class="chevron">›</text>
          </view>
        </picker>
      </view>
      <view class="detail-row">
        <view class="detail-icon amber"><image src="/static/ui/calendar.svg" mode="aspectFit" /></view>
        <text class="detail-label">时间</text>
        <picker class="detail-picker" mode="time" :value="form.time" @change="onTimeChange">
          <view class="picker-inner">
            <text class="detail-value">{{ form.time }}</text>
            <text class="chevron">›</text>
          </view>
        </picker>
      </view>
      <view class="detail-row last">
        <view class="detail-icon gray"><image src="/static/ui/note.svg" mode="aspectFit" /></view>
        <text class="detail-label">备注</text>
        <view class="remark-box">
          <input
            class="record-input"
            type="text"
            v-model="form.remark"
            placeholder="写点什么"
          />
        </view>
      </view>
    </view>

    <button class="btn btn-primary submit-btn" @click="handleSubmit" :loading="loading">{{ !loggedIn ? '登录后保存' : (editingID ? '保存修改' : '保存记录') }}</button>
    <button class="btn btn-secondary cancel-edit-btn" v-if="editingID" @click="cancelEditing">取消编辑</button>

    <view class="modal" v-if="showAccount" @tap="showAccount = false">
      <view class="modal-content" @tap.stop>
        <text class="modal-title">{{ form.op_type === 'transfer' ? '选择转出账户' : '选择扣款方式' }}</text>
        <view class="list">
          <view class="account-group" v-for="group in accountOptionGroups" :key="group.key">
            <view class="account-group-head">
              <text>{{ group.title }}</text>
              <text>{{ group.copy }}</text>
            </view>
            <view
              class="list-item account-option"
              v-for="option in group.options"
              :key="`${option.paymentAccount?.id || 0}-${option.account.id}`"
              @click="selectAccountOption(option)"
            >
              <view class="account-mini-icon" :class="option.account.type">{{ accountTypeIcon(option.account.type) }}</view>
              <view class="account-option-main">
                <text>{{ option.label }}</text>
                <text class="option-hint" v-if="option.paymentAccount">实际计入 {{ option.account.name }}</text>
              </view>
              <text class="balance">{{ accountMoneyText(option.account) }}</text>
            </view>
          </view>
        </view>
        <button class="btn btn-secondary" @click="showAccount = false">取消</button>
      </view>
    </view>

    <view class="modal" v-if="showTargetAccount" @tap="showTargetAccount = false">
      <view class="modal-content" @tap.stop>
        <text class="modal-title">选择转入账户</text>
        <view class="list">
          <view
            class="list-item"
            v-for="acc in targetAccounts"
            :key="acc.id"
            @click="selectTargetAccount(acc)"
          >
            <text>{{ acc.name }}</text>
            <text class="balance">{{ acc.type === 'credit' ? '还款后 · ' : '转入 · ' }}{{ accountMoneyText(acc) }}</text>
          </view>
        </view>
        <button class="btn btn-secondary" @click="showTargetAccount = false">取消</button>
      </view>
    </view>

    <view class="modal" v-if="showCategory" @tap="showCategory = false">
      <view class="modal-content" @tap.stop>
        <text class="modal-title">选择分类</text>
        <view class="category-tabs">
          <text class="tab" :class="{ active: tab === 'expense' }" @click="tab = 'expense'">支出</text>
          <text class="tab" :class="{ active: tab === 'income' }" @click="tab = 'income'">收入</text>
        </view>
        <view class="list">
          <view v-for="group in filteredCategoryGroups" :key="group.id" class="cat-group">
            <view class="cat-parent" @click="toggleRecordCategory(group.id)">
              <text class="cat-chevron" :class="{ open: isRecordCatOpen(group.id) }">›</text>
              <text>{{ group.icon || '📦' }} {{ group.name }}</text>
              <text class="cat-count" v-if="(group.children || []).length">{{ group.children.length }}</text>
            </view>
            <view v-if="isRecordCatOpen(group.id)">
              <view
                class="list-item child"
                v-for="child in group.children"
                :key="child.id"
                @click="selectCategory(child, group)"
              >
                <text>{{ child.icon || '·' }} {{ child.name }}</text>
              </view>
              <view
                v-if="!group.children || group.children.length === 0"
                class="list-item child"
                @click="selectCategory(group, null)"
              >
                <text>{{ group.icon || '📦' }} 使用「{{ group.name }}」</text>
              </view>
            </view>
          </view>
        </view>
        <button class="btn btn-secondary" @click="showCategory = false">取消</button>
      </view>
    </view>
  </view>
</template>

<script>
import { get, post, put } from '../../common/request.js'
import { isLoggedIn, requireLogin } from '../../common/auth.js'
import { accountMoneyText } from '../../common/money.js'

const EDIT_TRANSACTION_KEY = 'edit_transaction'

function currentTime() {
  const now = new Date()
  return `${String(now.getHours()).padStart(2, '0')}:${String(now.getMinutes()).padStart(2, '0')}`
}

function emptyForm() {
  return {
    account_id: null,
    payment_account_id: null,
    to_account_id: null,
    category_id: null,
    op_type: 'expense',
    amount: '',
    remark: '',
    date: new Date().toISOString().split('T')[0],
    time: currentTime()
  }
}

export default {
  data() {
    return {
      form: emptyForm(),
      editingID: null,
      loading: false,
      selectedAccount: null,
      selectedPaymentAccount: null,
      selectedTargetAccount: null,
      selectedCategory: null,
      selectedParent: null,
      accounts: [],
      categories: { expense: [], income: [] },
      tab: 'expense',
      showAccount: false,
      showTargetAccount: false,
      showCategory: false,
      loggedIn: false,
      openRecordCats: {}
    }
  },
  computed: {
    filteredCategoryGroups() {
      return this.categories[this.tab] || []
    },
    categoryLabel() {
      if (!this.selectedCategory) return ''
      if (this.selectedParent) {
        return `${this.selectedParent.icon || ''} ${this.selectedParent.name} / ${this.selectedCategory.name}`
      }
      return `${this.selectedCategory.icon || ''} ${this.selectedCategory.name}`
    },
    accountOptionGroups() {
      if (this.form.op_type === 'transfer') {
        return [
          { key: 'asset', title: '资产账户', copy: '从余额转出', options: this.accounts.filter(a => a.type !== 'credit').map(account => ({ account, paymentAccount: null, label: account.name })) },
          { key: 'credit', title: '信用卡', copy: '取现或转账', options: this.accounts.filter(a => a.type === 'credit').map(account => ({ account, paymentAccount: null, label: account.name })) }
        ].filter(group => group.options.length)
      }
      const groups = this.accounts
        .filter(account => account.type === 'alipay' || account.type === 'wechat')
        .map(payment => ({
          key: `payment-${payment.id}`,
          title: payment.name,
          copy: '余额或关联银行卡',
          options: [
            { account: payment, paymentAccount: null, label: `${payment.name}余额` },
            ...(payment.linked_accounts || []).map(account => ({ account, paymentAccount: payment, label: account.name }))
          ]
        }))
      const direct = this.accounts.filter(account => account.type !== 'alipay' && account.type !== 'wechat')
      if (direct.length) {
        groups.push({ key: 'direct', title: '直接使用账户', copy: '不经过支付渠道', options: direct.map(account => ({ account, paymentAccount: null, label: account.name })) })
      }
      return groups
    },
    selectedAccountLabel() {
      if (!this.selectedAccount) return ''
      return this.selectedPaymentAccount
        ? `${this.selectedPaymentAccount.name} · ${this.selectedAccount.name}`
        : ((this.selectedAccount.type === 'alipay' || this.selectedAccount.type === 'wechat') ? `${this.selectedAccount.name}余额` : this.selectedAccount.name)
    },
    targetAccounts() {
      return this.accounts.filter(account => account.id !== this.form.account_id)
    }
  },
  onLoad() {},
  async onShow() {
    this.loggedIn = isLoggedIn()
    const editTransaction = uni.getStorageSync(EDIT_TRANSACTION_KEY)
    if (editTransaction) uni.removeStorageSync(EDIT_TRANSACTION_KEY)
    if (this.loggedIn) await this.loadData()
    else {
      this.accounts = []
      this.categories = { expense: [], income: [] }
    }
    if (editTransaction) this.startEditing(editTransaction)
    this.$nextTick(() => setTimeout(() => this.fixInputHeight(), 100))
  },
  methods: {
    accountMoneyText,
    accountTypeIcon(type) {
      return ({ alipay: '支', wechat: '微', cash: '现', savings: '储', bank: '储', credit: '信' })[type] || '账'
    },
    switchOpType(type) {
      if (this.form.op_type === type) return
      this.form.op_type = type
      this.tab = type === 'borrow_in' ? 'income' : (type === 'borrow_out' ? 'expense' : type)
      this.selectedCategory = null
      this.selectedParent = null
      this.form.category_id = null
      this.form.payment_account_id = null
      this.form.to_account_id = null
      this.selectedPaymentAccount = null
      this.selectedTargetAccount = null
    },
    fixInputHeight() {
      // #ifdef H5
      const inputs = document.querySelectorAll('.record-input input')
      inputs.forEach(input => {
        const isAmount = Boolean(input.closest('.amount-input-box'))
        input.style.height = isAmount ? '72px' : '24px'
        input.style.lineHeight = isAmount ? '72px' : '24px'
        input.style.padding = '0 8px'
        input.style.margin = '0'
        input.style.border = 'none'
        input.style.outline = 'none'
        input.style.background = 'transparent'
        input.style.width = '100%'
        input.style.display = 'block'
        input.style.userSelect = 'auto'
        input.style.webkitUserSelect = 'auto'
      })
      // #endif
    },
    async loadData() {
      try {
        // 不传 ledgerId → 个人账本
        const accounts = await get('/accounts')
        this.accounts = (accounts || []).filter(a => !a.is_archived)
        if (this.accounts.length > 0) {
          const stillValid = this.selectedAccount && this.accounts.find(a => a.id === this.selectedAccount.id)
          if (!stillValid) {
            const preferred = this.accounts.find(a => a.type === 'alipay') || this.accounts[0]
            this.selectedAccount = preferred
            this.selectedPaymentAccount = null
            this.form.account_id = preferred.id
            this.form.payment_account_id = null
          }
        }

        const expenseCats = await get('/categories/tree', { kind: 'expense' })
        const incomeCats = await get('/categories/tree', { kind: 'income' })
        this.categories.expense = (expenseCats || []).map(c => ({
          ...c,
          children: c.children || []
        }))
        this.categories.income = (incomeCats || []).map(c => ({
          ...c,
          children: c.children || []
        }))
      } catch (err) {
        if (err?.code !== 'UNAUTHORIZED') console.error('加载记账选项失败:', err)
      }
    },
    showAccountModal() {
      if (!this.loggedIn) return requireLogin('登录后即可选择账户并保存这笔记录')
      this.showAccount = true
    },
    showTargetAccountModal() {
      if (!this.loggedIn) return requireLogin('登录后即可选择转入账户并保存转账')
      this.showTargetAccount = true
    },
    selectAccountOption(option) {
      this.selectedAccount = option.account
      this.selectedPaymentAccount = option.paymentAccount
      this.form.account_id = option.account.id
      this.form.payment_account_id = option.paymentAccount?.id || null
      if (this.form.to_account_id === option.account.id) {
        this.form.to_account_id = null
        this.selectedTargetAccount = null
      }
      this.showAccount = false
    },
    selectTargetAccount(acc) {
      this.selectedTargetAccount = acc
      this.form.to_account_id = acc.id
      this.showTargetAccount = false
    },
    showCategoryModal() {
      if (!this.loggedIn) return requireLogin('登录后即可选择自己的记账分类')
      this.openRecordCats = {}
      this.showCategory = true
    },
    isRecordCatOpen(id) {
      return !!this.openRecordCats[id]
    },
    toggleRecordCategory(id) {
      this.openRecordCats = {
        ...this.openRecordCats,
        [id]: !this.openRecordCats[id]
      }
    },
    selectCategory(cat, parent) {
      this.selectedCategory = cat
      this.selectedParent = parent
      this.form.category_id = cat.id
      const kind = cat.kind || parent?.kind || this.tab
      const currentKind = this.form.op_type === 'borrow_in' ? 'income' : (this.form.op_type === 'borrow_out' ? 'expense' : this.form.op_type)
      if (kind !== currentKind || this.form.op_type === 'transfer') this.form.op_type = kind
      this.showCategory = false
    },
    onDateChange(e) {
      this.form.date = e.detail.value
    },
    onTimeChange(e) {
      this.form.time = e.detail.value
    },
    findCategorySelection(id) {
      for (const kind of ['expense', 'income']) {
        for (const parent of this.categories[kind] || []) {
          if (parent.id === id) return { category: parent, parent: null }
          const child = (parent.children || []).find(item => item.id === id)
          if (child) return { category: child, parent }
        }
      }
      return null
    },
    startEditing(tx) {
      if (!tx?.id || tx.source_id) return
      this.editingID = tx.id
      const occurredAt = String(tx.occurred_at || '')
      this.form = {
        account_id: tx.account_id,
        payment_account_id: tx.payment_account_id || null,
        to_account_id: tx.to_account_id || null,
        category_id: tx.category_id || null,
        op_type: tx.op_type,
        amount: String(tx.amount || ''),
        remark: tx.remark || '',
        date: occurredAt.slice(0, 10) || new Date().toISOString().split('T')[0],
        time: occurredAt.length >= 16 ? occurredAt.slice(11, 16) : '12:00'
      }
      this.tab = tx.op_type === 'borrow_in' ? 'income' : (tx.op_type === 'borrow_out' ? 'expense' : tx.op_type)
      this.selectedAccount = this.accounts.find(item => item.id === tx.account_id) || tx.account || null
      this.selectedPaymentAccount = this.accounts.find(item => item.id === tx.payment_account_id) || tx.payment_account || null
      this.selectedTargetAccount = this.accounts.find(item => item.id === tx.to_account_id) || tx.to_account || null
      const categorySelection = this.findCategorySelection(tx.category_id)
      this.selectedCategory = categorySelection?.category || tx.category || null
      this.selectedParent = categorySelection?.parent || tx.category?.parent || null
    },
    cancelEditing() {
      this.editingID = null
      this.form = emptyForm()
      this.selectedAccount = null
      this.selectedPaymentAccount = null
      this.selectedCategory = null
      this.selectedParent = null
      this.selectedTargetAccount = null
      uni.switchTab({ url: '/pages/list/index' })
    },
    async handleSubmit() {
      if (!this.loggedIn) {
        requireLogin('登录后即可保存这笔记录，当前填写内容会保留在页面中')
        return
      }
      if (!this.form.account_id) {
        uni.showToast({ title: '请选择账户', icon: 'none' })
        return
      }
      if (this.form.op_type !== 'transfer' && !this.form.category_id) {
        uni.showToast({ title: '请选择分类', icon: 'none' })
        return
      }
      if (this.form.op_type === 'transfer' && !this.form.to_account_id) {
        uni.showToast({ title: '请选择转入账户', icon: 'none' })
        return
      }
      if (!this.form.amount || parseFloat(this.form.amount) <= 0) {
        uni.showToast({ title: '请输入金额', icon: 'none' })
        return
      }

      this.loading = true
      try {
        const payload = {
          account_id: this.form.account_id,
          payment_account_id: this.form.payment_account_id || undefined,
          to_account_id: this.form.op_type === 'transfer' ? this.form.to_account_id : undefined,
          category_id: this.form.op_type === 'transfer' ? undefined : this.form.category_id,
          op_type: this.form.op_type,
          amount: this.form.amount,
          remark: this.form.remark,
          occurred_at: `${this.form.date} ${this.form.time || '12:00'}:00`
        }
        if (this.editingID) await put(`/transactions/${this.editingID}`, payload)
        else await post('/transactions', payload)
        uni.showToast({ title: this.editingID ? '修改成功' : '记账成功', icon: 'success' })
        this.editingID = null
        this.form.amount = ''
        this.form.remark = ''
        setTimeout(() => {
          uni.switchTab({ url: '/pages/list/index' })
        }, 500)
      } catch (err) {
        uni.showToast({ title: err.message || '记账失败', icon: 'none' })
      } finally {
        this.loading = false
      }
    }
  }
}
</script>

<style scoped>
.page {
  padding: 22px 16px 30px;
  min-height: 100vh;
  background: #f7f9f7;
  box-sizing: border-box;
}
.record-head { padding: 0 4px 18px; }
.eyebrow {
  color: #36b88f;
  font-size: 10px;
  font-weight: 700;
  letter-spacing: 1.8px;
  display: block;
}
.title {
  margin-top: 5px;
  font-size: 27px;
  font-weight: 700;
  display: block;
  color: #18312b;
}
.subtitle {
  font-size: 13px;
  color: #87968f;
  display: block;
  margin-top: 6px;
}
.type-switch {
  display: flex;
  padding: 4px;
  border-radius: 15px;
  background: #edf1ef;
  margin-bottom: 22px;
}
.type-option {
  flex: 1;
  padding: 10px 0;
  border-radius: 12px;
  text-align: center;
  color: #7f8e88;
  font-size: 14px;
  font-weight: 600;
}
.type-option.active {
  background: #fff;
  color: #36b88f;
  box-shadow: 0 4px 12px rgba(24, 49, 43, 0.06);
}
.amount-panel {
  min-height: 116px;
  display: flex;
  justify-content: center;
  align-items: center;
  margin-bottom: 20px;
}
.currency {
  align-self: flex-start;
  margin-top: 27px;
  margin-right: 10px;
  color: #18312b;
  font-size: 28px;
  font-weight: 600;
}
.amount-input-box {
  width: 230px;
  min-width: 0;
}
.amount-input-box .record-input { height: 72px; }
.amount-input-box :deep(.uni-input-wrapper) {
  height: 72px;
  padding: 0 !important;
  display: flex;
  align-items: center;
}
.amount-input-box :deep(input) {
  height: 72px !important;
  line-height: 72px !important;
  color: #18312b !important;
  font-size: 56px !important;
  font-weight: 700 !important;
  text-align: left;
  padding: 0 8px !important;
  font-variant-numeric: tabular-nums;
}
.detail-card { padding: 4px 18px; }
.detail-row {
  min-height: 64px;
  display: flex;
  align-items: center;
  border-bottom: 1px solid #edf1ee;
}
.detail-row.last { border-bottom: 0; }
.detail-icon {
  width: 32px;
  height: 32px;
  flex: 0 0 32px;
  display: flex;
  justify-content: center;
  align-items: center;
  margin-right: 12px;
  border-radius: 11px;
  font-size: 12px;
  font-weight: 700;
}
.detail-icon image { width: 19px; height: 19px; }
.detail-icon.mint { color: #36b88f; background: #e7f7f1; }
.detail-icon.blue { color: #4b86db; background: #eaf2fe; }
.detail-icon.amber { color: #e99a24; background: #fff4df; }
.detail-icon.gray { color: #7f8e88; background: #eef1f0; }
.detail-label {
  width: 46px;
  color: #314d45;
  font-size: 15px;
  font-weight: 600;
}
.detail-value {
  flex: 1;
  min-width: 0;
  color: #71817a;
  font-size: 14px;
  text-align: right;
}
.chevron { margin-left: 8px; color: #aab5b0; font-size: 22px; flex-shrink: 0; }
.detail-picker {
  flex: 1;
  min-width: 0;
  width: 100%;
}
.picker-inner {
  display: flex;
  align-items: center;
  width: 100%;
  min-height: 64px;
}
.picker-inner .detail-value {
  flex: 1;
  min-width: 0;
  text-align: right;
}
.remark-box { flex: 1; min-width: 0; }
.remark-box .record-input { height: 32px; }
.remark-box :deep(.uni-input-wrapper) { height: 32px; display: flex; align-items: center; }
.remark-box :deep(input) { padding: 0 8px !important; text-align: right; color: #71817a !important; font-size: 14px !important; }
.remark-box :deep(.uni-input-placeholder) { text-align: right; }
.record-input {
  display: block;
  width: 100%;
  box-sizing: border-box;
}
:deep(.record-input) {
  padding: 0 !important;
  border: none !important;
}
:deep(.record-input .uni-input-wrapper) {
  width: 100%;
  padding: 0 !important;
  border: none !important;
  background: transparent;
}
:deep(.record-input input) {
  width: 100% !important;
  height: 24px !important;
  line-height: 24px !important;
  font-size: 16px !important;
  outline: none !important;
  background: transparent !important;
}
.submit-btn {
  margin-top: 20px;
  width: 100% !important;
  min-height: 52px;
  height: 52px;
  font-size: 16px;
  border-radius: 16px;
  display: flex;
  align-items: center;
  justify-content: center;
  line-height: 1.4 !important;
}
.cancel-edit-btn { width: 100% !important; min-height: 46px; margin-top: 10px; border-radius: 15px; font-size: 14px; }
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
  max-width: 400px;
  max-height: 80vh;
  overflow-y: auto;
}
.modal-title {
  font-size: 18px;
  font-weight: bold;
  margin-bottom: 16px;
  display: block;
  text-align: center;
}
.category-tabs {
  display: flex;
  gap: 8px;
  margin-bottom: 16px;
}
.tab {
  flex: 1;
  padding: 8px;
  text-align: center;
  border: 1px solid #dce6e2;
  border-radius: 12px;
}
.tab.active {
  background: #42c39c;
  color: #fff;
  border-color: #42c39c;
}
.list {
  max-height: 400px;
  overflow-y: auto;
}
.cat-group {
  margin-bottom: 8px;
}
.cat-parent {
  font-size: 13px;
  color: #526860;
  padding: 10px 4px;
  display: flex;
  align-items: center;
  gap: 6px;
  font-weight: 600;
}
.cat-chevron {
  width: 14px;
  color: #87968f;
  transition: transform .15s ease;
}
.cat-chevron.open { transform: rotate(90deg); }
.cat-count {
  margin-left: auto;
  font-size: 11px;
  color: #9aa6a1;
  font-weight: 400;
}
.list-item {
  display: flex;
  justify-content: space-between;
  padding: 12px;
  border-bottom: 1px solid #f0f0f0;
}
.list-item.child {
  padding-left: 20px;
}
.account-option-main {
  min-width: 0;
  display: flex;
  flex-direction: column;
  gap: 3px;
}
.account-group { margin-bottom: 13px; }
.account-group-head {
  padding: 7px 4px;
  display: flex;
  justify-content: space-between;
  color: #526860;
  font-size: 12px;
  font-weight: 650;
}
.account-group-head text:last-child { color: #9aa6a1; font-size: 10px; font-weight: 400; }
.account-option { align-items: center; padding-left: 7px; }
.account-mini-icon {
  width: 31px; height: 31px; flex: 0 0 31px;
  display: flex; align-items: center; justify-content: center;
  color: #36b88f; background: #e8f7f1; border-radius: 10px;
  font-size: 10px; font-weight: 700;
}
.account-mini-icon.credit { color: #e56258; background: #fff0ee; }
.account-mini-icon.cash { color: #d98b22; background: #fff4df; }
.option-hint {
  color: #9aa6a1;
  font-size: 11px;
}
.list-item:active {
  background: #f2f7f5;
}
.balance {
  color: #87968f;
  font-size: 13px;
}
</style>
