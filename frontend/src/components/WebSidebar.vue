<template>
  <!-- #ifdef H5 -->
  <view class="web-sidebar">
    <view class="sidebar-brand">
      <image class="brand-mark" src="/static/brand/zhangxiaoya-duck-app-icon.png" mode="aspectFill" />
      <view>
        <text class="brand-name">账小鸭</text>
        <text class="brand-copy">小鸭记账，心中有数</text>
      </view>
    </view>

    <view class="sidebar-nav">
      <view
        v-for="item in items"
        :key="item.path"
        class="sidebar-item"
        :class="{ active: currentPath === item.path }"
        @click="go(item.path)"
      >
        <image :src="currentPath === item.path ? item.activeIcon : item.icon" mode="aspectFit" />
        <text>{{ item.label }}</text>
      </view>
    </view>

    <view class="sidebar-foot">
      <text class="foot-dot"></text>
      <text>个人账本默认启用</text>
    </view>
  </view>
  <!-- #endif -->
</template>

<script>
export default {
  data() {
    return {
      currentPath: '',
      items: [
        { path: 'pages/chat/index', label: 'AI 记账', icon: '/static/tab/chat-v2.png', activeIcon: '/static/tab/chat-active-v2.png' },
        { path: 'pages/record/index', label: '记一笔', icon: '/static/tab/record-v2.png', activeIcon: '/static/tab/record-active-v2.png' },
        { path: 'pages/list/index', label: '明细', icon: '/static/tab/list-v2.png', activeIcon: '/static/tab/list-active-v2.png' },
        { path: 'pages/report/index', label: '报表', icon: '/static/tab/report-v2.png', activeIcon: '/static/tab/report-active-v2.png' },
        { path: 'pages/profile/index', label: '我的', icon: '/static/tab/profile-v2.png', activeIcon: '/static/tab/profile-active-v2.png' }
      ]
    }
  },
  mounted() {
    const pages = getCurrentPages()
    this.currentPath = pages.length ? pages[pages.length - 1].route : ''
    if (this.currentPath === 'pages/account/index' || this.currentPath === 'pages/category/index' || this.currentPath === 'pages/ledger/index') {
      this.currentPath = 'pages/profile/index'
    }
  },
  methods: {
    go(path) {
      if (path === this.currentPath) return
      uni.switchTab({ url: `/${path}` })
    }
  }
}
</script>

<style scoped>
.web-sidebar { display: none; }

@media (min-width: 900px) {
  .web-sidebar {
    position: fixed;
    inset: 0 auto 0 0;
    z-index: 1000;
    width: 220px;
    padding: 30px 18px 22px;
    display: flex;
    flex-direction: column;
    color: #526860;
    background: rgba(255,255,255,.96);
    border-right: 1px solid #e8eeeb;
    box-shadow: 12px 0 36px rgba(24,49,43,.035);
    box-sizing: border-box;
  }
  .sidebar-brand { display: flex; align-items: center; gap: 11px; padding: 0 8px 28px; }
  .brand-mark {
    width: 42px; height: 42px; flex: 0 0 42px; border-radius: 15px;
    display: block; background: #f7f9f7; box-shadow: 0 8px 18px rgba(66,195,156,.2);
  }
  .brand-name { display: block; color: #18312b; font-size: 18px; font-weight: 750; }
  .brand-copy { display: block; margin-top: 2px; color: #92a19b; font-size: 10px; }
  .sidebar-nav { display: flex; flex-direction: column; gap: 7px; }
  .sidebar-item {
    height: 48px; padding: 0 13px; display: flex; align-items: center; gap: 12px;
    border-radius: 14px; color: #71817a; font-size: 14px; font-weight: 600; cursor: pointer;
  }
  .sidebar-item image { width: 22px; height: 22px; }
  .sidebar-item.active { color: #2cae86; background: #eaf7f2; }
  .sidebar-item:hover { background: #f2f7f5; }
  .sidebar-foot {
    margin-top: auto; padding: 12px; display: flex; align-items: center; gap: 7px;
    color: #92a19b; font-size: 10px; background: #f7faf8; border-radius: 12px;
  }
  .foot-dot { width: 7px; height: 7px; border-radius: 50%; background: #42c39c; }
}
</style>
