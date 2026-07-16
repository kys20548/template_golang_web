import { createRouter, createWebHistory } from 'vue-router'
import { getToken } from '../auth/session'
import AppShell from '../components/AppShell.vue'
import LoginView from '../views/LoginView.vue'
import DashboardView from '../views/DashboardView.vue'
import UsersView from '../views/UsersView.vue'
import WalletView from '../views/WalletView.vue'
import OperationLogsView from '../views/OperationLogsView.vue'
import ChangePasswordView from '../views/ChangePasswordView.vue'

const router = createRouter({
  history: createWebHistory(),
  routes: [
    { path: '/login', component: LoginView },
    {
      path: '/',
      component: AppShell,
      meta: { requiresAuth: true },
      children: [
        { path: '', redirect: 'dashboard' },
        { path: 'dashboard', component: DashboardView },
        { path: 'users', component: UsersView },
        { path: 'wallet', component: WalletView },
        { path: 'operation-logs', component: OperationLogsView },
        { path: 'me/password', component: ChangePasswordView },
      ],
    },
  ],
})

router.beforeEach((to) => {
  if (to.meta.requiresAuth && !getToken()) {
    return '/login'
  }
})

export default router
