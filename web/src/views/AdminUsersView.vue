<script setup>
import { computed, inject, onMounted, ref, watch } from 'vue'
import { request } from '../api/client'
import { hasPerm } from '../auth/perm'
import Pagination from '../components/Pagination.vue'

const authUser = inject('authUser')
const canWrite = computed(() => hasPerm(authUser.value, 'admin_user:write'))

const pageNum = ref(1)
const pageSize = ref(10)
const total = ref(0)
const list = ref([])
const error = ref('')

const roles = ref([])

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

async function fetchRoles() {
  try {
    roles.value = await request('/roles')
  } catch (e) {
    error.value = e.message
  }
}

watch(pageNum, fetchList)
onMounted(() => {
  fetchList()
  fetchRoles()
})

// 新增後台使用者
const showCreate = ref(false)
const createUsername = ref('')
const createPassword = ref('')
const createRoleIDs = ref([])
const createError = ref('')
const creating = ref(false)

async function onCreate() {
  createError.value = ''
  creating.value = true
  try {
    await request('/admin-users', {
      method: 'POST',
      body: JSON.stringify({
        username: createUsername.value,
        password: createPassword.value,
        role_ids: createRoleIDs.value,
      }),
    })
    showCreate.value = false
    createUsername.value = ''
    createPassword.value = ''
    createRoleIDs.value = []
    fetchList()
  } catch (e) {
    createError.value = e.message
  } finally {
    creating.value = false
  }
}

// 指派角色（整組取代）
const editingUser = ref(null)
const editingRoleIDs = ref([])
const editError = ref('')
const saving = ref(false)

function startEdit(user) {
  editingUser.value = user
  editingRoleIDs.value = user.roles.map((r) => r.id)
  editError.value = ''
}

async function onSaveRoles() {
  editError.value = ''
  saving.value = true
  try {
    await request(`/admin-users/${editingUser.value.id}/roles`, {
      method: 'PUT',
      body: JSON.stringify({ role_ids: editingRoleIDs.value }),
    })
    editingUser.value = null
    fetchList()
  } catch (e) {
    editError.value = e.message
  } finally {
    saving.value = false
  }
}
</script>

<template>
  <h1>後台使用者</h1>
  <p class="muted">可登入本後台系統的帳號。角色異動要等對方重新登入才生效。</p>

  <div class="toolbar" v-if="canWrite">
    <button v-if="!showCreate" @click="showCreate = true">新增後台使用者</button>
  </div>

  <div class="card" v-if="showCreate" style="max-width: 420px; margin-bottom: var(--space-4)">
    <form @submit.prevent="onCreate">
      <div class="field" style="margin-bottom: var(--space-4)">
        <label>帳號</label>
        <input v-model="createUsername" required autocomplete="off" />
      </div>
      <div class="field" style="margin-bottom: var(--space-4)">
        <label>密碼</label>
        <input v-model="createPassword" type="password" required minlength="6" autocomplete="new-password" />
      </div>
      <div class="field" style="margin-bottom: var(--space-4)">
        <label>角色</label>
        <label v-for="role in roles" :key="role.id" class="checkbox-row">
          <input type="checkbox" :value="role.id" v-model="createRoleIDs" />
          {{ role.name }}<span class="muted">（{{ role.description }}）</span>
        </label>
      </div>
      <button type="submit" :disabled="creating">{{ creating ? '建立中…' : '建立' }}</button>
      <button type="button" class="secondary" style="margin-left: var(--space-2)" @click="showCreate = false">取消</button>
      <p v-if="createError" role="alert" style="margin-top: var(--space-3)">{{ createError }}</p>
    </form>
  </div>

  <p v-if="error" role="alert">{{ error }}</p>
  <div class="table-wrap" v-else>
    <table>
      <thead>
        <tr>
          <th>ID</th>
          <th>帳號</th>
          <th>角色</th>
          <th>建立時間</th>
          <th v-if="canWrite"></th>
        </tr>
      </thead>
      <tbody>
        <tr v-for="u in list" :key="u.id">
          <td class="mono">{{ u.id }}</td>
          <td>{{ u.username }}</td>
          <td>
            <span v-if="!u.roles.length" class="muted">—</span>
            <span v-for="role in u.roles" :key="role.id" class="badge role-badge">{{ role.name }}</span>
          </td>
          <td class="mono">{{ new Date(u.created_at).toLocaleString() }}</td>
          <td v-if="canWrite">
            <button class="secondary" @click="startEdit(u)">指派角色</button>
          </td>
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

  <div class="card" v-if="editingUser" style="max-width: 420px; margin-top: var(--space-4)">
    <div class="stat-label">指派角色：{{ editingUser.username }}</div>
    <div class="field" style="margin: var(--space-3) 0 var(--space-4)">
      <label v-for="role in roles" :key="role.id" class="checkbox-row">
        <input type="checkbox" :value="role.id" v-model="editingRoleIDs" />
        {{ role.name }}<span class="muted">（{{ role.description }}）</span>
      </label>
    </div>
    <button @click="onSaveRoles" :disabled="saving">{{ saving ? '儲存中…' : '儲存' }}</button>
    <button class="secondary" style="margin-left: var(--space-2)" @click="editingUser = null">取消</button>
    <p v-if="editError" role="alert" style="margin-top: var(--space-3)">{{ editError }}</p>
  </div>
</template>
