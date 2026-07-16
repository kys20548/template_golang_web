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
    const data = await request(`/operation-logs?pageNum=${pageNum.value}&pageSize=${pageSize.value}`)
    list.value = data.list
    total.value = data.total
  } catch (e) {
    error.value = e.message
  }
}

watch(pageNum, fetchList)
onMounted(fetchList)

function methodClass(method) {
  return `badge method-${method.toLowerCase()}`
}

function statusClass(status) {
  return status < 400 ? 'mono status-ok' : 'mono status-fail'
}
</script>

<template>
  <h1>操作日誌</h1>
  <p v-if="error" role="alert">{{ error }}</p>
  <div class="table-wrap" v-else>
    <table>
      <thead>
        <tr>
          <th>時間</th>
          <th>使用者</th>
          <th>Method</th>
          <th>Path</th>
          <th>Status</th>
          <th>Request ID</th>
        </tr>
      </thead>
      <tbody>
        <tr v-for="log in list" :key="log.id">
          <td class="mono">{{ new Date(log.created_at).toLocaleString() }}</td>
          <td>{{ log.username || '—' }}</td>
          <td><span :class="methodClass(log.method)">{{ log.method }}</span></td>
          <td class="mono">{{ log.path }}</td>
          <td :class="statusClass(log.status_code)">{{ log.status_code }}</td>
          <td class="mono" :title="log.request_id">{{ log.request_id.slice(0, 8) }}</td>
        </tr>
      </tbody>
    </table>
    <p class="empty-state" v-if="!list.length">目前沒有操作紀錄</p>
    <Pagination
      v-else
      :page-num="pageNum"
      :page-size="pageSize"
      :total="total"
      @update:page-num="pageNum = $event"
    />
  </div>
</template>
