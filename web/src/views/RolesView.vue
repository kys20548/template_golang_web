<script setup>
import { onMounted, ref } from 'vue'
import { request } from '../api/client'

const roles = ref([])
const error = ref('')

onMounted(async () => {
  try {
    roles.value = await request('/roles')
  } catch (e) {
    error.value = e.message
  }
})
</script>

<template>
  <h1>角色與權限</h1>
  <p class="muted">唯讀清單；角色與權限的異動由 migration / SQL 管理。</p>
  <p v-if="error" role="alert">{{ error }}</p>
  <div class="table-wrap" v-else>
    <table>
      <thead>
        <tr>
          <th>ID</th>
          <th>名稱</th>
          <th>說明</th>
          <th>權限</th>
        </tr>
      </thead>
      <tbody>
        <tr v-for="role in roles" :key="role.id">
          <td class="mono">{{ role.id }}</td>
          <td>{{ role.name }}</td>
          <td class="muted">{{ role.description }}</td>
          <td>
            <span v-for="code in role.permissions" :key="code" class="badge perm-badge mono">{{ code }}</span>
          </td>
        </tr>
      </tbody>
    </table>
    <p class="empty-state" v-if="!roles.length">目前沒有角色</p>
  </div>
</template>
