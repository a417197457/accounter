<template>
  <view class="page">
    <WebSidebar />
    <view class="page-head">
      <view class="back-row" @click="goBack">
        <text class="back-arrow">‹</text>
        <text class="back-text">返回</text>
      </view>
      <text class="page-title">{{ account?.name || '账户明细' }}</text>
      <text class="page-copy" v-if="account">
        {{ typeLabel(account.type) }} · {{ account.type === 'credit' ? '欠款' : '余额' }} ¥{{ formatMoney(account.balance) }}
      </text>
      <text class="page-copy" v-if="account?.type === 'credit' && account.statement_day">
        账单日每月 {{ account.statement_day }} 日
        <text v-if="account.due_day"> · 还款日每月 {{ account.due_day }} 日</text>
      </text>
    </view>

    <view class="filter-tabs">
      <text
        v-for="tab in filterTabs"
        :key="tab.key"
        class="ftab"
        :class="{ active: filterMode === tab.key }"
        @click="setFilterMode(tab.key)"
      >{{ tab.label }}</text>
    </view>

    <view class="custom-range" v-if="filterMode === 'custom'">
      <picker mode="date" :value="customStart" @change="onStartChange">
        <view class="date-chip">起 {{ customStart }}</view>
      </picker>
      <text class="range-sep">至</text>
      <picker mode="date" :value="customEnd" @change="onEndChange">
        <view class="date-chip">止 {{ customEnd }}</view>
      </picker>
    </view>
    <view class="range-tip" v-else-if="rangeLabel">{{ rangeLabel }}</view>

    <view class="list" v-if="transactions.length">
      <view class="tx-item" v-for="tx in transactions" :key="tx.id">
        <view class="tx-main">
          <text class="tx-title">{{ txTitle(tx) }}</text>
          <text class="tx-meta">{{ formatDate(tx.occurred_at) }} · {{ accountRoute(tx) }}</text>
          <text class="tx-remark" v-if="tx.remark">{{ tx.remark }}</text>
        </view>
        <text class="tx-amount" :class="tx.op_type">{{ formatAmount(tx) }}</text>
      </view>
    </view>
    <view class="empty" v-else-if="!loading">该时间范围暂无流水</view>
    <view class="empty" v-else>加载中…</view>
  </view>
</template>

<script>
import { get } from '../../common/request.js'
import { goBackOrProfile } from '../../common/nav.js'
import { billingCycleRange, formatDateYMD } from '../../common/credit.js'
import { paymentRouteLabel } from '../../common/account.js'

export default {
  data() {
    return {
      accountId: 0,
      account: null,
      transactions: [],
      loading: false,
      filterMode: 'month', // month | cycle | last_cycle | custom
      filterReady: false,
      customStart: '',
      customEnd: ''
    }
  },
  computed: {
    filterTabs() {
      const tabs = [
        { key: 'month', label: '本月' },
        { key: 'custom', label: '自定义' }
      ]
      if (this.account?.type === 'credit' && this.account.statement_day) {
        tabs.splice(1, 0,
          { key: 'cycle', label: '本账期' },
          { key: 'last_cycle', label: '上账期' }
        )
      }
      return tabs
    },
    rangeLabel() {
      const { start, end } = this.resolveRange()
      if (!start || !end) return ''
      return `${formatDateYMD(start)} ~ ${formatDateYMD(end)}`
    }
  },
  onLoad(query) {
    this.accountId = parseInt(query.id || '0', 10)
    const now = new Date()
    this.customStart = formatDateYMD(new Date(now.getFullYear(), now.getMonth(), 1))
    this.customEnd = formatDateYMD(now)
  },
  onShow() {
    this.bootstrap()
  },
  methods: {
    goBack() { goBackOrProfile() },
    formatMoney(v) { return (parseFloat(v) || 0).toFixed(2) },
    typeLabel(type) {
      return ({ alipay: '支付宝', wechat: '微信', cash: '现金', savings: '储蓄卡', bank: '储蓄卡', credit: '信用卡' })[type] || '账户'
    },
    async bootstrap() {
      if (!this.accountId) return
      try {
        const list = (await get('/accounts')) || []
        this.account = list.find(a => a.id === this.accountId) || null
        if (!this.account) {
          uni.showToast({ title: '账户不存在', icon: 'none' })
          return
        }
        if (!this.filterReady) {
          if (this.account.type === 'credit' && this.account.statement_day) {
            this.filterMode = 'cycle'
          }
          this.filterReady = true
        }
        await this.loadTx()
      } catch (e) {
        uni.showToast({ title: e.message || '加载失败', icon: 'none' })
      }
    },
    setFilterMode(mode) {
      this.filterMode = mode
      this.loadTx()
    },
    onStartChange(e) {
      this.customStart = e.detail.value
      this.loadTx()
    },
    onEndChange(e) {
      this.customEnd = e.detail.value
      this.loadTx()
    },
    resolveRange() {
      const now = new Date()
      if (this.filterMode === 'cycle' && this.account?.statement_day) {
        return billingCycleRange(this.account.statement_day, now, 0)
      }
      if (this.filterMode === 'last_cycle' && this.account?.statement_day) {
        return billingCycleRange(this.account.statement_day, now, -1)
      }
      if (this.filterMode === 'custom') {
        const start = new Date(this.customStart + 'T00:00:00')
        const end = new Date(this.customEnd + 'T23:59:59')
        return { start, end }
      }
      // month
      const start = new Date(now.getFullYear(), now.getMonth(), 1, 0, 0, 0)
      const end = new Date(now.getFullYear(), now.getMonth() + 1, 0, 23, 59, 59)
      return { start, end }
    },
    async loadTx() {
      if (!this.accountId) return
      this.loading = true
      try {
        const { start, end } = this.resolveRange()
        const result = await get('/transactions', {
          account_id: this.accountId,
          start: formatDateYMD(start),
          end: formatDateYMD(end),
          page: 1,
          page_size: 100
        })
        // request 可能直接返回数组或 {items,total}
        this.transactions = result?.list || []
      } catch (e) {
        this.transactions = []
        uni.showToast({ title: e.message || '加载失败', icon: 'none' })
      } finally {
        this.loading = false
      }
    },
    txTitle(tx) {
      if (tx.op_type === 'transfer') return '转账/还款'
      const cat = tx.category
      if (!cat) return '未分类'
      if (cat.parent?.name) return `${cat.parent.name}/${cat.name}`
      return cat.name
    },
    accountRoute(tx) {
      const from = paymentRouteLabel(tx, '账户')
      if (tx.op_type === 'transfer' && tx.to_account) return `${from} → ${tx.to_account.name}`
      return from
    },
    formatDate(v) {
      if (!v) return ''
      return String(v).slice(0, 10)
    },
    formatAmount(tx) {
      const n = parseFloat(tx.amount) || 0
      const sign = tx.op_type === 'income' || tx.op_type === 'borrow_in' ? '+' : '-'
      if (tx.op_type === 'transfer') return `¥${n.toFixed(2)}`
      return `${sign}¥${n.toFixed(2)}`
    }
  }
}
</script>

<style scoped>
.page { min-height: 100vh; padding: 22px 16px 40px; background: #f7f9f7; box-sizing: border-box; }
.page-head { padding: 0 4px 16px; }
.back-row {
  display: inline-flex; align-items: center; gap: 2px; margin-bottom: 10px;
  color: #36b88f; font-size: 14px; font-weight: 600; cursor: pointer;
}
.back-arrow { font-size: 22px; line-height: 1; }
.page-title { display: block; color: #18312b; font-size: 24px; font-weight: 700; }
.page-copy { display: block; margin-top: 6px; color: #87968f; font-size: 13px; }
.filter-tabs {
  display: flex; flex-wrap: wrap; gap: 8px; margin-bottom: 12px;
}
.ftab {
  padding: 8px 14px; border-radius: 999px; background: #fff; border: 1px solid #e5ebe8;
  color: #71817a; font-size: 13px; font-weight: 600;
}
.ftab.active { background: #42c39c; border-color: #42c39c; color: #fff; }
.custom-range {
  display: flex; align-items: center; gap: 8px; margin-bottom: 14px;
}
.date-chip {
  padding: 8px 12px; background: #fff; border: 1px solid #e1e9e5; border-radius: 12px;
  color: #31564c; font-size: 13px;
}
.range-sep { color: #9aa6a1; font-size: 12px; }
.range-tip { margin-bottom: 14px; color: #87968f; font-size: 12px; padding: 0 4px; }
.list { display: flex; flex-direction: column; gap: 8px; }
.tx-item {
  display: flex; align-items: center; gap: 12px; padding: 14px;
  background: #fff; border: 1px solid #edf1ee; border-radius: 16px;
}
.tx-main { flex: 1; min-width: 0; }
.tx-title { display: block; color: #263f38; font-size: 14px; font-weight: 600; }
.tx-meta,.tx-remark { display: block; margin-top: 3px; color: #92a19b; font-size: 11px; }
.tx-amount { font-size: 15px; font-weight: 700; color: #31564c; flex-shrink: 0; }
.tx-amount.expense,.tx-amount.borrow_out { color: #f06f64; }
.tx-amount.income,.tx-amount.borrow_in { color: #36b88f; }
.empty { padding: 48px 20px; text-align: center; color: #9aa6a1; font-size: 13px; }
@media (min-width: 900px) { .page { max-width: 760px !important; } }
</style>
