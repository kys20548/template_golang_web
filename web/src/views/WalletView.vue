<script setup>
import { onMounted, ref } from 'vue'
import { request } from '../api/client'

const wallet = ref(null)
const error = ref('')

onMounted(async () => {
  try {
    wallet.value = await request('/wallet')
  } catch (e) {
    error.value = e.message
  }
})
</script>

<template>
  <h1>我的錢包</h1>
  <p v-if="error" role="alert">{{ error }}</p>
  <div class="card" v-else-if="wallet">
    <div class="stat-label">餘額</div>
    <div class="stat-value">{{ wallet.balance.toLocaleString() }}</div>
    <p class="muted" style="margin-top: var(--space-3)">
      建立時間：<span class="mono">{{ new Date(wallet.created_at).toLocaleString() }}</span>
    </p>
  </div>
  <p v-else class="muted">載入中…</p>
</template>
