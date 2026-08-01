<template>
  <view class="page">
    <!-- 账本切换 -->
    <view class="ledger-tabs">
      <view
        class="tab"
        :class="{ active: currentLedger?.id === ledger.id }"
        v-for="ledger in ledgers"
        :key="ledger.id"
        @click="switchLedger(ledger)"
      >
        <text>{{ ledger.name }}</text>
      </view>
    </view>

    <!-- 流水列表 -->
    <view v-if="groupedTransactions.length === 0" class="empty">
      <text>暂无流水</text>
    </view>

    <view v-else class="month-groups">
      <view v-for="group in groupedTransactions" :key="group.month" class="month-group">
        <!-- 月汇总 -->
        <view class="month-header">
          <text class="month-title">{{ group.month }}</text>
          <text class="month-summary">
            支出 {{ group.expenseTotal }} · 收入 {{ group.incomeTotal }}
          </text>
        </view>

        <!-- 流水列表 -->
        <view class="transaction-list">
          <view
            class="transaction-item"
            v-for="tx in group.transactions"
            :key="tx.id"
          >
            <view class="tx-main">
              <text class="tx-category">
                {{ tx.category?.icon || '' }} {{ tx.category?.name || '未分类' }}
              </text>
              <text class="tx-remark" v-if="tx.remark">{{ tx.remark }}</text>
              <text class="tx-meta">
                {{ tx.account?.name }} · {{ formatDate(tx.occurred_at) }}
              </text>
            </view>
            <view class="tx-amount" :class="tx.op_type">
              <text>{{ formatAmount(tx) }}</text>
            </view>
          </view>
        </view>
      </view>
    </view>
  </view>
</template>

<script>
import { get } from '../../common/request.js'
import { checkLogin } from '../../common/auth.js'

export default {
  data() {
    return {
      ledgers: [],
      currentLedger: null,
      transactions: [],
      groupedTransactions: []
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
        // 默认选中个人账本
        if (this.ledgers.length > 0) {
          this.currentLedger = this.ledgers.find(l => l.is_personal) || this.ledgers[0]
          await this.loadTransactions()
        }
      } catch (err) {
        if (err.message && err.message.includes('未登录')) {
          uni.redirectTo({ url: '/pages/login/index' })
        }
      }
    },
    async switchLedger(ledger) {
      this.currentLedger = ledger
      await this.loadTransactions()
    },
    async loadTransactions() {
      if (!this.currentLedger) return

      try {
        const result = await get('/transactions', {
          page: 1,
          page_size: 100
        }, { ledgerId: this.currentLedger.id })

        const list = result?.list || []
        this.transactions = list
        this.groupTransactions(list)
      } catch (err) {
        console.error('加载流水失败:', err)
      }
    },
    groupTransactions(list) {
      const groups = {}
      let totalExpense = 0
      let totalIncome = 0

      for (const tx of list) {
        const month = tx.occurred_at.substring(0, 7) // YYYY-MM
        if (!groups[month]) {
          groups[month] = {
            month,
            transactions: [],
            expenseTotal: 0,
            incomeTotal: 0
          }
        }
        groups[month].transactions.push(tx)

        if (tx.op_type === 'expense') {
          groups[month].expenseTotal += parseFloat(tx.amount)
          totalExpense += parseFloat(tx.amount)
        } else if (tx.op_type === 'income') {
          groups[month].incomeTotal += parseFloat(tx.amount)
          totalIncome += parseFloat(tx.amount)
        }
      }

      // 按月份倒序
      this.groupedTransactions = Object.values(groups).sort((a, b) => {
        return b.month.localeCompare(a.month)
      })

      // 保留两位小数
      this.groupedTransactions.forEach(g => {
        g.expenseTotal = g.expenseTotal.toFixed(2)
        g.incomeTotal = g.incomeTotal.toFixed(2)
      })
    },
    formatAmount(tx) {
      const amount = parseFloat(tx.amount).toFixed(2)
      if (tx.op_type === 'expense' || tx.op_type === 'borrow_out' || tx.op_type === 'transfer') {
        return `-${amount}`
      }
      return `+${amount}`
    },
    formatDate(dateStr) {
      const date = new Date(dateStr)
      return `${date.getMonth() + 1}/${date.getDate()}`
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
.ledger-tabs {
  display: flex;
  gap: 8px;
  margin-bottom: 16px;
  overflow-x: auto;
}
.tab {
  padding: 8px 16px;
  border-radius: 20px;
  background: #fff;
  font-size: 14px;
  white-space: nowrap;
}
.tab.active {
  background: #007aff;
  color: #fff;
}
.empty {
  text-align: center;
  padding: 60px 20px;
  color: #999;
}
.month-groups {
  display: flex;
  flex-direction: column;
  gap: 16px;
}
.month-group {
  background: #fff;
  border-radius: 12px;
  overflow: hidden;
}
.month-header {
  padding: 12px 16px;
  background: #fafafa;
  border-bottom: 1px solid #f0f0f0;
  display: flex;
  justify-content: space-between;
  align-items: center;
}
.month-title {
  font-size: 14px;
  font-weight: bold;
}
.month-summary {
  font-size: 12px;
  color: #666;
}
.transaction-list {
  padding: 0 16px;
}
.transaction-item {
  display: flex;
  justify-content: space-between;
  align-items: center;
  padding: 12px 0;
  border-bottom: 1px solid #f0f0f0;
}
.transaction-item:last-child {
  border-bottom: none;
}
.tx-main {
  flex: 1;
  min-width: 0;
}
.tx-category {
  font-size: 15px;
  font-weight: 500;
  margin-bottom: 4px;
  display: block;
}
.tx-remark {
  font-size: 13px;
  color: #666;
  margin-bottom: 4px;
  display: block;
}
.tx-meta {
  font-size: 12px;
  color: #999;
}
.tx-amount {
  font-size: 16px;
  font-weight: bold;
  margin-left: 12px;
}
.tx-amount.expense,
.tx-amount.borrow_out,
.tx-amount.transfer {
  color: #ff3b30;
}
.tx-amount.income,
.tx-amount.borrow_in {
  color: #34c759;
}
</style>
