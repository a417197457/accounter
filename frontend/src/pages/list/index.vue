<template>
  <view class="page">
    <WebSidebar />
    <view class="page-head">
      <text class="page-title">收支明细</text>
      <text class="page-copy">每一笔都清楚，每个月都安心</text>
    </view>
    <!-- 账本切换 -->
    <view class="ledger-tabs">
      <view
        class="tab"
        :class="{ active: currentLedger?.id === ledger.id }"
        v-for="ledger in ledgers"
        :key="ledger.id"
        @click="switchLedger(ledger)"
      >
        <text>{{ ledger.is_personal ? '个人' : ledger.name }}</text>
      </view>
    </view>

    <view v-if="isFamily" class="hint">家庭账本只读，数据来自成员同步</view>

    <view v-if="groupedTransactions.length === 0" class="empty">
      <text class="empty-title">暂无流水</text>
      <text class="empty-tip" v-if="!isFamily">用 AI 说话记账，或手动记一笔</text>
      <text class="empty-tip" v-else>成员从个人账本同步后，账单会出现在这里</text>
      <view class="empty-actions" v-if="!isFamily">
        <button class="btn btn-primary" @click="goChat">AI 记账</button>
        <button class="btn btn-secondary" @click="goRecord">记一笔</button>
      </view>
    </view>

    <view v-else class="month-groups">
      <view v-for="group in groupedTransactions" :key="group.month" class="month-group">
        <view class="month-header" @click="toggleMonth(group)">
          <view class="month-left">
            <text class="month-title">{{ group.month }}</text>
            <text class="month-summary">
              {{ group.count }} 笔 · 支出 {{ group.expenseTotal }} · 收入 {{ group.incomeTotal }}
            </text>
          </view>
          <view class="month-actions">
            <text
              v-if="!isFamily && familyLedgers.length > 0"
              class="sync-month"
              @click.stop="pushMonth(group.month)"
            >⇪ 同步</text>
            <text class="month-chevron" :class="{ open: group.expanded }">⌄</text>
          </view>
        </view>

        <view class="transaction-list" v-if="group.expanded">
          <view class="month-loading" v-if="group.loading">正在加载明细…</view>
          <view class="month-empty" v-else-if="group.loaded && !(group.transactions || []).length">
            该月暂无明细
          </view>
          <view class="date-group" v-for="dateGroup in transactionDateGroups(group.transactions || [])" :key="dateGroup.date">
            <text class="date-label">{{ dateGroup.label }}</text>
            <view
              class="transaction-item"
              v-for="tx in dateGroup.transactions"
              :key="tx.id"
              @longpress="onLongPress(tx)"
            >
              <view class="tx-icon" :class="tx.op_type">{{ categoryIcon(tx) }}</view>
              <view class="tx-main">
                <view class="tx-title-row">
                  <text class="tx-category">{{ transactionTitle(tx) }}</text>
                  <text class="sync-badge" v-if="tx.source_id">同步</text>
                </view>
                <text class="tx-remark" v-if="tx.remark">{{ tx.remark }}</text>
                <text class="tx-meta">
                  {{ accountRoute(tx) }}<text v-if="formatDateTime(tx.occurred_at)"> · {{ formatDateTime(tx.occurred_at) }}</text>
                  <text v-if="isFamily && tx.member"> · {{ tx.member.nickname || tx.member.username }}</text>
                </text>
              </view>
              <view class="tx-side">
                <text class="tx-amount" :class="tx.op_type">{{ formatAmount(tx) }}</text>
                <text class="tx-more" v-if="!isFamily" @click.stop="showTxActions(tx)">•••</text>
              </view>
            </view>
          </view>
        </view>
      </view>
    </view>

    <!-- 推送到家庭 -->
    <view class="modal" v-if="showPush" @tap="showPush = false">
      <view class="modal-content" @tap.stop>
        <text class="modal-title">推送到家庭账本</text>
        <view
          class="list-item"
          v-for="fl in familyLedgers"
          :key="fl.id"
          @click="doPush(fl)"
        >
          <text>{{ fl.name }}</text>
        </view>
        <button class="btn btn-secondary" @click="showPush = false">取消</button>
      </view>
    </view>
  </view>
</template>

<script>
import { get, post, del } from '../../common/request.js'
import { checkLogin } from '../../common/auth.js'
import { paymentRouteLabel } from '../../common/account.js'

export default {
  data() {
    return {
      ledgers: [],
      currentLedger: null,
      groupedTransactions: [],
      showPush: false,
      pushTargetTx: null,
      pushMonthValue: ''
    }
  },
  computed: {
    isFamily() {
      return this.currentLedger && !this.currentLedger.is_personal
    },
    familyLedgers() {
      return (this.ledgers || []).filter(l => !l.is_personal)
    }
  },
  onLoad() {
    checkLogin()
  },
  onShow() {
    this.loadLedgers()
  },
  methods: {
    async loadLedgers() {
      try {
        const ledgers = await get('/ledgers')
        this.ledgers = ledgers || []
        if (this.ledgers.length === 0) return
        const keepId = this.currentLedger?.id
        this.currentLedger =
          this.ledgers.find(l => l.id === keepId) ||
          this.ledgers.find(l => l.is_personal) ||
          this.ledgers[0]
        await this.loadTransactions()
      } catch (err) {
        if (err.message && err.message.includes('未登录')) {
          uni.redirectTo({ url: '/pages/login/index' })
        }
      }
    },
    async switchLedger(ledger) {
      this.currentLedger = ledger
      await this.loadMonths()
    },
    async loadTransactions() {
      return this.loadMonths()
    },
    currentMonth() {
      const now = new Date()
      return `${now.getFullYear()}-${String(now.getMonth() + 1).padStart(2, '0')}`
    },
    async loadMonths() {
      if (!this.currentLedger) return
      try {
        const months = await get('/transaction-months', {}, { ledgerId: this.currentLedger.id })
        const current = this.currentMonth()
        const list = months || []
        const expandMonth = list.find(m => m.month === current)?.month || list[0]?.month
        this.groupedTransactions = list.map(month => ({
          month: month.month,
          expenseTotal: this.formatMoney(month.expense),
          incomeTotal: this.formatMoney(month.income),
          count: month.count,
          transactions: [],
          loaded: false,
          loading: false,
          expanded: month.month === expandMonth
        }))
        const openGroup = this.groupedTransactions.find(group => group.expanded)
        if (openGroup) await this.loadMonth(openGroup)
      } catch (err) {
        console.error('加载流水失败:', err)
      }
    },
    formatMoney(value) {
      return (parseFloat(value) || 0).toFixed(2)
    },
    async toggleMonth(group) {
      const next = !group.expanded
      const idx = this.groupedTransactions.findIndex(g => g.month === group.month)
      if (idx < 0) return
      // 替换对象，确保折叠/展开能刷新明细区
      this.groupedTransactions.splice(idx, 1, { ...this.groupedTransactions[idx], expanded: next })
      const updated = this.groupedTransactions[idx]
      if (updated.expanded && !updated.loaded) await this.loadMonth(updated)
    },
    async loadMonth(group) {
      const idx = this.groupedTransactions.findIndex(g => g.month === group.month)
      if (idx < 0) return
      this.groupedTransactions.splice(idx, 1, {
        ...this.groupedTransactions[idx],
        loading: true
      })
      try {
        // 用 month=YYYY-MM 与月份摘要同口径，避免 start/end 时区导致空列表
        const result = await get('/transactions', {
          month: group.month,
          page: 1,
          page_size: 100
        }, { ledgerId: this.currentLedger.id })
        const list = result?.list || []
        this.groupedTransactions.splice(idx, 1, {
          ...this.groupedTransactions[idx],
          transactions: list,
          loaded: true,
          loading: false,
          expanded: true
        })
      } catch (err) {
        console.error('加载月明细失败:', err)
        this.groupedTransactions.splice(idx, 1, {
          ...this.groupedTransactions[idx],
          loading: false,
          loaded: false,
          expanded: true
        })
        uni.showToast({ title: err.message || '加载明细失败', icon: 'none' })
      }
    },
    categoryIcon(tx) {
      if (tx.op_type === 'transfer') return tx.to_account?.type === 'credit' ? '✓' : '↔'
      return tx.category?.icon || tx.category?.parent?.icon || '·'
    },
    transactionTitle(tx) {
      if (tx.op_type === 'transfer') return tx.to_account?.type === 'credit' ? '信用卡还款' : '账户转账'
      return tx.category?.name || '未分类'
    },
    accountRoute(tx) {
      if (tx.op_type === 'transfer') return `${tx.account?.name || '账户'} → ${tx.to_account?.name || '账户'}`
      return paymentRouteLabel(tx)
    },
    formatAmount(tx) {
      const amount = parseFloat(tx.amount).toFixed(2)
      if (tx.op_type === 'transfer') return `¥${amount}`
      if (tx.op_type === 'expense' || tx.op_type === 'borrow_out') {
        return `-${amount}`
      }
      return `+${amount}`
    },
    formatDateTime(dateStr) {
      const value = String(dateStr || '')
      if (value.length < 10) return ''
      const month = Number(value.slice(5, 7))
      const day = Number(value.slice(8, 10))
      if (!month || !day) return ''
      const time = value.length >= 16 ? value.slice(11, 16) : ''
      return `${month}月${day}日${time ? ` ${time}` : ''}`
    },
    transactionDateGroups(list) {
      const result = []
      const map = {}
      const now = new Date()
      const today = `${now.getFullYear()}-${String(now.getMonth() + 1).padStart(2, '0')}-${String(now.getDate()).padStart(2, '0')}`
      const yesterdayDate = new Date(now.getFullYear(), now.getMonth(), now.getDate() - 1)
      const yesterday = `${yesterdayDate.getFullYear()}-${String(yesterdayDate.getMonth() + 1).padStart(2, '0')}-${String(yesterdayDate.getDate()).padStart(2, '0')}`
      for (const tx of list) {
        const date = (tx.occurred_at || '').slice(0, 10)
        if (!map[date]) {
          const label = date === today ? '今天' : (date === yesterday ? '昨天' : `${parseInt(date.slice(5, 7), 10)}月${parseInt(date.slice(8, 10), 10)}日`)
          map[date] = { date, label, transactions: [] }
          result.push(map[date])
        }
        map[date].transactions.push(tx)
      }
      return result
    },
    onLongPress(tx) {
      if (this.isFamily) return
      this.showTxActions(tx)
    },
    onTap(tx) {
      // H5 长按不便时，个人账本点击也可操作
      if (this.isFamily) return
      // #ifdef H5
      this.showTxActions(tx)
      // #endif
    },
    showTxActions(tx) {
      const itemList = ['删除']
      if (this.familyLedgers.length > 0 && (tx.op_type === 'expense' || tx.op_type === 'income')) {
        itemList.unshift('推送到家庭')
      }
      uni.showActionSheet({
        itemList,
        success: (res) => {
          const action = itemList[res.tapIndex]
          if (action === '推送到家庭') {
            this.pushTargetTx = tx
            this.pushMonthValue = ''
            if (this.familyLedgers.length === 1) {
              this.doPush(this.familyLedgers[0])
            } else {
              this.showPush = true
            }
          } else if (action === '删除') {
            this.deleteTx(tx)
          }
        }
      })
    },
    async deleteTx(tx) {
      uni.showModal({
        title: '确认删除',
        content: '删除后家庭账本中的同步副本也会一并删除',
        success: async (res) => {
          if (!res.confirm) return
          try {
            await del(`/transactions/${tx.id}`, {}, { ledgerId: this.currentLedger.id })
            uni.showToast({ title: '已删除', icon: 'success' })
            this.loadTransactions()
          } catch (err) {
            uni.showToast({ title: err.message || '删除失败', icon: 'none' })
          }
        }
      })
    },
    pushMonth(month) {
      if (this.familyLedgers.length === 0) {
        uni.showToast({ title: '请先创建或加入家庭账本', icon: 'none' })
        return
      }
      this.pushTargetTx = null
      this.pushMonthValue = month
      if (this.familyLedgers.length === 1) {
        this.doPush(this.familyLedgers[0])
      } else {
        this.showPush = true
      }
    },
    async doPush(familyLedger) {
      this.showPush = false
      try {
        if (this.pushMonthValue) {
          const result = await post('/push-month', {
            month: this.pushMonthValue,
            target_ledger_id: familyLedger.id
          }, { ledgerId: this.currentLedger.id })
          const pushed = result?.pushed ?? 0
          const skipped = result?.skipped ?? 0
          uni.showToast({
            title: pushed === 0
              ? (skipped > 0 ? '本月已全部同步过' : '本月没有可同步流水')
              : `已同步 ${pushed} 笔到「${familyLedger.name}」`,
            icon: 'none',
            duration: 2500
          })
        } else if (this.pushTargetTx) {
          await post(`/transactions/${this.pushTargetTx.id}/push`, {
            target_ledger_id: familyLedger.id
          }, { ledgerId: this.currentLedger.id })
          uni.showToast({ title: `已同步到「${familyLedger.name}」`, icon: 'success' })
        }
      } catch (err) {
        uni.showToast({ title: err.message || '同步失败', icon: 'none' })
      } finally {
        this.pushTargetTx = null
        this.pushMonthValue = ''
      }
    },
    goChat() {
      uni.switchTab({ url: '/pages/chat/index' })
    },
    goRecord() {
      uni.switchTab({ url: '/pages/record/index' })
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
.page-head { padding: 0 4px 18px; }
.page-title { display: block; color: #18312b; font-size: 27px; font-weight: 700; }
.page-copy { display: block; margin-top: 6px; color: #87968f; font-size: 13px; }
.ledger-tabs::-webkit-scrollbar { display: none; }
.ledger-tabs {
  scrollbar-width: none;
}
.ledger-tabs {
  display: flex;
  gap: 8px;
  margin-bottom: 14px;
  overflow-x: auto;
}
.tab {
  padding: 9px 16px;
  border-radius: 999px;
  background: #fff;
  border: 1px solid #e5ebe8;
  color: #71817a;
  font-size: 14px;
  white-space: nowrap;
}
.tab.active {
  background: #42c39c;
  border-color: #42c39c;
  color: #fff;
  box-shadow: 0 7px 16px rgba(66, 195, 156, 0.18);
}
.hint {
  font-size: 12px;
  color: #71817a;
  background: #eaf7f2;
  border-radius: 12px;
  padding: 10px 12px;
  margin-bottom: 14px;
}
.empty {
  text-align: center;
  padding: 64px 20px;
  color: #87968f;
  background: #fff;
  border: 1px solid #edf1ee;
  border-radius: 20px;
}
.empty-title {
  font-size: 18px;
  color: #31564c;
  font-weight: 600;
  display: block;
  margin-bottom: 8px;
}
.empty-tip {
  font-size: 13px;
  color: #bbb;
  display: block;
  margin-bottom: 20px;
}
.empty-actions {
  display: flex;
  align-items: center;
  gap: 12px;
  justify-content: center;
}
.empty-actions .btn {
  width: auto !important;
  min-width: 100px;
  min-height: 40px;
  margin: 0;
  padding: 10px 16px;
  font-size: 14px;
  display: flex;
  align-items: center;
  justify-content: center;
  line-height: 1.4 !important;
}
.month-groups {
  display: flex;
  flex-direction: column;
  gap: 16px;
}
.month-group {
  background: #fff;
  border: 1px solid #edf1ee;
  border-radius: 18px;
  overflow: hidden;
  box-shadow: 0 8px 28px rgba(24, 49, 43, 0.04);
}
.month-header {
  padding: 15px 17px;
  background: #fbfcfb;
  border-bottom: 1px solid #edf1ee;
  display: flex;
  justify-content: space-between;
  align-items: center;
}
.month-left {
  display: flex;
  flex-direction: column;
  gap: 4px;
}
.month-title {
  color: #18312b;
  font-size: 16px;
  font-weight: 700;
}
.month-summary {
  font-size: 12px;
  color: #87968f;
}
.sync-month {
  font-size: 13px;
  color: #36b88f;
  background: #eaf7f2;
  border-radius: 999px;
  padding: 6px 10px;
}
.month-actions { display: flex; align-items: center; }
.month-chevron {
  margin-left: 8px;
  color: #87968f;
  font-size: 20px;
  transition: transform .2s ease;
}
.month-chevron.open { transform: rotate(180deg); }
.month-loading,
.month-empty {
  padding: 18px 0;
  color: #87968f;
  font-size: 13px;
  text-align: center;
}
.transaction-list {
  padding: 0 16px;
}
.date-group { padding-top: 11px; }
.date-group + .date-group { border-top: 1px solid #edf1ee; }
.date-label {
  display: block;
  padding: 2px 0 3px;
  color: #92a19b;
  font-size: 11px;
  font-weight: 600;
}
.transaction-item {
  display: flex;
  align-items: center;
  gap: 11px;
  padding: 15px 0;
  border-bottom: 1px solid #edf1ee;
}
.tx-icon {
  width: 38px; height: 38px; flex: 0 0 38px;
  display: flex; align-items: center; justify-content: center;
  color: #31564c; background: #eef8f4; border-radius: 13px;
  font-size: 18px;
}
.tx-icon.transfer { color: #71817a; background: #f0f3f2; font-size: 15px; }
.transaction-item:last-child {
  border-bottom: none;
}
.tx-main {
  flex: 1;
  min-width: 0;
}
.tx-title-row {
  display: flex;
  align-items: center;
  gap: 6px;
  margin-bottom: 4px;
}
.tx-category {
  font-size: 15px;
  color: #263f38;
  font-weight: 600;
}
.sync-badge {
  font-size: 10px;
  padding: 1px 6px;
  border-radius: 999px;
  background: #eaf7f2;
  color: #36b88f;
}
.tx-remark {
  font-size: 13px;
  color: #71817a;
  margin-bottom: 4px;
  display: block;
}
.tx-meta {
  font-size: 12px;
  color: #9aa6a1;
}
.tx-amount {
  font-size: 17px;
  font-weight: 700;
  font-variant-numeric: tabular-nums;
}
.tx-side { flex-shrink: 0; display: flex; flex-direction: column; align-items: flex-end; gap: 5px; }
.tx-more { color: #a3afaa; font-size: 11px; letter-spacing: 1px; padding: 2px 0 2px 10px; }
.tx-amount.transfer {
  color: #71817a;
}
.tx-amount.expense,
.tx-amount.borrow_out {
  color: #f06f64;
}
.tx-amount.income,
.tx-amount.borrow_in {
  color: #36b88f;
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
.modal-title {
  font-size: 18px;
  font-weight: bold;
  margin-bottom: 16px;
  display: block;
  text-align: center;
}
.list-item {
  padding: 14px 0;
  border-bottom: 1px solid #f0f0f0;
  font-size: 15px;
}
.list-item:active {
  background: #f2f7f5;
}
</style>
