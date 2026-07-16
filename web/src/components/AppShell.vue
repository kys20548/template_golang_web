<script setup>
import { onMounted, provide, ref } from 'vue'
import { useRouter } from 'vue-router'
import { request } from '../api/client'
import { clearSession } from '../auth/session'

const user = ref(null)
const router = useRouter()

provide('authUser', user)

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
        <router-link to="/dashboard">首頁</router-link>
        <router-link to="/users">使用者</router-link>
        <router-link to="/wallet">我的錢包</router-link>
        <router-link to="/operation-logs">操作日誌</router-link>
        <router-link to="/me/password">修改密碼</router-link>
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
