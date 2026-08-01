<template>
  <view class="page">
    <!-- 月份切换 -->
    <view class="month-nav">
      <text class="nav-btn" @click="prevMonth">‹</text>
      <text class="nav-title">{{ currentMonth }}</text>
      <text class="nav-btn" @click="nextMonth">›</text>
    </view>

    <!-- 总览 -->
    <view class="card overview" v-if="overview">
      <view class="overview-item">
        <text class="overview-label">收入</text>
        <text class="overview-value income">+{{ overview.income }}</text>
      </view>
      <view class="overview-divider"></view>
      <view class="overview-item">
        <text class="overview-label">支出</text>
        <text class="overview-value expense">-{{ overview.expense }}</text>
      </view>
      <view class="overview-divider"></view>
      <view class="overview-item">
        <text class="overview-label">结余</text>
        <text class="overview-value">{{ overview.balance }}</text>
      </view>
    </view>

    <!-- 支出分类占比 -->
    <view class="card">
      <view class="card-title">支出分类</view>
      <view v-if="categories.length === 0" class="empty-mini">本月无支出</view>
      <view v-else class="bar-list">
        <view class="bar-item" v-for="cat in categories" :key="cat.category_id">
          <view class="bar-label">
            <text>{{ cat.icon }} {{ cat.category_name }}</text>
            <text class="bar-amount">¥{{ cat.amount }}</text>
          </view>
          <view class="bar-track">
            <view class="bar-fill" :style="{ width: catPercent(cat.amount) + '%' }"></view>
          </view>
        </view>
      </view>
    </view>

    <!-- 每日趋势 -->
    <view class="card">
      <view class="card-title">每日趋势</view>
      <view v-if="trend.length === 0" class="empty-mini">本月无流水</view>
      <view v-else class="trend-chart">
        <view class="trend-bar" v-for="point in trend" :key="point.period">
          <view class="trend-col">
            <view class="trend-col-expense" :style="{ height: trendExpHeight(point.expense) + 'px' }"></view>
          </view>
          <text class="trend-label">{{ point.period.slice(8) }}</text>
        </view>
      </view>
    </view>

    <!-- 预算 -->
    <view class="card" v-if="budgets.length > 0">
      <view class="card-title">预算</view>
      <view class="budget-item" v-for="b in budgets" :key="b.id">
        <view class="budget-label">
          <text>{{ b.category_name || '总预算' }}</text>
          <text class="budget-amount" :class="{ over: b.spent > b.amount }">
            ¥{{ b.spent }} / {{ b.amount }}
          </text>
        </view>
        <view class="bar-track">
          <view
            class="bar-fill"
            :class="{ 'bar-over': b.spent > b.amount }"
            :style="{ width: budgetPercent(b) + '%' }"
          ></view>
        </view>
      </view>
    </view>

    <!-- 导出 -->
    <button class="btn btn-primary export-btn" @click="exportExcel">导出本月 Excel</button>
  </view>
</template>

<script>
import { get } from '../../common/request.js'
import { checkLogin } from '../../common/auth.js'
import { BASE_URL, TOKEN_KEY } from '../../common/config.js'

export default {
  data() {
    return {
      currentMonth: '',
      overview: null,
      categories: [],
      trend: [],
      budgets: [],
      maxCategory: 0,
      maxTrend: 0
    }
  },
  onLoad() {
    checkLogin()
    const now = new Date()
    this.currentMonth = `${now.getFullYear()}-${String(now.getMonth() + 1).padStart(2, '0')}`
  },
  onShow() {
    if (this.currentMonth) this.loadData()
  },
  methods: {
    prevMonth() {
      const [y, m] = this.currentMonth.split('-').map(Number)
      const d = new Date(y, m - 2, 1)
      this.currentMonth = `${d.getFullYear()}-${String(d.getMonth() + 1).padStart(2, '0')}`
      this.loadData()
    },
    nextMonth() {
      const [y, m] = this.currentMonth.split('-').map(Number)
      const d = new Date(y, m, 1)
      this.currentMonth = `${d.getFullYear()}-${String(d.getMonth() + 1).padStart(2, '0')}`
      this.loadData()
    },
    monthRange() {
      const [y, m] = this.currentMonth.split('-').map(Number)
      const start = `${y}-${String(m).padStart(2, '0')}-01`
      const lastDay = new Date(y, m, 0).getDate()
      const end = `${y}-${String(m).padStart(2, '0')}-${String(lastDay).padStart(2, '0')}`
      return { start, end }
    },
    async loadData() {
      const { start, end } = this.monthRange()
      try {
        const [overview, categories, trend, budgets] = await Promise.all([
          get('/stats/overview'),
          get('/stats/category', { kind: 'expense', start, end }),
          get('/stats/trend', { granularity: 'day', start, end }),
          get('/budgets')
        ])
        this.overview = overview
        this.categories = categories || []
        this.trend = trend || []
        this.budgets = budgets || []
        this.maxCategory = Math.max(0, ...this.categories.map(c => parseFloat(c.amount) || 0))
        this.maxTrend = Math.max(0, ...this.trend.map(t => parseFloat(t.expense) || 0))
      } catch (err) {
        if (err.message && err.message.includes('未登录')) {
          uni.redirectTo({ url: '/pages/login/index' })
        }
      }
    },
    catPercent(amount) {
      if (this.maxCategory <= 0) return 0
      return Math.min(100, (parseFloat(amount) / this.maxCategory) * 100)
    },
    trendExpHeight(expense) {
      if (this.maxTrend <= 0) return 0
      return Math.max(2, (parseFloat(expense) / this.maxTrend) * 80)
    },
    budgetPercent(b) {
      const pct = (parseFloat(b.spent) / parseFloat(b.amount)) * 100
      return Math.min(100, pct)
    },
    async exportExcel() {
      // H5 用 fetch + blob 下载
      const token = uni.getStorageSync(TOKEN_KEY)
      const url = `${BASE_URL}/api/report/export?month=${this.currentMonth}`
      try {
        const res = await fetch(url, {
          headers: { Authorization: `Bearer ${token}` }
        })
        if (!res.ok) throw new Error('导出失败')
        const blob = await res.blob()
        const a = document.createElement('a')
        a.href = URL.createObjectURL(blob)
        a.download = `accounter_${this.currentMonth}.xlsx`
        a.click()
        URL.revokeObjectURL(a.href)
      } catch (err) {
        uni.showToast({ title: err.message || '导出失败', icon: 'none' })
      }
    }
  }
}
</script>

<style scoped>
.page {
  padding: 16px;
  min-height: 100vh;
  background: #f5f5f5;
}
.month-nav {
  display: flex;
  align-items: center;
  justify-content: center;
  gap: 24px;
  padding: 12px;
  background: #fff;
  border-radius: 12px;
  margin-bottom: 12px;
}
.nav-btn {
  font-size: 24px;
  color: #007aff;
  cursor: pointer;
  padding: 4px 12px;
}
.nav-title {
  font-size: 18px;
  font-weight: bold;
}
.card {
  background: #fff;
  border-radius: 12px;
  padding: 16px;
  margin-bottom: 12px;
}
.card-title {
  font-size: 15px;
  font-weight: bold;
  margin-bottom: 12px;
}
.overview {
  display: flex;
  align-items: center;
  justify-content: space-around;
  padding: 20px 16px;
}
.overview-item {
  flex: 1;
  display: flex;
  flex-direction: column;
  align-items: center;
}
.overview-label {
  font-size: 13px;
  color: #999;
  margin-bottom: 6px;
}
.overview-value {
  font-size: 18px;
  font-weight: bold;
}
.overview-value.income { color: #34c759; }
.overview-value.expense { color: #ff3b30; }
.overview-divider {
  width: 1px;
  height: 30px;
  background: #eee;
}
.empty-mini {
  text-align: center;
  color: #ccc;
  font-size: 13px;
  padding: 16px 0;
}
.bar-list {
  display: flex;
  flex-direction: column;
  gap: 12px;
}
.bar-item {
  display: flex;
  flex-direction: column;
  gap: 6px;
}
.bar-label {
  display: flex;
  justify-content: space-between;
  font-size: 13px;
}
.bar-amount { color: #666; }
.bar-track {
  width: 100%;
  height: 8px;
  background: #f0f0f0;
  border-radius: 4px;
  overflow: hidden;
}
.bar-fill {
  height: 100%;
  background: #007aff;
  border-radius: 4px;
  transition: width 0.3s;
}
.bar-over, .bar-fill.bar-over {
  background: #ff3b30;
}
.trend-chart {
  display: flex;
  align-items: flex-end;
  gap: 4px;
  height: 100px;
  overflow-x: auto;
  padding-bottom: 4px;
}
.trend-bar {
  display: flex;
  flex-direction: column;
  align-items: center;
  min-width: 24px;
  height: 100%;
  justify-content: flex-end;
}
.trend-col {
  width: 100%;
  height: 90px;
  display: flex;
  align-items: flex-end;
  justify-content: center;
}
.trend-col-expense {
  width: 60%;
  background: #ff3b30;
  border-radius: 2px 2px 0 0;
  min-height: 2px;
}
.trend-label {
  font-size: 10px;
  color: #999;
  margin-top: 4px;
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
  font-size: 13px;
}
.budget-amount { color: #666; }
.budget-amount.over { color: #ff3b30; font-weight: bold; }
.export-btn {
  width: 100%;
  height: 44px;
  font-size: 15px;
  margin-top: 8px;
  text-align: center !important;
  line-height: 44px !important;
}
</style>
