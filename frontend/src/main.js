import { createSSRApp } from 'vue'
import App from './App.vue'
import WebSidebar from './components/WebSidebar.vue'

export function createApp() {
  const app = createSSRApp(App)
  app.component('WebSidebar', WebSidebar)
  return { app }
}
