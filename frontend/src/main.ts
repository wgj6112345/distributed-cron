// frontend/src/main.ts
import { createApp } from 'vue'
import App from './App.vue'
import router from './router'

// 同时需引入 bootstrap 的 CSS 样式（可在 main.ts 或当前文件导入）
import 'bootstrap/dist/css/bootstrap.min.css';

const app = createApp(App)

app.use(router)

app.mount('#app')
