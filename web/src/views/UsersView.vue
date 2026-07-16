<script setup>
import { onMounted, ref, watch } from 'vue'
import { request } from '../api/client'
import Pagination from '../components/Pagination.vue'

const pageNum = ref(1)
const pageSize = ref(10)
const total = ref(0)
const list = ref([])
const listError = ref('')

async function fetchList() {
  listError.value = ''
  try {
    const data = await request(`/users?pageNum=${pageNum.value}&pageSize=${pageSize.value}`)
    list.value = data.list
    total.value = data.total
  } catch (e) {
    listError.value = e.message
  }
}

watch(pageNum, fetchList)
onMounted(fetchList)

const lookupId = ref('')
const lookupResult = ref(null)
const lookupError = ref('')
const lookupLoading = ref(false)

async function onLookup() {
  if (!lookupId.value) return
  lookupLoading.value = true
  lookupError.value = ''
  lookupResult.value = null
  try {
    lookupResult.value = await request(`/users/${lookupId.value}`)
  } catch (e) {
    lookupError.value = e.message
  } finally {
    lookupLoading.value = false
  }
}

function clearLookup() {
  lookupId.value = ''
  lookupResult.value = null
  lookupError.value = ''
}
</script>

<template>
  <h1>使用者</h1>

  <div class="toolbar">
    <div class="field">
      <label>依 ID 查詢</label>
      <input v-model="lookupId" type="number" min="1" placeholder="輸入使用者 ID" />
    </div>
    <button @click="onLookup" :disabled="lookupLoading">查詢</button>
    <button class="secondary" v-if="lookupResult || lookupError" @click="clearLookup">清除</button>
  </div>

  <p v-if="lookupError" role="alert">{{ lookupError }}</p>

  <div class="card" v-if="lookupResult">
    <div class="stat-label">查詢結果</div>
    <table>
      <tbody>
        <tr><td class="muted">ID</td><td class="mono">{{ lookupResult.id }}</td></tr>
        <tr><td class="muted">帳號</td><td>{{ lookupResult.username }}</td></tr>
        <tr><td class="muted">Email</td><td>{{ lookupResult.email }}</td></tr>
        <tr><td class="muted">建立時間</td><td class="mono">{{ new Date(lookupResult.created_at).toLocaleString() }}</td></tr>
      </tbody>
    </table>
  </div>

  <template v-else>
    <p v-if="listError" role="alert">{{ listError }}</p>
    <div class="table-wrap" v-else>
      <table>
        <thead>
          <tr>
            <th>ID</th>
            <th>帳號</th>
            <th>Email</th>
            <th>建立時間</th>
          </tr>
        </thead>
        <tbody>
          <tr v-for="u in list" :key="u.id">
            <td class="mono">{{ u.id }}</td>
            <td>{{ u.username }}</td>
            <td>{{ u.email }}</td>
            <td class="mono">{{ new Date(u.created_at).toLocaleString() }}</td>
          </tr>
        </tbody>
      </table>
      <p class="empty-state" v-if="!list.length">目前沒有使用者資料</p>
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
