<script setup>
import { computed, onMounted, provide, ref } from 'vue'
import { useRouter } from 'vue-router'
import { request } from '../api/client'
import { clearSession } from '../auth/session'
import { hasPerm } from '../auth/perm'

const user = ref(null)
const router = useRouter()

provide('authUser', user)

// 選單依登入者權限顯示；perm 為 null 代表登入就能看
const navItems = [
  { to: '/dashboard', label: '首頁', perm: null },
  { to: '/users', label: '前台使用者', perm: 'user:read' },
  { to: '/admin-users', label: '後台使用者', perm: 'admin_user:read' },
  { to: '/roles', label: '角色與權限', perm: 'admin_user:read' },
  { to: '/wallets', label: '錢包', perm: 'wallet:read' },
  { to: '/operation-logs', label: '操作日誌', perm: 'operation_log:read' },
  { to: '/me/password', label: '修改密碼', perm: null },
]

const visibleNavItems = computed(() =>
  navItems.filter((item) => !item.perm || hasPerm(user.value, item.perm)),
)

onMounted(async () => {
  try {
    user.value = await request('/me')
  } catch {
    clearSession()
    router.push('/login')
  }
})

async function onLogout() {
  try {
    await request('/logout', { method: 'POST' })
  } finally {
    clearSession()
    router.push('/login')
  }
}
</script>

<template>
  <div class="shell">
    <aside class="sidebar">
      <div class="brand">
        <span class="brand-mark">›_</span>
        <span>tgw-admin</span>
      </div>
      <nav class="nav">
        <router-link v-for="item in visibleNavItems" :key="item.to" :to="item.to">
          {{ item.label }}
        </router-link>
      </nav>
    </aside>
    <div class="main">
      <header class="topbar">
        <div />
        <div class="topbar-user">
          <span v-if="user">已登入：<strong>{{ user.username }}</strong></span>
          <button class="secondary" @click="onLogout">登出</button>
        </div>
      </header>
      <div class="content">
        <router-view />
      </div>
    </div>
  </div>
</template>
