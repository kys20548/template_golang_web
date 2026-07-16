<script setup>
import { ref } from 'vue'
import { useRouter } from 'vue-router'
import { request } from '../api/client'
import { clearSession } from '../auth/session'

const oldPassword = ref('')
const newPassword = ref('')
const error = ref('')
const loading = ref(false)
const router = useRouter()

async function onSubmit() {
  error.value = ''
  loading.value = true
  try {
    await request('/me/password', {
      method: 'PUT',
      body: JSON.stringify({ old_password: oldPassword.value, new_password: newPassword.value }),
    })
    clearSession()
    router.push({ path: '/login', query: { passwordChanged: '1' } })
  } catch (e) {
    error.value = e.message
  } finally {
    loading.value = false
  }
}
</script>

<template>
  <h1>修改密碼</h1>
  <p class="muted">變更成功後目前登入會失效，需要重新登入。</p>

  <div class="card" style="max-width: 360px">
    <form @submit.prevent="onSubmit">
      <div class="field" style="margin-bottom: var(--space-4)">
        <label>目前密碼</label>
        <input v-model="oldPassword" type="password" required autocomplete="current-password" />
      </div>
      <div class="field" style="margin-bottom: var(--space-4)">
        <label>新密碼</label>
        <input v-model="newPassword" type="password" required minlength="6" autocomplete="new-password" />
      </div>
      <button type="submit" :disabled="loading">{{ loading ? '送出中…' : '更新密碼' }}</button>
      <p v-if="error" role="alert" style="margin-top: var(--space-3)">{{ error }}</p>
    </form>
  </div>
</template>
