<script setup>
import { computed, inject, onMounted, ref, watch } from 'vue'
import { request } from '../api/client'
import { hasPerm } from '../auth/perm'
import Pagination from '../components/Pagination.vue'

const authUser = inject('authUser')
const canWrite = computed(() => hasPerm(authUser.value, 'user:write'))

const pageNum = ref(1)
const pageSize = ref(10)
const total = ref(0)
const list = ref([])
const listError = ref('')
const includeDeleted = ref(false)

async function fetchList() {
  listError.value = ''
  try {
    const data = await request(
      `/users?pageNum=${pageNum.value}&pageSize=${pageSize.value}&includeDeleted=${includeDeleted.value}`,
    )
    list.value = data.list
    total.value = data.total
  } catch (e) {
    listError.value = e.message
  }
}

watch(pageNum, fetchList)
watch(includeDeleted, () => {
  pageNum.value = 1
  fetchList()
})
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

// 刪除採兩段式確認（第一下變「確認刪除」，再點才送出），不用 window.confirm
const confirmDeleteId = ref(null)
const actionError = ref('')

async function onDelete(user) {
  if (confirmDeleteId.value !== user.id) {
    confirmDeleteId.value = user.id
    return
  }
  actionError.value = ''
  try {
    await request(`/users/${user.id}`, { method: 'DELETE' })
    confirmDeleteId.value = null
    fetchList()
  } catch (e) {
    actionError.value = e.message
  }
}

async function onRestore(user) {
  actionError.value = ''
  try {
    await request(`/users/${user.id}/restore`, { method: 'PUT' })
    fetchList()
  } catch (e) {
    actionError.value = e.message
  }
}
</script>

<template>
  <h1>前台使用者</h1>

  <div class="toolbar">
    <div class="field">
      <label>依 ID 查詢</label>
      <input v-model="lookupId" type="number" min="1" placeholder="輸入使用者 ID" />
    </div>
    <button @click="onLookup" :disabled="lookupLoading">查詢</button>
    <button class="secondary" v-if="lookupResult || lookupError" @click="clearLookup">清除</button>
    <label class="checkbox-row" style="margin-left: auto">
      <input type="checkbox" v-model="includeDeleted" />
      含已刪除
    </label>
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
        <tr v-if="lookupResult.deleted_at"><td class="muted">刪除時間</td><td class="mono">{{ new Date(lookupResult.deleted_at).toLocaleString() }}</td></tr>
      </tbody>
    </table>
  </div>

  <template v-else>
    <p v-if="listError" role="alert">{{ listError }}</p>
    <div class="table-wrap" v-else>
      <p v-if="actionError" role="alert">{{ actionError }}</p>
      <table>
        <thead>
          <tr>
            <th>ID</th>
            <th>帳號</th>
            <th>Email</th>
            <th>建立時間</th>
            <th v-if="canWrite"></th>
          </tr>
        </thead>
        <tbody>
          <tr v-for="u in list" :key="u.id">
            <td class="mono">{{ u.id }}</td>
            <td>
              {{ u.username }}
              <span v-if="u.deleted_at" class="badge deleted-badge">已刪除</span>
            </td>
            <td>{{ u.email }}</td>
            <td class="mono">{{ new Date(u.created_at).toLocaleString() }}</td>
            <td v-if="canWrite">
              <button v-if="u.deleted_at" class="secondary" @click="onRestore(u)">還原</button>
              <button v-else class="danger" @click="onDelete(u)">
                {{ confirmDeleteId === u.id ? '確認刪除' : '刪除' }}
              </button>
            </td>
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
