<template>
  <view class="page">
    <WebSidebar />
    <view class="page-head">
      <view class="back-row" @click="goBack">
        <text class="back-arrow">‹</text>
        <text class="back-text">返回</text>
      </view>
      <text class="eyebrow">ACCOUNTS</text>
      <text class="page-title">账户管理</text>
      <text class="page-copy">区分资产、欠款和实际支付渠道；点账户可查看明细</text>
    </view>

    <view class="summary-grid">
      <view class="summary-card">
        <text class="summary-label">总资产</text>
        <text class="summary-value asset">¥{{ formatMoney(assetTotal) }}</text>
      </view>
      <view class="summary-card">
        <text class="summary-label">信用卡欠款</text>
        <text class="summary-value debt">¥{{ formatMoney(debtTotal) }}</text>
      </view>
      <view class="summary-card net">
        <text class="summary-label">净资产</text>
        <text class="summary-value">¥{{ formatMoney(assetTotal - debtTotal) }}</text>
      </view>
    </view>

    <view class="section" v-for="group in accountGroups" :key="group.key">
      <view class="section-head">
        <view>
          <text class="section-title">{{ group.title }}</text>
          <text class="section-copy">{{ group.copy }}</text>
        </view>
        <text class="section-count">{{ group.items.length }}</text>
      </view>
      <view
        class="account-card"
        v-for="account in group.items"
        :key="account.id"
        @click="openDetail(account)"
      >
        <view class="account-icon" :class="account.type">{{ typeIcon(account.type) }}</view>
        <view class="account-main">
          <view class="account-name-row">
            <text class="account-name">{{ account.name }}</text>
            <text class="archived" v-if="account.is_archived">已归档</text>
          </view>
          <text class="account-type">{{ typeLabel(account.type) }}</text>
          <text class="linked" v-if="account.linked_accounts?.length">关联 {{ account.linked_accounts.map(item => item.name).join('、') }}</text>
          <text class="credit-meta" v-if="account.type === 'credit' && (account.statement_day || account.due_day)">
            <text v-if="account.statement_day">账单日每月 {{ account.statement_day }} 日</text>
            <text v-if="account.statement_day && account.due_day"> · </text>
            <text v-if="account.due_day">还款日每月 {{ account.due_day }} 日</text>
          </text>
        </view>
        <view class="account-side">
          <text class="money" :class="{ debt: account.type === 'credit' }">{{ account.type === 'credit' ? '欠款' : '余额' }} ¥{{ formatMoney(account.balance) }}</text>
          <text class="edit-link" @click.stop="openEdit(account)">管理</text>
        </view>
      </view>
      <view class="empty-row" v-if="group.items.length === 0">暂无{{ group.title }}</view>
    </view>

    <view class="add-button" @click="openCreate">+ 新增账户</view>

    <view class="modal" v-if="showEditor" @tap="closeEditor">
      <view class="modal-content" @tap.stop>
        <view class="modal-head">
          <text class="modal-title">{{ editing ? '管理账户' : '新增账户' }}</text>
          <text class="modal-close" @click="closeEditor">×</text>
        </view>
        <text class="field-label">账户名称</text>
        <input class="editor-input" v-model="form.name" :placeholder="accountNamePlaceholder" maxlength="64" />

        <text class="field-label">账户类型</text>
        <view class="type-grid">
          <view
            v-for="item in types"
            :key="item.value"
            class="type-option"
            :class="{ active: form.type === item.value, disabled: editing }"
            @click="!editing && (form.type = item.value)"
          >
            <text class="type-emoji">{{ item.icon }}</text>
            <text>{{ item.label }}</text>
          </view>
        </view>

        <view class="credit-panel" v-if="form.type === 'credit'">
          <text class="field-tip credit-platform-tip">适用于银行信用卡、蚂蚁花呗、京东白条等先消费后还款账户</text>
          <text class="field-label">账单日（使用周期）</text>
          <text class="field-tip">每月几号出账，用于按账期筛选明细，建议 1–28</text>
          <input class="editor-input" type="number" v-model="form.statement_day" placeholder="如 15" maxlength="2" />
          <text class="field-label">还款日</text>
          <text class="field-tip">每月几号还款；AI 记账使用该卡且距还款不足 3 天会提醒</text>
          <input class="editor-input" type="number" v-model="form.due_day" placeholder="如 3" maxlength="2" />
        </view>

        <view class="link-panel" v-if="form.type === 'alipay' || form.type === 'wechat'">
          <text class="field-label">关联储蓄卡 / 信用卡</text>
          <text class="field-tip">关联后，记账时可选择余额或具体银行卡</text>
          <view
            v-for="candidate in linkCandidates"
            :key="candidate.id"
            class="link-row"
            :class="{ selected: form.linkedIds.includes(candidate.id) }"
            @click="toggleLink(candidate.id)"
          >
            <text class="check">{{ form.linkedIds.includes(candidate.id) ? '✓' : '' }}</text>
            <text>{{ candidate.name }}</text>
            <text class="link-type">{{ typeLabel(candidate.type) }}</text>
          </view>
          <text class="field-tip" v-if="linkCandidates.length === 0">请先创建储蓄卡或信用卡</text>
        </view>

        <view class="danger-row" v-if="editing">
          <text @click="toggleArchive">{{ editing.is_archived ? '恢复账户' : '归档账户' }}</text>
          <text class="danger" @click="removeAccount">删除账户</text>
        </view>
        <view class="modal-actions">
          <button class="btn btn-secondary" @click="closeEditor">取消</button>
          <button class="btn btn-primary" @click="save" :loading="saving">保存</button>
        </view>
      </view>
    </view>
  </view>
</template>

<script>
import { get, post, put, del } from '../../common/request.js'
import { goBackOrProfile } from '../../common/nav.js'

function emptyForm() {
  return { name: '', type: 'savings', linkedIds: [], statement_day: '', due_day: '' }
}

function parseDay(v) {
  if (v === '' || v === null || v === undefined) return null
  const n = parseInt(v, 10)
  if (!Number.isFinite(n)) return null
  return n
}

export default {
  data() {
    return {
      accounts: [],
      showEditor: false,
      editing: null,
      saving: false,
      form: emptyForm(),
      types: [
        { value: 'alipay', label: '支付宝', icon: '支' },
        { value: 'wechat', label: '微信', icon: '微' },
        { value: 'cash', label: '现金', icon: '现' },
        { value: 'savings', label: '储蓄卡', icon: '储' },
        { value: 'credit', label: '信用卡', icon: '信' }
      ]
    }
  },
  computed: {
    activeAccounts() { return this.accounts.filter(item => !item.is_archived) },
    assetTotal() {
      return this.activeAccounts.filter(item => item.type !== 'credit').reduce((sum, item) => sum + (parseFloat(item.balance) || 0), 0)
    },
    debtTotal() {
      return this.activeAccounts.filter(item => item.type === 'credit').reduce((sum, item) => sum + (parseFloat(item.balance) || 0), 0)
    },
    accountGroups() {
      return [
        { key: 'payment', title: '支付渠道', copy: '支付宝和微信，可关联银行卡', items: this.accounts.filter(item => item.type === 'alipay' || item.type === 'wechat') },
        { key: 'asset', title: '资产账户', copy: '现金与储蓄卡余额', items: this.accounts.filter(item => item.type === 'cash' || item.type === 'savings' || item.type === 'bank') },
        { key: 'debt', title: '负债账户', copy: '支持信用卡、蚂蚁花呗、京东白条等，余额表示当前欠款', items: this.accounts.filter(item => item.type === 'credit') }
      ]
    },
    accountNamePlaceholder() {
      return this.form.type === 'credit' ? '如：中信、蚂蚁花呗、京东白条' : '如：招商储蓄卡'
    },
    linkCandidates() {
      return this.accounts.filter(item => !item.is_archived && (item.type === 'savings' || item.type === 'credit'))
    }
  },
  onShow() { this.load() },
  methods: {
    goBack() { goBackOrProfile() },
    formatMoney(value) { return (parseFloat(value) || 0).toFixed(2) },
    typeLabel(type) { return ({ alipay: '支付宝', wechat: '微信', cash: '现金', savings: '储蓄卡', bank: '储蓄卡', credit: '信用卡' })[type] || '账户' },
    typeIcon(type) { return ({ alipay: '支', wechat: '微', cash: '现', savings: '储', bank: '储', credit: '信' })[type] || '账' },
    async load() {
      try { this.accounts = (await get('/accounts')) || [] } catch (error) { console.error(error) }
    },
    openDetail(account) {
      uni.navigateTo({ url: `/pages/account-detail/index?id=${account.id}` })
    },
    openCreate() {
      this.editing = null
      this.form = emptyForm()
      this.showEditor = true
      this.fixInput()
    },
    openEdit(account) {
      this.editing = account
      this.form = {
        name: account.name,
        type: account.type === 'bank' ? 'savings' : account.type,
        linkedIds: (account.linked_accounts || []).map(item => item.id),
        statement_day: account.statement_day != null ? String(account.statement_day) : '',
        due_day: account.due_day != null ? String(account.due_day) : ''
      }
      this.showEditor = true
      this.fixInput()
    },
    closeEditor() { this.showEditor = false },
    fixInput() {
      this.$nextTick(() => setTimeout(() => {
        // #ifdef H5
        document.querySelectorAll('.editor-input input').forEach(input => { input.style.height = '24px'; input.style.lineHeight = '24px' })
        // #endif
      }, 50))
    },
    toggleLink(id) {
      const index = this.form.linkedIds.indexOf(id)
      if (index >= 0) this.form.linkedIds.splice(index, 1)
      else this.form.linkedIds.push(id)
    },
    async save() {
      if (!this.form.name.trim()) return uni.showToast({ title: '请输入账户名称', icon: 'none' })
      const statementDay = parseDay(this.form.statement_day)
      const dueDay = parseDay(this.form.due_day)
      if (this.form.type === 'credit') {
        if (statementDay != null && (statementDay < 1 || statementDay > 28)) {
          return uni.showToast({ title: '账单日请填 1–28', icon: 'none' })
        }
        if (dueDay != null && (dueDay < 1 || dueDay > 28)) {
          return uni.showToast({ title: '还款日请填 1–28', icon: 'none' })
        }
      }
      this.saving = true
      try {
        const payload = {
          name: this.form.name.trim(),
          type: this.form.type,
          sort: this.editing?.sort || (this.accounts.length + 1) * 10,
          is_archived: this.editing?.is_archived || false,
          linked_account_ids: (this.form.type === 'alipay' || this.form.type === 'wechat') ? this.form.linkedIds : [],
          statement_day: this.form.type === 'credit' ? statementDay : null,
          due_day: this.form.type === 'credit' ? dueDay : null
        }
        if (this.editing) await put(`/accounts/${this.editing.id}`, payload)
        else await post('/accounts', payload)
        this.showEditor = false
        uni.showToast({ title: '已保存', icon: 'success' })
        await this.load()
      } catch (error) { uni.showToast({ title: error.message || '保存失败', icon: 'none' }) }
      finally { this.saving = false }
    },
    async toggleArchive() {
      try {
        await put(`/accounts/${this.editing.id}`, {
          name: this.editing.name,
          sort: this.editing.sort,
          is_archived: !this.editing.is_archived,
          statement_day: this.editing.statement_day ?? null,
          due_day: this.editing.due_day ?? null
        })
        this.showEditor = false
        await this.load()
      } catch (error) { uni.showToast({ title: error.message || '操作失败', icon: 'none' }) }
    },
    removeAccount() {
      uni.showModal({ title: '删除账户', content: `确定删除「${this.editing.name}」吗？有流水或关联时不能删除。`, success: async res => {
        if (!res.confirm) return
        try { await del(`/accounts/${this.editing.id}`); this.showEditor = false; await this.load() }
        catch (error) { uni.showToast({ title: error.message || '删除失败', icon: 'none' }) }
      } })
    }
  }
}
</script>

<style scoped>
.page { min-height: 100vh; padding: 22px 16px 38px; background: #f7f9f7; box-sizing: border-box; }
.page-head { padding: 0 4px 20px; }
.back-row {
  display: inline-flex;
  align-items: center;
  gap: 2px;
  margin-bottom: 10px;
  color: #36b88f;
  font-size: 14px;
  font-weight: 600;
  cursor: pointer;
}
.back-arrow { font-size: 22px; line-height: 1; }
.eyebrow { display: block; color: #36b88f; font-size: 10px; font-weight: 700; letter-spacing: 1.8px; }
.page-title { display: block; margin-top: 5px; color: #18312b; font-size: 27px; font-weight: 700; }
.page-copy { display: block; margin-top: 6px; color: #87968f; font-size: 13px; }
.summary-grid { display: grid; grid-template-columns: 1fr 1fr; gap: 10px; margin-bottom: 24px; }
.summary-card { padding: 15px; background: #fff; border: 1px solid #edf1ee; border-radius: 16px; box-shadow: 0 7px 24px rgba(24,49,43,.035); }
.summary-card.net { grid-column: 1 / -1; color: #fff; background: #42c39c; border-color: #42c39c; }
.summary-label { display: block; color: #87968f; font-size: 11px; }
.summary-card.net .summary-label { color: rgba(255,255,255,.78); }
.summary-value { display: block; margin-top: 7px; color: #18312b; font-size: 20px; font-weight: 750; font-variant-numeric: tabular-nums; }
.summary-value.asset { color: #2cae86; }.summary-value.debt { color: #f06f64; }.summary-card.net .summary-value { color: #fff; }
.section { margin-bottom: 22px; }.section-head { padding: 0 3px 9px; display: flex; justify-content: space-between; align-items: flex-end; }
.section-title { display: block; color: #263f38; font-size: 16px; font-weight: 700; }.section-copy { display: block; margin-top: 3px; color: #92a19b; font-size: 10px; }
.section-count { color: #92a19b; font-size: 12px; }
.account-card { min-height: 76px; margin-bottom: 9px; padding: 14px; display: flex; align-items: center; gap: 12px; background: #fff; border: 1px solid #edf1ee; border-radius: 17px; cursor: pointer; }
.account-icon { width: 42px; height: 42px; flex: 0 0 42px; display: flex; align-items: center; justify-content: center; border-radius: 14px; color: #36b88f; background: #e8f7f1; font-size: 13px; font-weight: 700; }
.account-icon.credit { color: #e56258; background: #fff0ee; }.account-icon.cash { color: #d98b22; background: #fff4df; }
.account-main { flex: 1; min-width: 0; }.account-name-row { display: flex; align-items: center; gap: 6px; }.account-name { color: #263f38; font-size: 14px; font-weight: 650; }
.archived { padding: 2px 6px; color: #87968f; background: #edf1ef; border-radius: 99px; font-size: 9px; }.account-type,.linked,.credit-meta { display: block; margin-top: 3px; color: #92a19b; font-size: 10px; }.linked,.credit-meta { color: #36b88f; }
.account-side { display: flex; flex-direction: column; align-items: flex-end; gap: 7px; }.money { color: #31564c; font-size: 11px; font-variant-numeric: tabular-nums; }.money.debt { color: #e56258; }.edit-link { color: #36b88f; font-size: 12px; }
.empty-row { padding: 22px; color: #a4afaa; background: #fff; border-radius: 16px; text-align: center; font-size: 12px; }
.add-button {
  width: 100%; margin-top: 8px;
  height: 50px; display: flex; align-items: center; justify-content: center;
  color: #fff; background: #42c39c; border-radius: 16px; box-shadow: 0 10px 24px rgba(66,195,156,.24);
  font-size: 15px; font-weight: 600;
}
.credit-platform-tip { margin-top: 10px; color: #52756a; line-height: 1.55; }
.modal { position: fixed; inset: 0; z-index: 999; padding: 20px; display: flex; align-items: center; justify-content: center; background: rgba(24,49,43,.34); box-sizing: border-box; }
.modal-content { width: 100%; max-width: 430px; max-height: 82vh; padding: 22px; overflow-y: auto; background: #fff; border-radius: 22px; box-sizing: border-box; }
.modal-head { display: flex; justify-content: space-between; align-items: center; margin-bottom: 18px; }.modal-title { color: #18312b; font-size: 18px; font-weight: 700; }.modal-close { color: #92a19b; font-size: 25px; }
.field-label { display: block; margin: 14px 0 7px; color: #526860; font-size: 12px; font-weight: 600; }.field-tip { display: block; margin: -2px 0 9px; color: #92a19b; font-size: 10px; }
.editor-input { width: 100%; height: 48px; padding: 0 13px; border: 1px solid #e1e9e5; border-radius: 13px; background: #fbfcfb; box-sizing: border-box; }
:deep(.editor-input .uni-input-wrapper) { height: 48px; display: flex; align-items: center; }
:deep(.editor-input input) { padding: 0 8px !important; box-sizing: border-box; }
.type-grid { display: grid; grid-template-columns: repeat(3,1fr); gap: 8px; }.type-option { padding: 10px 5px; display: flex; flex-direction: column; align-items: center; gap: 5px; color: #71817a; border: 1px solid #e1e9e5; border-radius: 12px; font-size: 11px; }.type-option.active { color: #2cae86; border-color: #42c39c; background: #effaf6; }.type-option.disabled { opacity: .7; }.type-emoji { font-size: 15px; font-weight: 700; }
.credit-panel,.link-panel { margin-top: 14px; padding: 2px 13px 10px; background: #f7faf8; border-radius: 14px; }
.link-row { min-height: 42px; display: flex; align-items: center; gap: 9px; color: #526860; border-bottom: 1px solid #e8eeeb; font-size: 12px; }.check { width: 19px; height: 19px; display: flex; align-items: center; justify-content: center; color: #fff; background: #dce6e2; border-radius: 6px; }.link-row.selected .check { background: #42c39c; }.link-type { margin-left: auto; color: #92a19b; }
.danger-row { margin-top: 17px; display: flex; justify-content: space-between; color: #71817a; font-size: 12px; }.danger-row .danger { color: #f06f64; }.modal-actions { margin-top: 18px; display: flex; gap: 10px; }.modal-actions .btn { flex: 1; width: auto !important; margin: 0; }
@media (min-width: 900px) { .page { max-width: 760px !important; }.summary-grid { grid-template-columns: repeat(3,1fr); }.summary-card.net { grid-column: auto; } }
</style>
