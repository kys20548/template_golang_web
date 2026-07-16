<script setup>
import { onMounted, ref } from 'vue'
import { useRouter } from 'vue-router'
import { request } from '../api/client'
import { clearSession } from '../auth/session'

const user = ref(null)
const error = ref('')
const router = useRouter()

onMounted(async () => {
  try {
    user.value = await request('/me')
  } catch (e) {
    error.value = e.message
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
  <h1>後台首頁</h1>
  <p v-if="error" role="alert">{{ error }}</p>
  <p v-else-if="user">已登入：{{ user.username }}</p>
  <p v-else>載入中…</p>
  <button @click="onLogout">登出</button>
</template>
