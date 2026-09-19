<template>
  <view class="page" :class="{ 'selection-active': selectionMode }">
    <WebSidebar />
    <view class="page-head">
      <view>
        <text class="page-title">收支明细</text>
        <text class="page-copy">每一笔都清楚，每个月都安心</text>
      </view>
      <text class="batch-entry" v-if="loggedIn" @click="toggleSelectionMode">{{ selectionMode ? '取消' : '批量管理' }}</text>
    </view>
    <view class="current-ledger" @click="goLedgerManagement">
      <view class="current-ledger-icon">{{ ledgerTypeIcon(currentLedger) }}</view>
      <view class="current-ledger-main">
        <text class="current-ledger-label">当前账本</text>
        <text class="current-ledger-name">{{ currentLedger?.name || (loggedIn ? '正在加载' : '游客预览') }}</text>
      </view>
      <text class="current-ledger-switch">切换账本 ›</text>
    </view>

    <view v-if="isSharedLedger" class="hint">{{ ledgerTypeLabel(currentLedger) }}只展示同步流水，可在这里单独移除副本</view>
    <view v-if="selectionMode" class="selection-hint">点击流水进行多选，单次最多操作 100 笔</view>

    <view v-if="groupedTransactions.length === 0" class="empty">
      <text class="empty-title">{{ loggedIn ? '暂无流水' : '登录后查看个人明细' }}</text>
      <text class="empty-tip" v-if="!loggedIn">你的账单只对登录后的本人可见，现在可以继续浏览功能。</text>
      <text class="empty-tip" v-else-if="!isSharedLedger">用 AI 说话记账，或手动记一笔</text>
      <text class="empty-tip" v-else>成员从个人账本同步后，账单会出现在这里</text>
      <view class="empty-actions" v-if="!loggedIn">
        <button class="btn btn-primary" @click="requestLogin()">登录后查看</button>
        <button class="btn btn-secondary" @click="goRecord">先体验记一笔</button>
      </view>
      <view class="empty-actions" v-else-if="!isSharedLedger">
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
              v-if="selectionMode"
              class="sync-month"
              @click.stop="toggleMonthSelection(group)"
            >{{ isMonthSelected(group) ? '取消本月' : '选择本月' }}</text>
            <text
              v-else-if="!isSharedLedger && targetLedgers.length > 0"
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
              :class="{ selected: isSelected(tx), disabled: selectionMode && !canSelectTransaction(tx) }"
              v-for="tx in dateGroup.transactions"
              :key="tx.id"
              @longpress="onLongPress(tx)"
              @click="onTap(tx)"
            >
              <view class="selection-check" v-if="selectionMode" :class="{ checked: isSelected(tx) }">
                {{ isSelected(tx) ? '✓' : '' }}
              </view>
              <view class="tx-icon" :class="tx.op_type">{{ categoryIcon(tx) }}</view>
              <view class="tx-main">
                <view class="tx-title-row">
                  <text class="tx-category">{{ transactionTitle(tx) }}</text>
                  <text class="sync-badge" v-if="tx.source_id">同步</text>
                </view>
                <text class="tx-remark" v-if="tx.remark">{{ tx.remark }}</text>
                <text class="tx-meta">
                  {{ accountRoute(tx) }}<text v-if="formatDateTime(tx.occurred_at)"> · {{ formatDateTime(tx.occurred_at) }}</text>
                  <text v-if="isSharedLedger && tx.member"> · {{ tx.member.nickname || tx.member.username }}</text>
                </text>
              </view>
              <view class="tx-side">
                <text class="tx-amount" :class="tx.op_type">{{ formatAmount(tx) }}</text>
                <text class="tx-more" v-if="!selectionMode && (!isSharedLedger || tx.source_id)" @click.stop="showTxActions(tx)">•••</text>
              </view>
            </view>
          </view>
        </view>
      </view>
    </view>

    <view class="batch-bar" v-if="selectionMode">
      <view class="batch-count">
        <text>已选 {{ selectedCount }} 笔</text>
        <text class="batch-clear" @click="clearSelection">清空</text>
      </view>
      <button
        v-if="!isSharedLedger"
        class="batch-button sync"
        :disabled="selectedCount === 0 || targetLedgers.length === 0 || batchLoading"
        @click="startBatchPush"
      >同步</button>
      <button
        class="batch-button delete"
        :disabled="selectedCount === 0 || batchLoading"
        @click="batchDelete"
      >删除</button>
    </view>

    <!-- 选择同步目标账本 -->
    <view class="modal" v-if="showPush" @tap="showPush = false">
      <view class="modal-content" @tap.stop>
        <text class="modal-title">{{ pushTransactionIDs.length ? `同步已选 ${pushTransactionIDs.length} 笔` : '选择同步到哪个账本' }}</text>
        <view
          class="list-item"
          v-for="ledger in targetLedgers"
          :key="ledger.id"
          @click="doPush(ledger)"
        >
          <text>{{ ledger.name }} · {{ ledgerTypeLabel(ledger) }}</text>
        </view>
        <button class="btn btn-secondary" @click="showPush = false">取消</button>
      </view>
    </view>
  </view>
</template>

<script>
import { get, post, del } from '../../common/request.js'
import { isLoggedIn, requireLogin } from '../../common/auth.js'
import { paymentRouteLabel } from '../../common/account.js'

const OPEN_LEDGER_KEY = 'open_ledger_id'

export default {
  data() {
    return {
      ledgers: [],
      currentLedger: null,
      groupedTransactions: [],
      showPush: false,
      pushTargetTx: null,
      pushMonthValue: '',
      pushTransactionIDs: [],
      selectionMode: false,
      selectedMap: {},
      loggedIn: false,
      batchLoading: false
    }
  },
  computed: {
    isSharedLedger() {
      return this.currentLedger && !this.currentLedger.is_personal
    },
    targetLedgers() {
      return (this.ledgers || []).filter(l => !l.is_personal)
    },
    selectedIDs() {
      return Object.keys(this.selectedMap).filter(id => this.selectedMap[id]).map(id => Number(id))
    },
    selectedCount() {
      return this.selectedIDs.length
    }
  },
  onLoad() {},
  onShow() {
    this.loggedIn = isLoggedIn()
    if (this.loggedIn) this.loadLedgers()
    else {
      this.ledgers = []
      this.currentLedger = null
      this.groupedTransactions = []
      this.cancelSelection()
    }
  },
  methods: {
    requestLogin() {
      requireLogin('登录后即可查看仅属于你的收支明细')
    },
    async loadLedgers() {
      try {
        const ledgers = await get('/ledgers')
        this.ledgers = ledgers || []
        if (this.ledgers.length === 0) return
        const requestedID = Number(uni.getStorageSync(OPEN_LEDGER_KEY) || 0)
        if (requestedID) uni.removeStorageSync(OPEN_LEDGER_KEY)
        const keepId = this.currentLedger?.id
        this.currentLedger =
          this.ledgers.find(l => l.id === requestedID) ||
          this.ledgers.find(l => l.id === keepId) ||
          this.ledgers.find(l => l.is_personal) ||
          this.ledgers[0]
        await this.loadTransactions()
      } catch (err) {
        if (err?.code !== 'UNAUTHORIZED') console.error('加载账本失败:', err)
      }
    },
    async switchLedger(ledger) {
      this.currentLedger = ledger
      this.cancelSelection()
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
      this.cancelSelection()
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
    ledgerTypeLabel(ledger) {
      const labels = { personal: '个人账本', family: '家庭账本', travel: '旅游账本', gift: '礼账账本' }
      return labels[ledger?.type] || (ledger?.is_personal ? '个人账本' : '家庭账本')
    },
    ledgerTypeIcon(ledger) {
      return ({ personal: '账', family: '🏠', travel: '✈️', gift: '🧧' })[ledger?.type] || '账'
    },
    goLedgerManagement() {
      if (!this.loggedIn) return requireLogin('登录后即可查看和管理个人账本')
      uni.navigateTo({ url: '/pages/ledger/index' })
    },
    toggleSelectionMode() {
      if (this.selectionMode) {
        this.cancelSelection()
      } else {
        this.selectionMode = true
        this.selectedMap = {}
      }
    },
    cancelSelection() {
      this.selectionMode = false
      this.selectedMap = {}
      this.pushTransactionIDs = []
    },
    clearSelection() {
      this.selectedMap = {}
    },
    canSelectTransaction(tx) {
      return !this.isSharedLedger || Boolean(tx.source_id)
    },
    isSelected(tx) {
      return Boolean(this.selectedMap[tx.id])
    },
    toggleTransaction(tx) {
      if (!this.canSelectTransaction(tx)) {
        uni.showToast({ title: '该流水不能在此账本删除', icon: 'none' })
        return
      }
      const next = { ...this.selectedMap }
      if (next[tx.id]) delete next[tx.id]
      else if (this.selectedCount < 100) next[tx.id] = true
      else {
        uni.showToast({ title: '单次最多选择 100 笔', icon: 'none' })
        return
      }
      this.selectedMap = next
    },
    async toggleMonthSelection(group) {
      if (!group.loaded) await this.loadMonth(group)
      const selectable = (this.groupedTransactions.find(item => item.month === group.month)?.transactions || [])
        .filter(tx => this.canSelectTransaction(tx))
      const allSelected = selectable.length > 0 && selectable.every(tx => this.isSelected(tx))
      const next = { ...this.selectedMap }
      if (allSelected) {
        selectable.forEach(tx => delete next[tx.id])
      } else {
        const available = Math.max(0, 100 - this.selectedCount)
        selectable.filter(tx => !next[tx.id]).slice(0, available).forEach(tx => { next[tx.id] = true })
        if (selectable.filter(tx => !this.selectedMap[tx.id]).length > available) {
          uni.showToast({ title: '已选择前 100 笔', icon: 'none' })
        }
      }
      this.selectedMap = next
    },
    isMonthSelected(group) {
      const selectable = (group.transactions || []).filter(tx => this.canSelectTransaction(tx))
      return selectable.length > 0 && selectable.every(tx => this.isSelected(tx))
    },
    onLongPress(tx) {
      if (!this.canSelectTransaction(tx)) return
      this.selectionMode = true
      if (!this.isSelected(tx)) this.toggleTransaction(tx)
    },
    onTap(tx) {
      if (this.selectionMode) {
        this.toggleTransaction(tx)
        return
      }
      // H5 长按不便时，个人账本点击也可操作
      if (this.isSharedLedger && !tx.source_id) return
      // #ifdef H5
      this.showTxActions(tx)
      // #endif
    },
    showTxActions(tx) {
      const itemList = [tx.source_id ? '从本账本移除' : '删除原流水']
      if (!tx.source_id && !this.isSharedLedger) {
        itemList.unshift('编辑流水')
      }
      if (!tx.source_id && this.targetLedgers.length > 0 && ['expense', 'income', 'transfer'].includes(tx.op_type)) {
        itemList.unshift('同步到账本')
      }
      uni.showActionSheet({
        itemList,
        success: (res) => {
          const action = itemList[res.tapIndex]
          if (action === '同步到账本') {
            this.pushTargetTx = tx
            this.pushMonthValue = ''
            if (this.targetLedgers.length === 1) {
              this.doPush(this.targetLedgers[0])
            } else {
              this.showPush = true
            }
          } else if (action === '编辑流水') {
            this.editTx(tx)
          } else {
            this.deleteTx(tx)
          }
        }
      })
    },
    editTx(tx) {
      uni.setStorageSync('edit_transaction', tx)
      uni.switchTab({ url: '/pages/record/index' })
    },
    async deleteTx(tx) {
      uni.showModal({
        title: '确认删除',
        content: tx.source_id
          ? `只会从「${this.currentLedger?.name || '当前账本'}」移除这条同步记录，不会删除个人账本中的原流水`
          : '删除个人账本中的原流水后，其他账本里仍存在的同步记录也会一并删除',
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
    batchDelete() {
      if (!this.selectedCount || this.batchLoading) return
      const content = this.isSharedLedger
        ? `将从「${this.currentLedger?.name || '当前账本'}」移除 ${this.selectedCount} 条同步记录，不影响个人账本原流水。`
        : `将删除 ${this.selectedCount} 条原流水，并同步删除其他账本里的对应记录。此操作不可撤销。`
      uni.showModal({
        title: '批量删除流水',
        content,
        confirmText: '确认删除',
        confirmColor: '#df665d',
        success: async result => {
          if (!result.confirm) return
          this.batchLoading = true
          try {
            const response = await post('/batch-delete-transactions', {
              transaction_ids: this.selectedIDs
            }, { ledgerId: this.currentLedger.id })
            uni.showToast({ title: `已删除 ${response?.deleted || this.selectedCount} 笔`, icon: 'success' })
            this.cancelSelection()
            await this.loadTransactions()
          } catch (err) {
            uni.showToast({ title: err.message || '批量删除失败', icon: 'none' })
          } finally {
            this.batchLoading = false
          }
        }
      })
    },
    startBatchPush() {
      if (!this.selectedCount) return
      if (this.targetLedgers.length === 0) {
        uni.showToast({ title: '请先创建或加入其他账本', icon: 'none' })
        return
      }
      this.pushTargetTx = null
      this.pushMonthValue = ''
      this.pushTransactionIDs = [...this.selectedIDs]
      if (this.targetLedgers.length === 1) this.doPush(this.targetLedgers[0])
      else this.showPush = true
    },
    pushMonth(month) {
      if (this.targetLedgers.length === 0) {
        uni.showToast({ title: '请先创建或加入其他账本', icon: 'none' })
        return
      }
      this.pushTargetTx = null
      this.pushMonthValue = month
      if (this.targetLedgers.length === 1) {
        this.doPush(this.targetLedgers[0])
      } else {
        this.showPush = true
      }
    },
    async doPush(targetLedger) {
      this.showPush = false
      try {
        if (this.pushTransactionIDs.length) {
          this.batchLoading = true
          const result = await post('/batch-push-transactions', {
            transaction_ids: this.pushTransactionIDs,
            target_ledger_id: targetLedger.id
          }, { ledgerId: this.currentLedger.id })
          const pushed = result?.pushed || 0
          const skipped = result?.skipped || 0
          const failed = result?.failed || 0
          const suffix = [skipped ? `跳过 ${skipped} 笔` : '', failed ? `失败 ${failed} 笔` : ''].filter(Boolean).join('，')
          uni.showToast({
            title: pushed ? `已同步 ${pushed} 笔${suffix ? `，${suffix}` : ''}` : (suffix || '没有可同步流水'),
            icon: 'none',
            duration: 2800
          })
          this.cancelSelection()
        } else if (this.pushMonthValue) {
          const result = await post('/push-month', {
            month: this.pushMonthValue,
            target_ledger_id: targetLedger.id
          }, { ledgerId: this.currentLedger.id })
          const pushed = result?.pushed ?? 0
          const skipped = result?.skipped ?? 0
          uni.showToast({
            title: pushed === 0
              ? (skipped > 0 ? '本月已全部同步过' : '本月没有可同步流水')
              : `已同步 ${pushed} 笔到「${targetLedger.name}」`,
            icon: 'none',
            duration: 2500
          })
        } else if (this.pushTargetTx) {
          await post(`/transactions/${this.pushTargetTx.id}/push`, {
            target_ledger_id: targetLedger.id
          }, { ledgerId: this.currentLedger.id })
          uni.showToast({ title: `已同步到「${targetLedger.name}」`, icon: 'success' })
        }
      } catch (err) {
        uni.showToast({ title: err.message || '同步失败', icon: 'none' })
      } finally {
        this.pushTargetTx = null
        this.pushMonthValue = ''
        this.pushTransactionIDs = []
        this.batchLoading = false
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
.page.selection-active { padding-bottom: 112px; }
.page-head { padding: 0 4px 18px; display: flex; align-items: flex-start; justify-content: space-between; gap: 12px; }
.page-title { display: block; color: #18312b; font-size: 27px; font-weight: 700; }
.page-copy { display: block; margin-top: 6px; color: #87968f; font-size: 13px; }
.batch-entry { flex-shrink: 0; margin-top: 5px; padding: 7px 11px; border-radius: 999px; color: #2fa680; background: #eaf7f2; font-size: 12px; font-weight: 650; }
.current-ledger {
  display: flex;
  align-items: center;
  gap: 12px;
  padding: 14px 15px;
  margin-bottom: 14px;
  border: 1px solid #e3ebe7;
  border-radius: 17px;
  background: #fff;
}
.current-ledger-icon { width: 39px; height: 39px; flex: 0 0 39px; display: flex; align-items: center; justify-content: center; border-radius: 13px; color: #2d8e70; background: #eaf7f2; font-size: 18px; font-weight: 700; }
.current-ledger-main { flex: 1; min-width: 0; }
.current-ledger-label { display: block; color: #98a59f; font-size: 10px; }
.current-ledger-name { display: block; margin-top: 2px; color: #263f38; font-size: 15px; font-weight: 700; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
.current-ledger-switch { flex-shrink: 0; color: #36a984; font-size: 12px; }
.hint {
  font-size: 12px;
  color: #71817a;
  background: #eaf7f2;
  border-radius: 12px;
  padding: 10px 12px;
  margin-bottom: 14px;
}
.selection-hint { padding: 9px 12px; margin-bottom: 12px; color: #7b6b3f; background: #fff7df; border-radius: 11px; font-size: 12px; }
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
.transaction-item.selected { margin: 0 -8px; padding-left: 8px; padding-right: 8px; border-radius: 13px; background: #f0faf6; }
.transaction-item.disabled { opacity: .52; }
.selection-check { width: 22px; height: 22px; flex: 0 0 22px; display: flex; align-items: center; justify-content: center; box-sizing: border-box; border: 1.5px solid #cbd7d2; border-radius: 7px; color: #fff; font-size: 13px; }
.selection-check.checked { border-color: #42c39c; background: #42c39c; }
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
.batch-bar { position: fixed; z-index: 900; left: 12px; right: 12px; bottom: calc(58px + env(safe-area-inset-bottom)); display: flex; align-items: center; gap: 9px; padding: 11px 12px; border: 1px solid #dfe8e4; border-radius: 17px; background: rgba(255,255,255,.97); box-shadow: 0 10px 35px rgba(24,49,43,.16); }
.batch-count { flex: 1; min-width: 0; display: flex; flex-direction: column; color: #31564c; font-size: 13px; font-weight: 700; }
.batch-clear { margin-top: 2px; color: #8b9994; font-size: 10px; font-weight: 400; }
.batch-button { width: 70px; min-height: 38px; margin: 0; padding: 8px 10px; border-radius: 11px; font-size: 13px; line-height: 1.4; }
.batch-button.sync { color: #fff; background: #42c39c; }
.batch-button.delete { color: #fff; background: #e56e66; }
.batch-button[disabled] { color: #9da9a4; background: #e8edeb; opacity: 1; }
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
@media (min-width: 900px) {
  .batch-bar { left: 256px; right: 36px; bottom: 24px; max-width: 860px; margin: 0 auto; }
}
</style>
