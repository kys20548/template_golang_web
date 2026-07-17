<script setup>
import { onMounted, ref, watch } from 'vue'
import { useRoute } from 'vue-router'
import { request } from '../api/client'
import Pagination from '../components/Pagination.vue'

const route = useRoute()
const walletId = route.params.id

const wallet = ref(null)
const pageNum = ref(1)
const pageSize = ref(10)
const total = ref(0)
const list = ref([])
const error = ref('')

async function fetchWallet() {
  try {
    wallet.value = await request(`/wallets/${walletId}`)
  } catch (e) {
    error.value = e.message
  }
}

async function fetchList() {
  error.value = ''
  try {
    const data = await request(
      `/wallets/${walletId}/entries?pageNum=${pageNum.value}&pageSize=${pageSize.value}`,
    )
    list.value = data.list
    total.value = data.total
  } catch (e) {
    error.value = e.message
  }
}

watch(pageNum, fetchList)
onMounted(() => {
  fetchWallet()
  fetchList()
})
</script>

<template>
  <h1>錢包明細</h1>
  <p class="muted">
    <RouterLink to="/wallets">← 返回錢包列表</RouterLink>
  </p>

  <p v-if="error" role="alert">{{ error }}</p>
  <template v-else>
    <div class="card" v-if="wallet" style="margin-bottom: var(--space-4)">
      <div class="stat-label">
        錢包 #{{ wallet.id }}｜{{ wallet.username }}<span class="muted">（{{ wallet.email }}）</span>
      </div>
      <div class="stat-value">{{ wallet.balance.toLocaleString() }}</div>
    </div>

    <div class="table-wrap">
      <table>
        <thead>
          <tr>
            <th>ID</th>
            <th>金額</th>
            <th>備註</th>
            <th>操作者</th>
            <th>時間</th>
          </tr>
        </thead>
        <tbody>
          <tr v-for="entry in list" :key="entry.id">
            <td class="mono">{{ entry.id }}</td>
            <td class="mono" :class="entry.amount > 0 ? 'status-ok' : 'status-fail'">
              {{ entry.amount > 0 ? '+' : '' }}{{ entry.amount.toLocaleString() }}
            </td>
            <td>{{ entry.note || '—' }}</td>
            <td>{{ entry.operator_username }}</td>
            <td class="mono">{{ new Date(entry.created_at).toLocaleString() }}</td>
          </tr>
        </tbody>
      </table>
      <p class="empty-state" v-if="!list.length">目前沒有異動紀錄</p>
      <Pagination
        v-else
        :page-num="pageNum"
        :page-size="pageSize"
        :total="total"
        @update:page-num="pageNum = $event"
      />
    </div>
  </template>
</template>
