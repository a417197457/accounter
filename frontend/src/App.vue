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
page, uni-page-body {
  height: 100%;
  margin: 0;
  padding: 0;
  color: #18312b;
  font-family: -apple-system, BlinkMacSystemFont, "PingFang SC", "Microsoft YaHei", "Segoe UI", sans-serif;
  background-color: #f7f9f7;
  -webkit-font-smoothing: antialiased;
}

/* H5 全局修复 */
/* #ifdef H5 */
body, uni-page-body {
  overflow-x: hidden;
  background: #f7f9f7;
}

@media (min-width: 900px) {
  uni-page-head,
  uni-tabbar { display: none !important; }
  uni-page-wrapper {
    padding-left: 220px;
    background: #f3f7f5;
    box-sizing: border-box;
  }
  uni-page-body > .page {
    width: 100%;
    max-width: 880px;
    margin: 0 auto;
    padding: 34px 28px 48px;
    box-sizing: border-box;
  }
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

/* 按钮文字居中；默认不要撑满整行（全宽按钮在各页自行设 width:100%） */
button, .btn, uni-button {
  text-align: center !important;
  box-sizing: border-box;
}
uni-button {
  width: auto;
  margin: 0;
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
  border: 1px solid #edf1ee;
  border-radius: 18px;
  padding: 18px;
  margin-bottom: 14px;
  box-shadow: 0 8px 28px rgba(24, 49, 43, 0.045);
}

/* 按钮样式 */
.btn {
  min-height: 44px;
  padding: 10px 20px;
  border-radius: 14px;
  font-size: 15px;
  font-weight: 600;
  text-align: center !important;
  cursor: pointer;
  border: none;
  transition: opacity 0.18s, transform 0.18s;
  box-sizing: border-box;
}

.btn::after { border: none; }

.btn:active {
  opacity: 0.8;
  transform: scale(0.985);
}
.btn[disabled],
button[disabled] {
  opacity: .55;
  box-shadow: none;
  transform: none;
}

.btn-primary {
  background: #42c39c;
  color: #fff;
  box-shadow: 0 8px 18px rgba(66, 195, 156, 0.2);
}

.btn-secondary {
  background: #eef5f2;
  color: #31564c;
}

/* 输入框样式 */
.input {
  width: 100%;
  padding: 13px 14px;
  border: 1px solid #e1e9e5;
  border-radius: 13px;
  font-size: 16px;
  box-sizing: border-box;
  min-width: 0;
  background: #fbfcfb;
}

.input:focus {
  outline: none;
  border-color: #42c39c;
}
</style>
