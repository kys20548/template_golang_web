<script setup>
import { computed, inject, onMounted, ref, watch } from 'vue'
import { request } from '../api/client'
import { hasPerm } from '../auth/perm'
import Pagination from '../components/Pagination.vue'

const authUser = inject('authUser')
const canWrite = computed(() => hasPerm(authUser.value, 'wallet:write'))

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

// 加款/扣款：direction +1 / -1，金額輸入一律正數，送出時帶正負
const adjusting = ref(null) // { wallet, direction }
const adjustAmount = ref('')
const adjustNote = ref('')
const adjustError = ref('')
const submitting = ref(false)

function startAdjust(wallet, direction) {
  adjusting.value = { wallet, direction }
  adjustAmount.value = ''
  adjustNote.value = ''
  adjustError.value = ''
}

async function onAdjust() {
  adjustError.value = ''
  submitting.value = true
  try {
    await request(`/wallets/${adjusting.value.wallet.id}/adjust`, {
      method: 'POST',
      body: JSON.stringify({
        amount: adjusting.value.direction * Number(adjustAmount.value),
        note: adjustNote.value,
      }),
    })
    adjusting.value = null
    fetchList()
  } catch (e) {
    adjustError.value = e.message
  } finally {
    submitting.value = false
  }
}
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
          <th></th>
        </tr>
      </thead>
      <tbody>
        <tr v-for="w in list" :key="w.id">
          <td class="mono">{{ w.id }}</td>
          <td>{{ w.username }}</td>
          <td>{{ w.email }}</td>
          <td class="mono">{{ w.balance.toLocaleString() }}</td>
          <td class="mono">{{ new Date(w.created_at).toLocaleString() }}</td>
          <td>
            <RouterLink :to="`/wallets/${w.id}`">明細</RouterLink>
            <template v-if="canWrite">
              <button style="margin-left: var(--space-2)" @click="startAdjust(w, 1)">加款</button>
              <button class="danger" style="margin-left: var(--space-2)" @click="startAdjust(w, -1)">扣款</button>
            </template>
          </td>
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

  <div class="card" v-if="adjusting" style="max-width: 420px; margin-top: var(--space-4)">
    <div class="stat-label">
      {{ adjusting.direction > 0 ? '加款' : '扣款' }}：{{ adjusting.wallet.username }}
      <span class="muted">（目前餘額 {{ adjusting.wallet.balance.toLocaleString() }}）</span>
    </div>
    <form @submit.prevent="onAdjust">
      <div class="field" style="margin: var(--space-3) 0 var(--space-4)">
        <label>金額</label>
        <input v-model="adjustAmount" type="number" min="1" required placeholder="輸入正整數" />
      </div>
      <div class="field" style="margin-bottom: var(--space-4)">
        <label>備註</label>
        <input v-model="adjustNote" maxlength="255" placeholder="選填" />
      </div>
      <button type="submit" :disabled="submitting">
        {{ submitting ? '送出中…' : (adjusting.direction > 0 ? '確認加款' : '確認扣款') }}
      </button>
      <button type="button" class="secondary" style="margin-left: var(--space-2)" @click="adjusting = null">取消</button>
      <p v-if="adjustError" role="alert" style="margin-top: var(--space-3)">{{ adjustError }}</p>
    </form>
  </div>
</template>
