<template>
  <view class="page">
    <WebSidebar />
    <view class="page-head">
      <view class="back-row" @click="goBack">
        <text class="back-arrow">‹</text>
        <text class="back-text">返回</text>
      </view>
      <text class="eyebrow">CATEGORIES</text>
      <text class="page-title">分类管理</text>
      <text class="page-copy">一级负责归纳，二级负责记录得更具体</text>
    </view>

    <view class="kind-tabs">
      <view
        class="kind-tab"
        :class="{ active: kind === 'expense' }"
        @tap.stop="switchKind('expense')"
      >支出分类</view>
      <view
        class="kind-tab"
        :class="{ active: kind === 'income' }"
        @tap.stop="switchKind('income')"
      >收入分类</view>
    </view>

    <view class="category-list">
      <view class="category-card" v-for="group in visibleGroups" :key="group.id">
        <view class="parent-row" @click="toggle(group.id)">
          <view class="category-icon">{{ group.icon || '📦' }}</view>
          <view class="category-main">
            <text class="category-name">{{ group.name }}</text>
            <text class="category-meta">{{ (group.children || []).length }} 个二级分类</text>
          </view>
          <view class="parent-actions" @click.stop>
            <text @click.stop="openCreate(group)">+ 子类</text>
            <text @click.stop="openEdit(group)">编辑</text>
          </view>
          <text class="chevron" :class="{ open: expanded[group.id] }">›</text>
        </view>
        <view class="children" v-if="expanded[group.id]">
          <view class="child-row" v-for="child in group.children || []" :key="child.id">
            <view class="child-icon">{{ child.icon || group.icon || '·' }}</view>
            <text class="child-name">{{ child.name }}</text>
            <text class="child-edit" @click.stop="openEdit(child)">编辑</text>
          </view>
          <view class="child-empty" v-if="!(group.children || []).length" @click.stop="openCreate(group)">暂无二级分类，点击添加</view>
        </view>
      </view>
      <view class="empty" v-if="visibleGroups.length === 0">暂无{{ kind === 'expense' ? '支出' : '收入' }}分类</view>
    </view>

    <view class="add-button" @tap.stop="openCreate(null)">+ 新增{{ kind === 'expense' ? '支出' : '收入' }}一级分类</view>

    <view class="modal" v-if="showEditor" @tap="showEditor = false">
      <view class="modal-content" @tap.stop>
        <view class="modal-head">
          <text class="modal-title">{{ editing ? '编辑分类' : (parent ? '新增二级分类' : '新增一级分类') }}</text>
          <text class="modal-close" @click="showEditor = false">×</text>
        </view>
        <text class="parent-tip" v-if="parent">归属于 {{ parent.icon }} {{ parent.name }}</text>
        <text class="kind-tip" v-if="!editing && !parent">将添加到「{{ kind === 'expense' ? '支出' : '收入' }}」分类</text>
        <text class="field-label">分类名称</text>
        <input class="category-input" v-model="form.name" placeholder="输入分类名称" maxlength="64" />
        <text class="field-label">选择图标</text>
        <view class="icon-picker">
          <text v-for="icon in icons" :key="icon" :class="{ active: form.icon === icon }" @click="form.icon = icon">{{ icon }}</text>
        </view>
        <text class="delete-link" v-if="editing" @click="remove">删除这个分类</text>
        <view class="modal-actions">
          <button class="btn btn-secondary" @click="showEditor = false">取消</button>
          <button class="btn btn-primary" @click="save" :loading="saving">保存</button>
        </view>
      </view>
    </view>
  </view>
</template>

<script>
import { get, post, put, del } from '../../common/request.js'
import { goBackOrProfile } from '../../common/nav.js'

export default {
  data() {
    return {
      groups: { expense: [], income: [] },
      kind: 'expense',
      expanded: {},
      showEditor: false,
      editing: null,
      parent: null,
      saving: false,
      form: { name: '', icon: '📦' },
      icons: ['🍽️','🥐','🍱','🍜','🥤','🚗','🚇','🚕','🛒','🧴','👕','💻','🛍️','🏠','🔑','🎮','🎬','✈️','💊','📚','📱','🎁','💰','📈','💼','🧧','📦','💡','🔧','🩺']
    }
  },
  computed: { visibleGroups() { return this.groups[this.kind] || [] } },
  onShow() { this.load() },
  methods: {
    goBack() { goBackOrProfile() },
    async load() {
      try {
        const [expense, income] = await Promise.all([get('/categories/tree', { kind: 'expense' }), get('/categories/tree', { kind: 'income' })])
        this.groups.expense = (expense || []).map(item => ({ ...item, children: item.children || [] }))
        this.groups.income = (income || []).map(item => ({ ...item, children: item.children || [] }))
      } catch (error) { console.error(error) }
    },
    switchKind(kind) {
      if (kind !== 'expense' && kind !== 'income') return
      // 仅切换列表类型，绝不打开新增弹层
      this.showEditor = false
      this.editing = null
      this.parent = null
      this.kind = kind
      this.expanded = {}
    },
    toggle(id) { this.expanded = { ...this.expanded, [id]: !this.expanded[id] } },
    openCreate(parent) {
      this.editing = null
      this.parent = parent
      this.form = { name: '', icon: parent?.icon || '📦' }
      if (parent) this.expanded = { ...this.expanded, [parent.id]: true }
      this.showEditor = true
      this.fixInput()
    },
    openEdit(category) {
      this.editing = category
      this.parent = category.parent_id ? this.visibleGroups.find(group => group.id === category.parent_id) : null
      this.form = { name: category.name, icon: category.icon || this.parent?.icon || '📦' }
      this.showEditor = true
      this.fixInput()
    },
    fixInput() {
      this.$nextTick(() => setTimeout(() => {
        // #ifdef H5
        document.querySelectorAll('.category-input input').forEach(input => { input.style.height = '24px'; input.style.lineHeight = '24px' })
        // #endif
      }, 50))
    },
    async save() {
      const name = this.form.name.trim()
      if (!name) return uni.showToast({ title: '请输入分类名称', icon: 'none' })
      this.saving = true
      try {
        if (this.editing) await put(`/categories/${this.editing.id}`, { name, icon: this.form.icon, sort: this.editing.sort || 0 })
        else await post('/categories', { name, icon: this.form.icon, kind: this.kind, parent_id: this.parent?.id || null, sort: this.parent ? (this.parent.children?.length || 0) + 1 : this.visibleGroups.length + 1 })
        this.showEditor = false
        uni.showToast({ title: '已保存', icon: 'success' })
        await this.load()
      } catch (error) { uni.showToast({ title: error.message || '保存失败', icon: 'none' }) }
      finally { this.saving = false }
    },
    remove() {
      uni.showModal({ title: '删除分类', content: `确定删除「${this.editing.name}」吗？存在子分类或流水时不能删除。`, success: async res => {
        if (!res.confirm) return
        try { await del(`/categories/${this.editing.id}`); this.showEditor = false; await this.load() }
        catch (error) { uni.showToast({ title: error.message || '删除失败', icon: 'none' }) }
      } })
    }
  }
}
</script>

<style scoped>
.page { min-height: 100vh; padding: 22px 16px 110px; background: #f7f9f7; box-sizing: border-box; }
.page-head { padding: 0 4px 20px; }
.back-row {
  display: inline-flex; align-items: center; gap: 2px; margin-bottom: 10px;
  color: #36b88f; font-size: 14px; font-weight: 600; cursor: pointer;
}
.back-arrow { font-size: 22px; line-height: 1; }
.eyebrow { display: block; color: #36b88f; font-size: 10px; font-weight: 700; letter-spacing: 1.8px; }
.page-title { display: block; margin-top: 5px; color: #18312b; font-size: 27px; font-weight: 700; }
.page-copy { display: block; margin-top: 6px; color: #87968f; font-size: 13px; }
.kind-tabs {
  position: relative;
  z-index: 5;
  margin-bottom: 17px;
  padding: 4px;
  display: flex;
  background: #e9efec;
  border-radius: 15px;
}
.kind-tab {
  flex: 1;
  padding: 10px 0;
  color: #7f8e88;
  border-radius: 12px;
  text-align: center;
  font-size: 13px;
  font-weight: 600;
  cursor: pointer;
  user-select: none;
}
.kind-tab.active { color: #2cae86; background: #fff; box-shadow: 0 4px 12px rgba(24,49,43,.06); }
.category-list { display: flex; flex-direction: column; gap: 10px; position: relative; z-index: 1; }
.category-card { overflow: hidden; background: #fff; border: 1px solid #edf1ee; border-radius: 18px; box-shadow: 0 7px 24px rgba(24,49,43,.035); }
.parent-row { min-height: 70px; padding: 11px 14px; display: flex; align-items: center; gap: 11px; box-sizing: border-box; }
.category-icon { width: 42px; height: 42px; flex: 0 0 42px; display: flex; align-items: center; justify-content: center; background: #f0f8f5; border-radius: 14px; font-size: 21px; }
.category-main { flex: 1; min-width: 0; }
.category-name { display: block; color: #263f38; font-size: 14px; font-weight: 650; }
.category-meta { display: block; margin-top: 3px; color: #92a19b; font-size: 10px; }
.parent-actions { display: flex; gap: 9px; color: #36b88f; font-size: 11px; }
.chevron { color: #aab5b0; font-size: 21px; transition: transform .2s; }
.chevron.open { transform: rotate(90deg); }
.children { padding: 0 14px 8px 67px; background: #fbfcfb; border-top: 1px solid #edf1ee; }
.child-row { min-height: 47px; display: flex; align-items: center; gap: 9px; border-bottom: 1px solid #edf1ee; }
.child-row:last-child { border-bottom: 0; }
.child-icon { width: 28px; height: 28px; display: flex; align-items: center; justify-content: center; background: #fff; border-radius: 9px; font-size: 15px; }
.child-name { flex: 1; color: #526860; font-size: 12px; }
.child-edit { color: #36b88f; font-size: 11px; }
.child-empty { padding: 17px 0; color: #92a19b; font-size: 11px; }
.empty { padding: 50px 20px; color: #92a19b; text-align: center; }
.add-button {
  position: fixed; z-index: 20; left: 16px; right: 16px; bottom: calc(20px + env(safe-area-inset-bottom));
  height: 50px; display: flex; align-items: center; justify-content: center;
  color: #fff; background: #42c39c; border-radius: 16px; box-shadow: 0 10px 24px rgba(66,195,156,.24);
  font-size: 15px; font-weight: 600;
}
.modal { position: fixed; inset: 0; z-index: 999; padding: 20px; display: flex; align-items: center; justify-content: center; background: rgba(24,49,43,.34); box-sizing: border-box; }
.modal-content { width: 100%; max-width: 430px; max-height: 82vh; padding: 22px; overflow-y: auto; background: #fff; border-radius: 22px; box-sizing: border-box; }
.modal-head { display: flex; justify-content: space-between; align-items: center; }
.modal-title { color: #18312b; font-size: 18px; font-weight: 700; }
.modal-close { color: #92a19b; font-size: 25px; }
.parent-tip,.kind-tip { display: block; margin-top: 8px; color: #36b88f; font-size: 11px; }
.field-label { display: block; margin: 16px 0 7px; color: #526860; font-size: 12px; font-weight: 600; }
.category-input { width: 100%; height: 48px; padding: 0 13px; border: 1px solid #e1e9e5; border-radius: 13px; background: #fbfcfb; box-sizing: border-box; }
:deep(.category-input .uni-input-wrapper) { height: 48px; display: flex; align-items: center; }
:deep(.category-input input) { padding: 0 8px !important; box-sizing: border-box; }
.icon-picker { display: grid; grid-template-columns: repeat(7,1fr); gap: 7px; }
.icon-picker text { aspect-ratio: 1; display: flex; align-items: center; justify-content: center; border: 1px solid #e1e9e5; border-radius: 10px; background: #fbfcfb; font-size: 17px; }
.icon-picker text.active { border-color: #42c39c; background: #e8f7f1; }
.delete-link { display: block; margin-top: 17px; color: #f06f64; font-size: 11px; }
.modal-actions { margin-top: 18px; display: flex; gap: 10px; }
.modal-actions .btn { flex: 1; width: auto !important; margin: 0; }
@media (min-width: 900px) {
  .page { max-width: 760px !important; }
  .category-list { display: grid; grid-template-columns: 1fr 1fr; align-items: start; }
  .add-button { position: static; width: 100%; margin-top: 18px; }
}
</style>
