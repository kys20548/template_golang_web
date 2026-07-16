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
    const data = await request(`/admin-users?pageNum=${pageNum.value}&pageSize=${pageSize.value}`)
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
  <h1>後台使用者</h1>
  <p class="muted">可登入本後台系統的帳號。</p>
  <p v-if="error" role="alert">{{ error }}</p>
  <div class="table-wrap" v-else>
    <table>
      <thead>
        <tr>
          <th>ID</th>
          <th>帳號</th>
          <th>建立時間</th>
        </tr>
      </thead>
      <tbody>
        <tr v-for="u in list" :key="u.id">
          <td class="mono">{{ u.id }}</td>
          <td>{{ u.username }}</td>
          <td class="mono">{{ new Date(u.created_at).toLocaleString() }}</td>
        </tr>
      </tbody>
    </table>
    <p class="empty-state" v-if="!list.length">目前沒有後台使用者</p>
    <Pagination
      v-else
      :page-num="pageNum"
      :page-size="pageSize"
      :total="total"
      @update:page-num="pageNum = $event"
    />
  </div>
</template>
