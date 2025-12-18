// frontend/src/router/index.ts
import { createRouter, createWebHistory } from 'vue-router'
import HomeView from '../views/HomeView.vue'

const router = createRouter({
  history: createWebHistory(import.meta.env.BASE_URL),
  routes: [
    {
      path: '/',
      name: 'home',
      component: HomeView
    },
    {
      path: '/job/:jobName/history',
      name: 'job-history',
      component: () => import('../views/HistoryView.vue'),
      props: true // This allows the :jobName param to be passed as a prop
    }
  ]
})

export default router
