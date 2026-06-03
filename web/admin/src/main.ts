import { createApp } from 'vue'
import { createPinia } from 'pinia'
import ElementPlus from 'element-plus'
import * as ElementPlusIconsVue from '@element-plus/icons-vue'
import 'element-plus/dist/index.css'
import './styles/global.css'

import App from './App.vue'
import ScenarioStepTreeNode from './views/scenarios/ScenarioStepTreeNode.vue'
import router from './router'
import './permission'
import { initializeAppearance } from './utils/appearance'

// 请求错误已在 axios 拦截器里提示；未显式 catch 的 await 仍会产生未处理的 Promise 拒绝，这里避免控制台刷屏
if (typeof window !== 'undefined') {
  window.addEventListener('unhandledrejection', (event) => {
    const r = event.reason
    if (r && (r.name === 'AxiosError' || r.isAxiosError)) {
      event.preventDefault()
    }
  })
}

initializeAppearance()

const app = createApp(App)
const pinia = createPinia()

app.component('ScenarioStepTreeNode', ScenarioStepTreeNode)

for (const [key, component] of Object.entries(ElementPlusIconsVue)) {
  app.component(key, component)
}

app.use(pinia)
app.use(router)
app.use(ElementPlus)
app.mount('#app')
