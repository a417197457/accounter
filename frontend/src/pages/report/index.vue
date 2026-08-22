<template>
  <view class="page">
    <WebSidebar />
    <view class="page-head">
      <view class="page-head-main">
        <text class="page-title">财务报表</text>
        <text class="page-copy">看见趋势，也看见生活</text>
      </view>
      <text class="page-action" @click="exportExcel">导出 Excel</text>
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

    <!-- 月份切换 -->
    <view class="month-nav">
      <text class="nav-btn" :class="{ disabled: !canPrevMonth }" @click="canPrevMonth && prevMonth()">‹</text>
      <view class="nav-current">
        <text class="nav-title">{{ currentMonth }}</text>
        <text class="nav-copy">仅显示当前月和有记录的月份</text>
      </view>
      <text class="nav-btn" :class="{ disabled: !canNextMonth }" @click="canNextMonth && nextMonth()">›</text>
    </view>

    <view class="report-loading" v-if="dataLoading">
      <view class="loading-dot"></view>
      <text>正在加载本月报表…</text>
    </view>

    <view v-else class="report-body">
    <!-- 当月总览 -->
    <view class="overview" v-if="overview">
      <text class="balance-label">本月结余</text>
      <text class="balance-value">¥{{ formatNum(overview.balance) }}</text>
      <view class="overview-stats">
        <view class="overview-item">
          <text class="overview-label">收入</text>
          <text class="overview-value income">¥{{ formatNum(overview.income) }}</text>
        </view>
        <view class="overview-divider"></view>
        <view class="overview-item">
          <text class="overview-label">支出</text>
          <text class="overview-value expense">¥{{ formatNum(overview.expense) }}</text>
        </view>
        <view class="overview-divider"></view>
        <view class="overview-item">
          <text class="overview-label">转账/还款</text>
          <text class="overview-value transfer">¥{{ formatNum(overview.transfer) }}</text>
        </view>
        <view class="overview-item">
          <text class="overview-label">当前净资产</text>
          <text class="overview-value">¥{{ formatNum(overview.total_assets) }}</text>
        </view>
      </view>
      <text class="assets-note">净资产已扣除信用卡欠款，转账与还款不计入支出</text>
    </view>

    <!-- 日历热力图 -->
    <view class="card calendar-card">
      <view class="card-title-row calendar-head">
        <view>
          <text class="card-title">月度日历</text>
          <text class="calendar-copy">颜色越深，当天金额越高</text>
        </view>
        <view class="calendar-modes">
          <text
            class="calendar-mode"
            :class="{ active: calendarMode === mode.value }"
            v-for="mode in calendarModes"
            :key="mode.value"
            @click="calendarMode = mode.value"
          >{{ mode.label }}</text>
        </view>
      </view>

      <view class="week-row">
        <text class="week-name" v-for="week in weekNames" :key="week">{{ week }}</text>
      </view>
      <view class="calendar-grid">
        <view
          class="calendar-cell"
          :class="{ empty: !cell.day, selected: cell.date === selectedDate, today: cell.date === todayDate }"
          :style="cell.day ? calendarCellStyle(cell) : {}"
          v-for="cell in calendarDays"
          :key="cell.key"
          @click="cell.day && selectCalendarDay(cell.date)"
        >
          <text class="calendar-day" v-if="cell.day">{{ cell.day }}</text>
          <text class="calendar-amount" v-if="cell.day">{{ calendarCellAmount(cell) }}</text>
        </view>
      </view>

      <view class="calendar-summary">
        <view class="calendar-summary-item">
          <text class="summary-label">{{ calendarModeLabel }}合计</text>
          <text class="summary-value" :class="calendarMode">¥{{ formatNum(calendarMonthValue) }}</text>
        </view>
        <view class="calendar-summary-item align-right">
          <text class="summary-label">日均{{ calendarModeLabel }}</text>
          <text class="summary-value">¥{{ formatNum(calendarDailyAverage) }}</text>
        </view>
      </view>

      <view class="selected-day">
        <view class="selected-day-head">
          <view>
            <text class="selected-day-title">{{ selectedDateLabel }}账单</text>
            <text class="selected-day-total">
              收入 ¥{{ formatNum(selectedDayStats.income) }} · 支出 ¥{{ formatNum(selectedDayStats.expense) }}
            </text>
          </view>
          <text class="selected-day-count">{{ selectedTransactions.length }} 笔</text>
        </view>
        <view class="day-loading" v-if="selectedDayLoading">正在加载当天明细…</view>
        <view class="day-empty" v-else-if="selectedTransactions.length === 0">当天没有流水</view>
        <view class="day-transactions" v-else>
          <view class="day-transaction" v-for="tx in selectedTransactions" :key="tx.id">
            <view class="day-tx-icon" :class="tx.op_type">{{ transactionIcon(tx) }}</view>
            <view class="day-tx-main">
              <text class="day-tx-title">{{ transactionTitle(tx) }}</text>
              <text class="day-tx-meta">{{ transactionMeta(tx) }}</text>
            </view>
            <text class="day-tx-amount" :class="tx.op_type">{{ transactionAmount(tx) }}</text>
          </view>
        </view>
      </view>
    </view>

    <view class="card">
      <view class="card-title">支出分类</view>
      <view v-if="categories.length === 0" class="empty-mini">本月无支出</view>
      <view v-else class="pie-wrap">
        <view class="pie" :style="{ background: pieBackgroundStyle }"></view>
        <view class="pie-legend">
          <view class="legend-item" v-for="(cat, i) in categories" :key="cat.category_id">
            <view class="legend-dot" :style="{ background: pieColors[i % pieColors.length] }"></view>
            <text class="legend-name">{{ cat.icon }} {{ cat.category_name }}</text>
            <text class="legend-amount">¥{{ formatNum(cat.amount) }}</text>
            <text class="legend-pct">{{ piePercent(cat.amount) }}%</text>
          </view>
        </view>
      </view>
    </view>

    <view class="card">
      <view class="card-title-row">
        <text class="card-title">每日趋势</text>
        <view class="trend-legend"><text class="income-dot"></text><text>收入</text><text class="expense-dot"></text><text>支出</text></view>
      </view>
      <view v-if="trend.length === 0" class="empty-mini">本月无流水</view>
      <view v-else class="trend-chart" :style="trendChartStyle">
        <view class="trend-bar" v-for="(point, i) in trend" :key="point.period">
          <view class="trend-col">
            <view
              class="trend-col-income"
              :style="{ height: trendValueHeight(point.income) + 'px', width: trendBarWidth }"
            ></view>
            <view
              class="trend-col-expense"
              :style="{ height: trendValueHeight(point.expense) + 'px', width: trendBarWidth }"
            ></view>
          </view>
          <text class="trend-label" :class="{ hide: !showTrendLabel(i) }">
            {{ showTrendLabel(i) ? dayLabel(point.period) : '' }}
          </text>
        </view>
      </view>
    </view>

    <view class="card">
      <view class="card-title-row">
        <text class="card-title">预算</text>
        <text class="card-action" @click="openBudgetModal">+ 设置</text>
      </view>
      <view v-if="budgets.length === 0" class="empty-mini">尚未设置预算，点击右上角添加</view>
      <view v-else>
        <view class="budget-item" v-for="b in budgets" :key="b.id">
          <view class="budget-label">
            <text>{{ b.category_name || '总预算' }}</text>
            <view class="budget-right">
              <text class="budget-amount" :class="{ over: parseFloat(b.spent) > parseFloat(b.amount) }">
                ¥{{ formatNum(b.spent) }} / {{ formatNum(b.amount) }}
              </text>
              <text class="budget-del" @click="deleteBudget(b)">删除</text>
            </view>
          </view>
          <view class="bar-track">
            <view
              class="bar-fill"
              :class="{ 'bar-over': parseFloat(b.spent) > parseFloat(b.amount) }"
              :style="{ width: budgetPercent(b) + '%' }"
            ></view>
          </view>
        </view>
      </view>
    </view>
    </view>

    <!-- 设置预算 -->
    <view class="modal" v-if="showBudget" @tap="closeBudgetModal">
      <view class="modal-content" @tap.stop>
        <text class="modal-title">设置预算</text>
        <view class="field">
          <text class="label">类型</text>
          <view class="budget-tabs">
            <text class="btab" :class="{ active: budgetForm.mode === 'total' }" @tap="setBudgetMode('total')">总预算</text>
            <text class="btab" :class="{ active: budgetForm.mode === 'category' }" @tap="setBudgetMode('category')">分类预算</text>
          </view>
        </view>
        <view class="field" v-if="budgetForm.mode === 'category'">
          <text class="label">一级分类</text>
          <scroll-view scroll-y class="cat-pick">
            <view
              class="cat-pick-item"
              :class="{ active: budgetForm.category_id === c.id }"
              v-for="c in expenseParents"
              :key="c.id"
              @click="budgetForm.category_id = c.id"
            >
              <text>{{ c.icon }} {{ c.name }}</text>
            </view>
            <text class="cat-pick-empty" v-if="expenseParents.length === 0">暂无支出一级分类，请先到分类管理中添加</text>
          </scroll-view>
        </view>
        <view class="field">
          <text class="label">每月金额</text>
          <view class="input-box">
            <input class="budget-input" type="digit" v-model="budgetForm.amount" placeholder="如 3000" />
          </view>
        </view>
        <view class="modal-actions">
          <button class="btn btn-secondary" @tap="closeBudgetModal">取消</button>
          <button class="btn btn-primary" @click="saveBudget" :loading="budgetSaving">保存</button>
        </view>
      </view>
    </view>
  </view>
</template>

<script>
import { get, put, del } from '../../common/request.js'
import { checkLogin } from '../../common/auth.js'
import { BASE_URL, TOKEN_KEY } from '../../common/config.js'
import { paymentRouteLabel } from '../../common/account.js'

export default {
  data() {
    return {
      ledgers: [],
      currentLedger: null,
      currentMonth: '',
      reportMonths: [],
      overview: null,
      dataLoading: false,
      dataRequestId: 0,
      categories: [],
      trend: [],
      budgets: [],
      expenseParents: [],
      calendarMode: 'expense',
      calendarModes: [
        { value: 'expense', label: '支出' },
        { value: 'income', label: '收入' },
        { value: 'balance', label: '结余' }
      ],
      weekNames: ['周一', '周二', '周三', '周四', '周五', '周六', '周日'],
      selectedDate: '',
      selectedTransactions: [],
      selectedDayLoading: false,
      selectedDayRequestId: 0,
      maxCategory: 0,
      maxTrend: 0,
      chartWidth: 320,
      pieColors: ['#42c39c', '#5b9bd5', '#f0a05a', '#f06f64', '#9b7bb8', '#6bc4c1', '#e8c468', '#7d8f99'],
      showBudget: false,
      budgetSaving: false,
      budgetForm: {
        mode: 'total',
        category_id: null,
        amount: ''
      }
    }
  },
  computed: {
    canPrevMonth() {
      const index = this.reportMonths.indexOf(this.currentMonth)
      return index > 0
    },
    canNextMonth() {
      const index = this.reportMonths.indexOf(this.currentMonth)
      return index >= 0 && index < this.reportMonths.length - 1
    },
    todayDate() {
      const now = new Date()
      return `${now.getFullYear()}-${String(now.getMonth() + 1).padStart(2, '0')}-${String(now.getDate()).padStart(2, '0')}`
    },
    dailyTrendMap() {
      const map = {}
      for (const point of this.trend || []) {
        map[point.period] = {
          income: parseFloat(point.income) || 0,
          expense: parseFloat(point.expense) || 0
        }
      }
      return map
    },
    calendarDays() {
      if (!this.currentMonth) return []
      const [year, month] = this.currentMonth.split('-').map(Number)
      const firstWeekday = (new Date(year, month - 1, 1).getDay() + 6) % 7
      const daysInMonth = new Date(year, month, 0).getDate()
      const result = []
      for (let i = 0; i < firstWeekday; i++) {
        result.push({ key: `empty-start-${i}`, day: 0, date: '' })
      }
      for (let day = 1; day <= daysInMonth; day++) {
        const date = `${this.currentMonth}-${String(day).padStart(2, '0')}`
        const values = this.dailyTrendMap[date] || { income: 0, expense: 0 }
        result.push({ key: date, day, date, ...values, balance: values.income - values.expense })
      }
      while (result.length % 7 !== 0) {
        result.push({ key: `empty-end-${result.length}`, day: 0, date: '' })
      }
      return result
    },
    calendarMaxValue() {
      const values = this.calendarDays.filter(c => c.day).map(c => Math.abs(this.calendarMetricValue(c)))
      return Math.max(0, ...values)
    },
    calendarModeLabel() {
      return this.calendarModes.find(mode => mode.value === this.calendarMode)?.label || '支出'
    },
    calendarMonthValue() {
      if (!this.overview) return 0
      if (this.calendarMode === 'income') return parseFloat(this.overview.income) || 0
      if (this.calendarMode === 'balance') return parseFloat(this.overview.balance) || 0
      return parseFloat(this.overview.expense) || 0
    },
    calendarDailyAverage() {
      const [year, month] = this.currentMonth.split('-').map(Number)
      const days = new Date(year, month, 0).getDate() || 1
      return this.calendarMonthValue / days
    },
    selectedDayStats() {
      return this.dailyTrendMap[this.selectedDate] || { income: 0, expense: 0 }
    },
    selectedDateLabel() {
      if (!this.selectedDate) return '当日'
      const [, month, day] = this.selectedDate.split('-').map(Number)
      return `${month}月${day}日`
    },
    pieBackgroundStyle() {
      const total = this.categories.reduce((s, c) => s + (parseFloat(c.amount) || 0), 0)
      if (total <= 0) return '#edf1ee'
      let cursor = 0
      const parts = this.categories.map((cat, i) => {
        const pct = ((parseFloat(cat.amount) || 0) / total) * 100
        const start = cursor
        cursor += pct
        const color = this.pieColors[i % this.pieColors.length]
        return `${color} ${start}% ${cursor}%`
      })
      return `conic-gradient(${parts.join(', ')})`
    },
    trendSlotWidth() {
      const n = Math.max(this.trend.length, 1)
      return this.chartWidth / n
    },
    trendLabelStep() {
      const slot = this.trendSlotWidth
      if (slot >= 20) return 1
      if (slot >= 14) return 2
      if (slot >= 10) return 3
      return 5
    },
    trendChartStyle() {
      const gap = this.trendSlotWidth >= 18 ? 3 : this.trendSlotWidth >= 12 ? 2 : 1
      return { gap: `${gap}px` }
    },
    trendBarWidth() {
      const slot = this.trendSlotWidth
      if (slot >= 22) return '34%'
      if (slot >= 14) return '32%'
      return '38%'
    }
  },
  onLoad() {
    checkLogin()
    const now = new Date()
    this.currentMonth = `${now.getFullYear()}-${String(now.getMonth() + 1).padStart(2, '0')}`
    this._onTrendResize = () => this.measureTrendChart()
  },
  onReady() {
    this.measureTrendChart()
    // #ifdef H5
    if (typeof window !== 'undefined') window.addEventListener('resize', this._onTrendResize)
    // #endif
    if (typeof uni.onWindowResize === 'function') uni.onWindowResize(this._onTrendResize)
  },
  onUnload() {
    // #ifdef H5
    if (typeof window !== 'undefined') window.removeEventListener('resize', this._onTrendResize)
    // #endif
    if (typeof uni.offWindowResize === 'function') uni.offWindowResize(this._onTrendResize)
  },
  onShow() {
    this.loadLedgers()
    this.measureTrendChart()
  },
  methods: {
    measureTrendChart() {
      this.$nextTick(() => {
        uni.createSelectorQuery()
          .in(this)
          .select('.trend-chart')
          .boundingClientRect((rect) => {
            if (rect?.width > 0) this.chartWidth = rect.width
          })
          .exec()
      })
    },
    dayLabel(period) {
      const day = (period || '').slice(8)
      return day ? String(Number(day)) : ''
    },
    showTrendLabel(i) {
      const n = this.trend.length
      if (n <= 1) return true
      const step = this.trendLabelStep
      return i % step === 0 || i === n - 1
    },
    formatNum(v) {
      const n = parseFloat(v)
      return Number.isFinite(n) ? n.toFixed(2) : '0.00'
    },
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
        if (this.currentMonth) {
          await this.loadReportMonths()
          this.loadData()
        }
      } catch (err) {
        if (err.message && err.message.includes('未登录')) {
          uni.redirectTo({ url: '/pages/login/index' })
        }
      }
    },
    async switchLedger(ledger) {
      this.currentLedger = ledger
      await this.loadReportMonths()
      this.loadData()
    },
    async loadReportMonths() {
      if (!this.currentLedger) return
      const now = new Date()
      const current = `${now.getFullYear()}-${String(now.getMonth() + 1).padStart(2, '0')}`
      try {
        const result = await get('/transaction-months', {}, this.ledgerOpts())
        const months = (result || []).map(item => item.month).filter(month => month && month <= current)
        this.reportMonths = [...new Set([...months, current])].sort()
      } catch (err) {
        this.reportMonths = [current]
      }
      if (!this.reportMonths.includes(this.currentMonth)) this.currentMonth = current
    },
    prevMonth() {
      const index = this.reportMonths.indexOf(this.currentMonth)
      if (index <= 0) return
      this.currentMonth = this.reportMonths[index - 1]
      this.resetCalendarSelection()
      this.loadData()
    },
    nextMonth() {
      const index = this.reportMonths.indexOf(this.currentMonth)
      if (index < 0 || index >= this.reportMonths.length - 1) return
      this.currentMonth = this.reportMonths[index + 1]
      this.resetCalendarSelection()
      this.loadData()
    },
    monthRange() {
      const [y, m] = this.currentMonth.split('-').map(Number)
      const start = `${y}-${String(m).padStart(2, '0')}-01`
      const lastDay = new Date(y, m, 0).getDate()
      const end = `${y}-${String(m).padStart(2, '0')}-${String(lastDay).padStart(2, '0')}`
      return { start, end }
    },
    ledgerOpts() {
      return this.currentLedger ? { ledgerId: this.currentLedger.id } : {}
    },
    async loadData() {
      if (!this.currentLedger) return
      const requestId = ++this.dataRequestId
      this.dataLoading = true
      const { start, end } = this.monthRange()
      const opts = this.ledgerOpts()
      try {
        const [overview, categories, trend, budgets, tree] = await Promise.all([
          get('/stats/overview', { start, end }, opts),
          get('/stats/category', { kind: 'expense', start, end }, opts),
          get('/stats/trend', { granularity: 'day', start, end }, opts),
          get('/budgets', { month: this.currentMonth }, opts),
          get('/categories/tree', { kind: 'expense' }, opts)
        ])
        if (requestId !== this.dataRequestId) return
        this.overview = overview
        this.categories = categories || []
        this.trend = trend || []
        this.budgets = budgets || []
        this.expenseParents = (tree || []).map(c => ({ id: c.id, name: c.name, icon: c.icon }))
        this.maxCategory = Math.max(0, ...this.categories.map(c => parseFloat(c.amount) || 0))
        this.maxTrend = Math.max(0, ...this.trend.flatMap(t => [parseFloat(t.expense) || 0, parseFloat(t.income) || 0]))
        this.ensureCalendarSelection()
        this.loadSelectedDay()
        this.measureTrendChart()
      } catch (err) {
        if (requestId !== this.dataRequestId) return
        if (err.message && err.message.includes('未登录')) {
          uni.redirectTo({ url: '/pages/login/index' })
        } else {
          uni.showToast({ title: err.message || '加载报表失败', icon: 'none' })
        }
      } finally {
        if (requestId === this.dataRequestId) this.dataLoading = false
      }
    },
    piePercent(amount) {
      const total = this.categories.reduce((s, c) => s + (parseFloat(c.amount) || 0), 0)
      if (total <= 0) return '0.0'
      return ((parseFloat(amount) / total) * 100).toFixed(1)
    },
    trendValueHeight(value) {
      if (this.maxTrend <= 0) return 0
      const amount = parseFloat(value) || 0
      if (amount <= 0) return 0
      return Math.max(2, (amount / this.maxTrend) * 86)
    },
    resetCalendarSelection() {
      this.selectedDate = ''
      this.selectedTransactions = []
    },
    ensureCalendarSelection() {
      if (this.selectedDate.startsWith(this.currentMonth)) return
      this.selectedDate = this.todayDate.startsWith(this.currentMonth)
        ? this.todayDate
        : `${this.currentMonth}-01`
    },
    calendarMetricValue(cell) {
      if (this.calendarMode === 'income') return cell.income || 0
      if (this.calendarMode === 'balance') return cell.balance || 0
      return cell.expense || 0
    },
    calendarCellStyle(cell) {
      const value = this.calendarMetricValue(cell)
      const ratio = this.calendarMaxValue > 0 ? Math.abs(value) / this.calendarMaxValue : 0
      const alpha = ratio <= 0 ? 0.05 : 0.12 + ratio * 0.62
      let rgb = '240, 111, 100'
      if (this.calendarMode === 'income' || (this.calendarMode === 'balance' && value >= 0)) {
        rgb = '66, 195, 156'
      }
      return {
        background: `rgba(${rgb}, ${alpha.toFixed(2)})`,
        color: ratio > 0.64 ? '#ffffff' : '#31564c'
      }
    },
    calendarCellAmount(cell) {
      const value = this.calendarMetricValue(cell)
      if (!value) return '—'
      const sign = this.calendarMode === 'expense' ? '-' : value > 0 ? '+' : '-'
      const abs = Math.abs(value)
      const compact = abs >= 10000
        ? `${(abs / 10000).toFixed(abs >= 100000 ? 0 : 1)}万`
        : abs >= 1000
          ? `${(abs / 1000).toFixed(1)}k`
          : abs.toFixed(abs >= 100 ? 0 : 1).replace(/\.0$/, '')
      return `${sign}${compact}`
    },
    selectCalendarDay(date) {
      if (!date || date === this.selectedDate) return
      this.selectedDate = date
      this.loadSelectedDay()
    },
    nextDate(date) {
      const [year, month, day] = date.split('-').map(Number)
      const next = new Date(year, month - 1, day + 1)
      return `${next.getFullYear()}-${String(next.getMonth() + 1).padStart(2, '0')}-${String(next.getDate()).padStart(2, '0')}`
    },
    async loadSelectedDay() {
      if (!this.currentLedger || !this.selectedDate) return
      const requestId = ++this.selectedDayRequestId
      this.selectedDayLoading = true
      this.selectedTransactions = []
      try {
        const result = await get('/transactions', {
          start: this.selectedDate,
          end: this.nextDate(this.selectedDate),
          page: 1,
          page_size: 100
        }, this.ledgerOpts())
        if (requestId !== this.selectedDayRequestId) return
        this.selectedTransactions = (result?.list || []).filter(tx => (tx.occurred_at || '').slice(0, 10) === this.selectedDate)
      } catch (err) {
        if (requestId !== this.selectedDayRequestId) return
        uni.showToast({ title: err.message || '加载当天明细失败', icon: 'none' })
      } finally {
        if (requestId === this.selectedDayRequestId) this.selectedDayLoading = false
      }
    },
    transactionIcon(tx) {
      if (tx.op_type === 'transfer') return tx.to_account?.type === 'credit' ? '✓' : '↔'
      return tx.category?.icon || tx.category?.parent?.icon || '·'
    },
    transactionTitle(tx) {
      if (tx.op_type === 'transfer') return tx.to_account?.type === 'credit' ? '信用卡还款' : '账户转账'
      return tx.category?.name || '未分类'
    },
    transactionMeta(tx) {
      const time = (tx.occurred_at || '').slice(11, 16)
      const account = paymentRouteLabel(tx)
      return [tx.remark, account, time].filter(Boolean).join(' · ')
    },
    transactionAmount(tx) {
      const amount = this.formatNum(tx.amount)
      if (tx.op_type === 'expense' || tx.op_type === 'borrow_out') return `-¥${amount}`
      if (tx.op_type === 'income' || tx.op_type === 'borrow_in') return `+¥${amount}`
      return `¥${amount}`
    },
    budgetPercent(b) {
      const amount = parseFloat(b.amount) || 0
      if (amount <= 0) return 0
      return Math.min(100, (parseFloat(b.spent) / amount) * 100)
    },
    openBudgetModal() {
      this.budgetForm = { mode: 'total', category_id: null, amount: '' }
      this.showBudget = true
      this.$nextTick(() => {
        // #ifdef H5
        setTimeout(() => {
          document.querySelectorAll('.budget-input input').forEach(input => {
            input.style.height = '24px'
            input.style.lineHeight = '24px'
            input.style.width = '100%'
            input.style.padding = '0 8px'
            input.style.border = 'none'
            input.style.outline = 'none'
            input.style.background = 'transparent'
            input.style.display = 'block'
          })
        }, 50)
        // #endif
      })
    },
    closeBudgetModal() {
      if (!this.budgetSaving) this.showBudget = false
    },
    setBudgetMode(mode) {
      this.budgetForm.mode = mode
      if (mode === 'total') this.budgetForm.category_id = null
    },
    async saveBudget() {
      const amount = parseFloat(this.budgetForm.amount)
      if (!amount || amount <= 0) {
        uni.showToast({ title: '请输入有效金额', icon: 'none' })
        return
      }
      if (this.budgetForm.mode === 'category' && !this.budgetForm.category_id) {
        uni.showToast({ title: '请选择分类', icon: 'none' })
        return
      }
      this.budgetSaving = true
      try {
        const body = { amount: String(amount) }
        if (this.budgetForm.mode === 'category') {
          body.category_id = this.budgetForm.category_id
        }
        await put('/budgets', body, this.ledgerOpts())
        uni.showToast({ title: '预算已保存', icon: 'success' })
        this.showBudget = false
        this.loadData()
      } catch (err) {
        uni.showToast({ title: err.message || '保存失败', icon: 'none' })
      } finally {
        this.budgetSaving = false
      }
    },
    async deleteBudget(b) {
      uni.showModal({
        title: '删除预算',
        content: `确定删除「${b.category_name || '总预算'}」吗？`,
        success: async (res) => {
          if (!res.confirm) return
          try {
            await del(`/budgets/${b.id}`, {}, this.ledgerOpts())
            uni.showToast({ title: '已删除', icon: 'success' })
            this.loadData()
          } catch (err) {
            uni.showToast({ title: err.message || '删除失败', icon: 'none' })
          }
        }
      })
    },
    async exportExcel() {
      if (!this.currentLedger) return
      const token = uni.getStorageSync(TOKEN_KEY)
      const url = `${BASE_URL}/api/report/export?month=${this.currentMonth}`

      // #ifdef H5
      try {
        const res = await fetch(url, {
          headers: {
            Authorization: `Bearer ${token}`,
            'X-Ledger-Id': String(this.currentLedger.id)
          }
        })
        if (!res.ok) {
          let msg = '导出失败'
          try {
            const j = await res.json()
            if (j.error) msg = j.error
          } catch (e) {}
          throw new Error(msg)
        }
        const blob = await res.blob()
        const a = document.createElement('a')
        a.href = URL.createObjectURL(blob)
        a.download = `accounter_${this.currentMonth}.xlsx`
        a.click()
        URL.revokeObjectURL(a.href)
      } catch (err) {
        uni.showToast({ title: err.message || '导出失败', icon: 'none' })
      }
      // #endif

      // #ifdef MP-WEIXIN
      uni.showLoading({ title: '正在生成报表...' })
      try {
        const filePath = await new Promise((resolve, reject) => {
          uni.downloadFile({
            url,
            header: {
              Authorization: `Bearer ${token}`,
              'X-Ledger-Id': String(this.currentLedger.id)
            },
            success: (res) => {
              if (res.statusCode >= 200 && res.statusCode < 300 && res.tempFilePath) {
                resolve(res.tempFilePath)
              } else {
                reject(new Error(`导出失败（HTTP ${res.statusCode || 0}）`))
              }
            },
            fail: reject
          })
        })
        await new Promise((resolve, reject) => {
          uni.openDocument({
            filePath,
            fileType: 'xlsx',
            showMenu: true,
            success: resolve,
            fail: reject
          })
        })
      } catch (err) {
        uni.showToast({ title: err.errMsg || err.message || '导出失败', icon: 'none' })
      } finally {
        uni.hideLoading()
      }
      // #endif
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
.report-loading {
  min-height: 180px;
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  gap: 12px;
  color: #87968f;
  background: #fff;
  border: 1px solid #edf1ee;
  border-radius: 20px;
  font-size: 13px;
}
.loading-dot {
  width: 10px;
  height: 10px;
  border-radius: 50%;
  background: #42c39c;
  box-shadow: -17px 0 0 rgba(66,195,156,.24), 17px 0 0 rgba(66,195,156,.54);
  animation: report-pulse 1s ease-in-out infinite alternate;
}
@keyframes report-pulse { from { opacity: .45; } to { opacity: 1; } }
.page-head {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
  padding: 0 4px 18px;
}
.page-head-main { flex: 1; min-width: 0; }
.page-title { display: block; color: #18312b; font-size: 27px; font-weight: 700; }
.page-copy { display: block; margin-top: 6px; color: #87968f; font-size: 13px; }
.page-action {
  flex-shrink: 0;
  padding: 8px 0 8px 8px;
  color: #36b88f;
  font-size: 13px;
  font-weight: 600;
  line-height: 1.4;
  white-space: nowrap;
  cursor: pointer;
}
.ledger-tabs {
  display: flex;
  gap: 8px;
  margin-bottom: 14px;
  overflow-x: auto;
  scrollbar-width: none;
}
.ledger-tabs::-webkit-scrollbar { display: none; }
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
}
.month-nav {
  display: flex;
  align-items: center;
  justify-content: center;
  gap: 24px;
  padding: 11px 14px;
  background: #fff;
  border: 1px solid #edf1ee;
  border-radius: 15px;
  margin-bottom: 14px;
}
.nav-btn {
  font-size: 25px;
  color: #36b88f;
  padding: 4px 12px;
}
.nav-btn.disabled { color: #cbd3cf; }
.nav-current { min-width: 178px; display: flex; flex-direction: column; align-items: center; gap: 2px; }
.nav-title {
  color: #31564c;
  font-size: 16px;
  font-weight: 700;
}
.nav-copy { color: #9aa7a1; font-size: 9px; }
.card {
  background: #fff;
  border: 1px solid #edf1ee;
  border-radius: 18px;
  padding: 18px;
  margin-bottom: 14px;
  box-shadow: 0 8px 28px rgba(24, 49, 43, 0.04);
}
.card-title-row {
  display: flex;
  justify-content: space-between;
  align-items: center;
  gap: 12px;
  margin-bottom: 12px;
}
.card-title {
  color: #263f38;
  font-size: 16px;
  font-weight: 700;
  margin-bottom: 12px;
}
.card-title-row .card-title {
  flex: 1;
  min-width: 0;
  margin-bottom: 0;
}
.card-action {
  flex-shrink: 0;
  padding: 4px 0 4px 8px;
  font-size: 13px;
  font-weight: 600;
  color: #36b88f;
  white-space: nowrap;
  cursor: pointer;
}
.overview {
  padding: 22px 20px 18px;
  margin-bottom: 14px;
  color: #fff;
  background: #43c59e;
  border-radius: 20px;
  overflow: hidden;
  box-shadow: 0 14px 28px rgba(67, 197, 158, 0.22);
}
.balance-label { display: block; color: rgba(255,255,255,.82); font-size: 13px; }
.balance-value { display: block; margin-top: 5px; font-size: 34px; font-weight: 700; letter-spacing: .5px; }
.overview-stats {
  display: grid;
  grid-template-columns: 1fr 1fr;
  gap: 8px;
  margin-top: 18px;
}
.overview-item {
  min-width: 0;
  padding: 12px;
  display: flex;
  flex-direction: column;
  align-items: flex-start;
  background: rgba(255,255,255,.15);
  border-radius: 12px;
}
.overview-label {
  font-size: 13px;
  color: rgba(255,255,255,.75);
  margin-bottom: 6px;
}
.overview-value {
  color: #fff;
  font-size: 16px;
  font-weight: 700;
  font-variant-numeric: tabular-nums;
}
.overview-value.income { color: #fff; }
.overview-value.expense { color: #fff5f2; }
.overview-value.transfer { color: #e8fff7; }
.assets-note {
  display: block;
  margin-top: 13px;
  color: rgba(255,255,255,.76);
  font-size: 11px;
  text-align: left;
}
.overview-divider {
  display: none;
}
.calendar-card { padding: 18px 14px 0; overflow: hidden; }
.calendar-head { align-items: flex-start; margin-bottom: 16px; padding: 0 4px; }
.calendar-copy { display: block; margin-top: 5px; color: #98a59f; font-size: 11px; }
.calendar-modes {
  display: flex;
  align-items: center;
  padding: 3px;
  background: #f1f5f3;
  border-radius: 11px;
}
.calendar-mode {
  padding: 6px 9px;
  color: #87958f;
  font-size: 11px;
  line-height: 1;
  border-radius: 8px;
  cursor: pointer;
}
.calendar-mode.active {
  color: #31564c;
  font-weight: 700;
  background: #fff;
  box-shadow: 0 3px 10px rgba(24,49,43,.08);
}
.week-row,
.calendar-grid {
  display: grid;
  grid-template-columns: repeat(7, minmax(0, 1fr));
  gap: 6px;
}
.week-row { margin-bottom: 7px; }
.week-name {
  color: #95a19c;
  font-size: 10px;
  line-height: 20px;
  text-align: center;
}
.calendar-cell {
  position: relative;
  min-width: 0;
  height: 49px;
  padding: 6px 2px 4px;
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: space-between;
  border: 1.5px solid transparent;
  border-radius: 12px;
  box-sizing: border-box;
  cursor: pointer;
  transition: transform .16s, border-color .16s;
}
.calendar-cell.empty { background: transparent !important; cursor: default; }
.calendar-cell:not(.empty):active { transform: scale(.95); }
.calendar-cell.selected {
  border-color: #31564c;
  box-shadow: 0 4px 12px rgba(24,49,43,.12);
}
.calendar-cell.today::after {
  content: '';
  position: absolute;
  top: 5px;
  right: 5px;
  width: 4px;
  height: 4px;
  border-radius: 50%;
  background: #31564c;
}
.calendar-day { font-size: 13px; font-weight: 700; line-height: 17px; }
.calendar-amount {
  width: 100%;
  font-size: 8px;
  line-height: 12px;
  text-align: center;
  white-space: nowrap;
  overflow: hidden;
}
.calendar-summary {
  margin: 16px -14px 0;
  padding: 14px 18px;
  display: grid;
  grid-template-columns: 1fr 1fr;
  gap: 12px;
  border-top: 1px solid #edf1ee;
  border-bottom: 1px solid #edf1ee;
  background: #fbfcfb;
}
.calendar-summary-item { display: flex; flex-direction: column; gap: 5px; }
.calendar-summary-item.align-right { align-items: flex-end; }
.summary-label { color: #87958f; font-size: 11px; }
.summary-value { color: #31564c; font-size: 15px; font-weight: 700; font-variant-numeric: tabular-nums; }
.summary-value.expense { color: #f06f64; }
.summary-value.income { color: #36b88f; }
.selected-day { margin: 0 -14px; }
.selected-day-head {
  padding: 16px 18px 11px;
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
}
.selected-day-title { display: block; color: #263f38; font-size: 14px; font-weight: 700; }
.selected-day-total { display: block; margin-top: 5px; color: #8a9892; font-size: 10px; }
.selected-day-count {
  flex-shrink: 0;
  padding: 4px 8px;
  color: #36b88f;
  background: #eaf7f2;
  border-radius: 999px;
  font-size: 10px;
}
.day-loading,
.day-empty { padding: 7px 18px 18px; color: #a0aba6; font-size: 12px; text-align: center; }
.day-transactions { padding: 0 18px 6px; }
.day-transaction {
  min-width: 0;
  padding: 11px 0;
  display: flex;
  align-items: center;
  gap: 10px;
  border-top: 1px solid #f0f3f1;
}
.day-tx-icon {
  width: 34px;
  height: 34px;
  flex: 0 0 34px;
  display: flex;
  align-items: center;
  justify-content: center;
  color: #f06f64;
  background: #fff0ed;
  border-radius: 12px;
  font-size: 16px;
}
.day-tx-icon.income,.day-tx-icon.borrow_in { color: #36b88f; background: #eaf7f2; }
.day-tx-icon.transfer { color: #71817a; background: #eef3f1; }
.day-tx-main { flex: 1; min-width: 0; }
.day-tx-title { display: block; color: #31564c; font-size: 13px; font-weight: 600; }
.day-tx-meta {
  display: block;
  margin-top: 4px;
  color: #95a19c;
  font-size: 10px;
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
}
.day-tx-amount { flex-shrink: 0; color: #526860; font-size: 13px; font-weight: 700; font-variant-numeric: tabular-nums; }
.day-tx-amount.expense,.day-tx-amount.borrow_out { color: #f06f64; }
.day-tx-amount.income,.day-tx-amount.borrow_in { color: #36b88f; }
.empty-mini {
  text-align: center;
  color: #ccc;
  font-size: 13px;
  padding: 16px 0;
}
.pie-wrap {
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 18px;
}
.pie {
  width: 168px;
  height: 168px;
  border-radius: 50%;
  box-shadow: inset 0 0 0 14px #fff;
  background-color: #edf1ee;
}
.pie-legend {
  width: 100%;
  display: flex;
  flex-direction: column;
  gap: 10px;
}
@media (min-width: 700px) {
  .pie-wrap { flex-direction: row; justify-content: center; gap: 34px; }
  .pie { flex: 0 0 168px; }
  .pie-legend { max-width: 460px; }
  .calendar-card { padding-left: 20px; padding-right: 20px; }
  .calendar-cell { height: 62px; padding-top: 9px; border-radius: 14px; }
  .calendar-day { font-size: 14px; }
  .calendar-amount { font-size: 10px; }
  .calendar-summary,.selected-day { margin-left: -20px; margin-right: -20px; }
}
.legend-item {
  display: flex;
  align-items: center;
  gap: 8px;
  font-size: 13px;
}
.legend-dot {
  width: 8px;
  height: 8px;
  border-radius: 50%;
  flex-shrink: 0;
}
.legend-name {
  flex: 1;
  min-width: 0;
  color: #31564c;
}
.legend-amount {
  color: #526860;
  font-weight: 600;
}
.legend-pct {
  width: 42px;
  text-align: right;
  color: #9aa6a1;
  font-size: 12px;
}
.bar-track {
  width: 100%;
  height: 8px;
  background: #edf3f0;
  border-radius: 4px;
  overflow: hidden;
}
.bar-fill {
  height: 100%;
  background: #43c59e;
  border-radius: 4px;
}
.bar-over, .bar-fill.bar-over {
  background: #f06f64;
}
.trend-chart {
  display: flex;
  align-items: flex-end;
  width: 100%;
  height: 118px;
  box-sizing: border-box;
  overflow: hidden;
}
.trend-bar {
  flex: 1 1 0;
  min-width: 0;
  height: 100%;
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: flex-end;
}
.trend-col {
  width: 100%;
  height: 96px;
  display: flex;
  align-items: flex-end;
  justify-content: center;
  gap: 2px;
}
.trend-col-income,
.trend-col-expense {
  max-width: 18px;
  min-width: 2px;
  background: #f47b70;
  border-radius: 4px 4px 2px 2px;
  min-height: 2px;
}
.trend-col-income { background: #42c39c; }
.trend-legend { display: flex; align-items: center; gap: 5px; color: #87968f; font-size: 10px; }
.trend-legend .income-dot,.trend-legend .expense-dot { width: 7px; height: 7px; border-radius: 50%; }
.trend-legend .income-dot { background: #42c39c; }.trend-legend .expense-dot { margin-left: 5px; background: #f47b70; }
.trend-label {
  width: 100%;
  height: 14px;
  margin-top: 4px;
  font-size: 10px;
  line-height: 14px;
  color: #9aa6a1;
  text-align: center;
  overflow: hidden;
}
.trend-label.hide {
  color: transparent;
}
.budget-item {
  margin-bottom: 12px;
  display: flex;
  flex-direction: column;
  gap: 6px;
}
.budget-label {
  display: flex;
  justify-content: space-between;
  align-items: center;
  font-size: 13px;
  gap: 8px;
}
.budget-right {
  display: flex;
  align-items: center;
  gap: 8px;
}
.budget-amount { color: #666; }
.budget-amount.over { color: #f06f64; font-weight: bold; }
.budget-del {
  font-size: 12px;
  color: #f06f64;
}
.modal {
  position: fixed;
  top: 0; left: 0; right: 0; bottom: 0;
  background: rgba(24,49,43,0.32);
  display: flex;
  align-items: center;
  justify-content: center;
  z-index: 999;
}
.modal-content {
  background: #fff;
  border-radius: 20px;
  padding: 24px;
  width: 85%;
  max-width: 400px;
  max-height: 80vh;
  overflow-y: auto;
}
.modal-title {
  font-size: 18px;
  font-weight: bold;
  display: block;
  text-align: center;
  margin-bottom: 16px;
}
.field { margin-bottom: 14px; }
.label {
  font-size: 13px;
  color: #666;
  display: block;
  margin-bottom: 8px;
}
.budget-tabs {
  display: flex;
  gap: 8px;
}
.btab {
  flex: 1;
  text-align: center;
  padding: 8px;
  border: 1px solid #dce6e2;
  border-radius: 12px;
  font-size: 13px;
}
.btab.active {
  background: #42c39c;
  color: #fff;
  border-color: #42c39c;
}
.cat-pick {
  max-height: 180px;
}
.cat-pick-item {
  padding: 10px 8px;
  border-bottom: 1px solid #f0f0f0;
  font-size: 14px;
}
.cat-pick-item.active {
  color: #36b88f;
  font-weight: 600;
  background: #eaf7f2;
}
.input-box {
  height: 48px;
  padding: 0 13px;
  display: flex;
  align-items: center;
  border: 1px solid #dce6e2;
  border-radius: 12px;
  box-sizing: border-box;
}
.budget-input { width: 100%; height: 24px; display: block; }
.cat-pick-empty {
  display: block;
  padding: 18px 12px;
  color: #96a39d;
  font-size: 12px;
  line-height: 1.5;
  text-align: center;
}
:deep(.budget-input .uni-input-wrapper) {
  width: 100%;
  height: 24px;
  border: none !important;
  padding: 0 !important;
}
:deep(.budget-input input) {
  height: 24px !important;
  line-height: 24px !important;
  padding: 0 8px !important;
  margin: 0 !important;
}
.modal-actions {
  display: flex;
  align-items: center;
  gap: 12px;
  margin-top: 8px;
}
.modal-actions .btn {
  flex: 1;
  width: auto !important;
  margin: 0;
  min-height: 44px;
  display: flex;
  align-items: center;
  justify-content: center;
  line-height: 1.4 !important;
}
</style>
