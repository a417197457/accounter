<template>
  <view class="page">
    <view class="card">
      <text class="title">记一笔</text>

      <!-- 账户选择 -->
      <view class="field">
        <text class="label">账户</text>
        <view class="select-btn" @click="showAccountModal">
          <text>{{ selectedAccount?.name || '选择账户' }}</text>
        </view>
      </view>

      <!-- 分类选择 -->
      <view class="field">
        <text class="label">分类</text>
        <view class="select-btn" @click="showCategoryModal">
          <text>{{ selectedCategory?.name || '选择分类' }}</text>
        </view>
      </view>

      <!-- 金额输入 -->
      <view class="field">
        <text class="label">金额</text>
        <input
          class="record-input"
          type="digit"
          v-model="form.amount"
          placeholder="0.00"
          :style="inputStyle"
        />
      </view>

      <!-- 日期选择 -->
      <view class="field">
        <text class="label">日期</text>
        <picker mode="date" :value="form.date" @change="onDateChange">
          <view class="select-btn">
            <text>{{ form.date }}</text>
          </view>
        </picker>
      </view>

      <!-- 备注 -->
      <view class="field">
        <text class="label">备注（可选）</text>
        <input
          class="record-input"
          type="text"
          v-model="form.remark"
          placeholder="添加备注"
          :style="inputStyle"
        />
      </view>

      <!-- 提交按钮 -->
      <button class="btn btn-primary submit-btn" @click="handleSubmit" :loading="loading">
        保存
      </button>
    </view>

    <!-- 账户选择弹层 -->
    <view class="modal" v-if="showAccount" @click.self="showAccount = false">
      <view class="modal-content">
        <text class="modal-title">选择账户</text>
        <view class="list">
          <view
            class="list-item"
            v-for="acc in accounts"
            :key="acc.id"
            @click="selectAccount(acc)"
          >
            <text>{{ acc.name }}</text>
            <text class="balance">余额：{{ acc.balance }}</text>
          </view>
        </view>
        <button class="btn btn-secondary" @click="showAccount = false">取消</button>
      </view>
    </view>

    <!-- 分类选择弹层 -->
    <view class="modal" v-if="showCategory" @click.self="showCategory = false">
      <view class="modal-content">
        <text class="modal-title">选择分类</text>
        <view class="category-tabs">
          <text
            class="tab"
            :class="{ active: tab === 'expense' }"
            @click="tab = 'expense'"
          >支出</text>
          <text
            class="tab"
            :class="{ active: tab === 'income' }"
            @click="tab = 'income'"
          >收入</text>
        </view>
        <view class="list">
          <view
            class="list-item"
            v-for="cat in filteredCategories"
            :key="cat.id"
            @click="selectCategory(cat)"
          >
            <text>{{ cat.icon }} {{ cat.name }}</text>
          </view>
        </view>
        <button class="btn btn-secondary" @click="showCategory = false">取消</button>
      </view>
    </view>
  </view>
</template>

<script>
import { get, post } from '../../common/request.js'
import { checkLogin } from '../../common/auth.js'

export default {
  data() {
    return {
      form: {
        account_id: null,
        category_id: null,
        op_type: 'expense',
        amount: '',
        remark: '',
        date: new Date().toISOString().split('T')[0]
      },
      loading: false,
      selectedAccount: null,
      selectedCategory: null,
      accounts: [],
      categories: { expense: [], income: [] },
      tab: 'expense',
      showAccount: false,
      showCategory: false,
      inputStyle: {
        'height': '24px',
        'line-height': '24px',
        'user-select': 'auto',
        '-webkit-user-select': 'auto',
        'pointer-events': 'auto',
        'cursor': 'text'
      }
    }
  },
  computed: {
    filteredCategories() {
      return this.categories[this.tab] || []
    }
  },
  onLoad() {
    checkLogin()
  },
  onShow() {
    this.loadData()
  },
  mounted() {
    this.$nextTick(() => {
      setTimeout(() => {
        const inputs = document.querySelectorAll('.record-input input')
        inputs.forEach(input => {
          input.style.height = '24px'
          input.style.lineHeight = '24px'
          input.style.padding = '0'
          input.style.margin = '0'
          input.style.border = 'none'
          input.style.outline = 'none'
          input.style.background = 'transparent'
          input.style.width = '100%'
          input.style.display = 'block'
          input.style.userSelect = 'auto'
          input.style.webkitUserSelect = 'auto'
        })
      }, 100)
    })
  },
  methods: {
    async loadData() {
      try {
        const accounts = await get('/accounts')
        this.accounts = accounts || []
        if (this.accounts.length > 0 && !this.selectedAccount) {
          this.selectedAccount = this.accounts[0]
          this.form.account_id = this.accounts[0].id
        }

        const expenseCats = await get('/categories/tree', { kind: 'expense' })
        const incomeCats = await get('/categories/tree', { kind: 'income' })
        this.categories.expense = expenseCats || []
        this.categories.income = incomeCats || []
      } catch (err) {
        if (err.message && err.message.includes('未登录')) {
          uni.redirectTo({ url: '/pages/login/index' })
        }
      }
    },
    showAccountModal() {
      this.showAccount = true
    },
    selectAccount(acc) {
      this.selectedAccount = acc
      this.form.account_id = acc.id
      this.showAccount = false
    },
    showCategoryModal() {
      this.showCategory = true
    },
    selectCategory(cat) {
      this.selectedCategory = cat
      this.form.category_id = cat.id
      this.form.op_type = cat.kind
      this.showCategory = false
    },
    onDateChange(e) {
      this.form.date = e.detail.value
    },
    async handleSubmit() {
      if (!this.form.account_id) {
        uni.showToast({ title: '请选择账户', icon: 'none' })
        return
      }
      if (!this.form.category_id) {
        uni.showToast({ title: '请选择分类', icon: 'none' })
        return
      }
      if (!this.form.amount || parseFloat(this.form.amount) <= 0) {
        uni.showToast({ title: '请输入金额', icon: 'none' })
        return
      }

      this.loading = true
      try {
        const data = {
          account_id: this.form.account_id,
          category_id: this.form.category_id,
          op_type: this.form.op_type,
          amount: this.form.amount,
          remark: this.form.remark,
          occurred_at: this.form.date + ' 12:00:00'
        }
        await post('/transactions', data)
        uni.showToast({ title: '记账成功', icon: 'success' })
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
  padding: 16px;
  min-height: 100vh;
  background: #f5f5f5;
}
.card {
  background: #fff;
  border-radius: 12px;
  padding: 24px;
}
.title {
  font-size: 20px;
  font-weight: bold;
  margin-bottom: 20px;
  display: block;
  text-align: center;
}
.field {
  margin-bottom: 16px;
}
.label {
  font-size: 14px;
  color: #666;
  margin-bottom: 8px;
  display: block;
}
.select-btn {
  padding: 12px;
  border: 1px solid #ddd;
  border-radius: 8px;
  font-size: 16px;
  background: #fff;
}
.record-input {
  display: block;
  width: 100%;
  box-sizing: border-box;
  background: #fff;
}
:deep(.record-input) {
  padding: 0 !important;
  border: none !important;
}
:deep(.record-input .uni-input-wrapper) {
  width: 100%;
  padding: 0 !important;
  border: 1px solid #ddd !important;
  border-radius: 8px !important;
  background: #fff;
  box-sizing: border-box;
}
:deep(.record-input input) {
  width: 100% !important;
  height: 24px !important;
  line-height: 24px !important;
  padding: 0 !important;
  margin: 0 !important;
  border: none !important;
  font-size: 16px !important;
  outline: none !important;
  background: transparent !important;
  user-select: auto !important;
  -webkit-user-select: auto !important;
}
.submit-btn {
  margin-top: 8px;
  width: 100%;
  height: 48px;
  font-size: 16px;
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
  border: 1px solid #ddd;
  border-radius: 8px;
  cursor: pointer;
}
.tab.active {
  background: #007aff;
  color: #fff;
  border-color: #007aff;
}
.list {
  max-height: 400px;
  overflow-y: auto;
}
.list-item {
  display: flex;
  justify-content: space-between;
  padding: 12px;
  border-bottom: 1px solid #f0f0f0;
  cursor: pointer;
}
.list-item:active {
  background: #f5f5f5;
}
.balance {
  color: #999;
  font-size: 13px;
}
</style>
