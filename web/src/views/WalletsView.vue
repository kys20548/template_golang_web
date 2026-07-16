<script setup>
import { onMounted, ref, watch } from 'vue'
import { request } from '../api/client'
import Pagination from '../components/Pagination.vue'

const pageNum = ref(1)
const pageSize = ref(10)
const total = ref(0)
const list = ref([])
const error = ref('')

async function fetchList() {
  error.value = ''
  try {
    const data = await request(`/wallets?pageNum=${pageNum.value}&pageSize=${pageSize.value}`)
    list.value = data.list
    total.value = data.total
  } catch (e) {
    error.value = e.message
  }
}

watch(pageNum, fetchList)
onMounted(fetchList)
</script>

<template>
  <h1>錢包</h1>
  <p class="muted">所有前台使用者的錢包餘額。</p>
  <p v-if="error" role="alert">{{ error }}</p>
  <div class="table-wrap" v-else>
    <table>
      <thead>
        <tr>
          <th>錢包 ID</th>
          <th>使用者</th>
          <th>Email</th>
          <th>餘額</th>
          <th>建立時間</th>
        </tr>
      </thead>
      <tbody>
        <tr v-for="w in list" :key="w.id">
          <td class="mono">{{ w.id }}</td>
          <td>{{ w.username }}</td>
          <td>{{ w.email }}</td>
          <td class="mono">{{ w.balance.toLocaleString() }}</td>
          <td class="mono">{{ new Date(w.created_at).toLocaleString() }}</td>
        </tr>
      </tbody>
    </table>
    <p class="empty-state" v-if="!list.length">目前沒有錢包資料</p>
    <Pagination
      v-else
      :page-num="pageNum"
      :page-size="pageSize"
      :total="total"
      @update:page-num="pageNum = $event"
    />
  </div>
</template>
