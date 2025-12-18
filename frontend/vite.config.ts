import { defineConfig } from 'vite'
import vue from '@vitejs/plugin-vue' // 引入插件



export default defineConfig({
//   // 配置路径别名（解决 @ 指向 src 的问题）
    plugins: [vue()] // 注册插件
})