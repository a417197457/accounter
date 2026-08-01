<script>
export default {
  onLaunch() {
    console.log('App Launch')
  },
  onShow() {
    console.log('App Show')
  },
  onHide() {
    console.log('App Hide')
  }
}
</script>

<style>
/* 全局样式 */
page, uni-page-body {
  height: 100%;
  margin: 0;
  padding: 0;
  font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, 'Helvetica Neue', Arial, sans-serif;
  background-color: #f5f5f5;
}

/* H5 全局修复 */
/* #ifdef H5 */
body, uni-page-body {
  overflow-x: hidden;
}

/* 导航栏标题去除文本光标 */
uni-page-head * {
  user-select: none !important;
  caret-color: transparent !important;
  cursor: default !important;
}

/* 确保 input 可交互 */
uni-input input,
uni-textarea textarea {
  user-select: auto !important;
  -webkit-user-select: auto !important;
}

/* 按钮文字居中 */
button, .btn, uni-button {
  text-align: center !important;
}
uni-button::v-deep {
  text-align: center !important;
}
/* #endif */

/* 通用工具类 */
.flex { display: flex; }
.flex-col { display: flex; flex-direction: column; }
.flex-1 { flex: 1; min-width: 0; }
.items-center { align-items: center; }
.justify-center { justify-content: center; }
.justify-between { justify-content: space-between; }

/* 卡片样式 */
.card {
  background: #fff;
  border-radius: 12px;
  padding: 16px;
  margin-bottom: 12px;
  box-shadow: 0 2px 8px rgba(0, 0, 0, 0.05);
}

/* 按钮样式 */
.btn {
  padding: 12px 24px;
  border-radius: 8px;
  font-size: 16px;
  font-weight: 500;
  text-align: center !important;
  cursor: pointer;
  border: none;
  transition: opacity 0.2s;
}

.btn:active {
  opacity: 0.8;
}

.btn-primary {
  background: #007aff;
  color: #fff;
}

.btn-secondary {
  background: #f0f0f0;
  color: #333;
}

/* 输入框样式 */
.input {
  width: 100%;
  padding: 12px;
  border: 1px solid #ddd;
  border-radius: 8px;
  font-size: 16px;
  box-sizing: border-box;
  min-width: 0; /* 修复 flex 子项 input 撑破容器 */
}

.input:focus {
  outline: none;
  border-color: #007aff;
}
</style>
