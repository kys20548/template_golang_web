<script setup>
import { computed, inject, onMounted, ref } from 'vue'
import { request } from '../api/client'
import { hasPerm } from '../auth/perm'

const user = inject('authUser')

// 統計卡片：後端依權限個別過濾，無權限的欄位是 null，前端只顯示有值的
const stats = ref(null)
const statsError = ref('')

async function fetchStats() {
  statsError.value = ''
  try {
    stats.value = await request('/dashboard/stats')
  } catch (e) {
    statsError.value = e.message
  }
}

onMounted(fetchStats)

const statCards = computed(() => {
  if (!stats.value) return []
  return [
    { label: '前台使用者數', value: stats.value.user_count },
    { label: '錢包總餘額', value: stats.value.wallet_balance_total },
    { label: '今日操作數', value: stats.value.today_operation_count },
  ].filter((card) => card.value !== null)
})

const cards = [
  { to: '/users', label: '前台使用者', desc: '查詢帳號、瀏覽分頁列表', perm: 'user:read' },
  { to: '/admin-users', label: '後台使用者', desc: '帳號管理與角色指派', perm: 'admin_user:read' },
  { to: '/roles', label: '角色與權限', desc: '各角色的權限清單', perm: 'admin_user:read' },
  { to: '/wallets', label: '錢包', desc: '所有前台使用者的餘額', perm: 'wallet:read' },
  { to: '/operation-logs', label: '操作日誌', desc: '追蹤誰在什麼時候改了什麼', perm: 'operation_log:read' },
  { to: '/me/password', label: '修改密碼', desc: '變更後需要重新登入', perm: null },
]

const visibleCards = computed(() =>
  cards.filter((card) => !card.perm || hasPerm(user.value, card.perm)),
)
</script>

<template>
  <h1>首頁</h1>
  <p class="muted" v-if="user">
    歡迎回來，<strong>{{ user.username }}</strong>。
  </p>

  <p v-if="statsError" role="alert">{{ statsError }}</p>
  <div class="card-grid" v-if="statCards.length" style="margin-bottom: var(--space-5)">
    <div v-for="card in statCards" :key="card.label" class="card">
      <div class="stat-label">{{ card.label }}</div>
      <div class="stat-value">{{ card.value.toLocaleString() }}</div>
    </div>
  </div>

  <div class="card-grid">
    <router-link v-for="card in visibleCards" :key="card.to" :to="card.to" class="card">
      <div class="stat-label">{{ card.label }}</div>
      <p class="muted" style="margin: 0">{{ card.desc }}</p>
    </router-link>
  </div>
</template>
