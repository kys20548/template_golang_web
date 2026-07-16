<script setup>
import { ref } from 'vue'
import { useRouter } from 'vue-router'
import { request } from '../api/client'
import { setSession } from '../auth/session'

const username = ref('')
const password = ref('')
const error = ref('')
const loading = ref(false)
const router = useRouter()

async function onSubmit() {
  error.value = ''
  loading.value = true
  try {
    const data = await request('/login', {
      method: 'POST',
      body: JSON.stringify({ username: username.value, password: password.value }),
    })
    setSession(data.token, data.user)
    router.push('/dashboard')
  } catch (e) {
    error.value = e.message
  } finally {
    loading.value = false
  }
}
</script>

<template>
  <h1>登入</h1>
  <form @submit.prevent="onSubmit">
    <div>
      <label>帳號</label>
      <input v-model="username" required autocomplete="username" />
    </div>
    <div>
      <label>密碼</label>
      <input v-model="password" type="password" required autocomplete="current-password" />
    </div>
    <button type="submit" :disabled="loading">{{ loading ? '登入中…' : '登入' }}</button>
    <p v-if="error" role="alert">{{ error }}</p>
  </form>
</template>
