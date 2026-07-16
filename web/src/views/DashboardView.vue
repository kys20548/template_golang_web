<script setup>
import { inject } from 'vue'

const user = inject('authUser')
</script>

<template>
  <h1>首頁</h1>
  <p class="muted" v-if="user">
    歡迎回來，<strong>{{ user.username }}</strong>。
  </p>

  <div class="card-grid">
    <router-link to="/users" class="card">
      <div class="stat-label">使用者</div>
      <p class="muted" style="margin: 0">查詢帳號、瀏覽分頁列表</p>
    </router-link>
    <router-link to="/wallet" class="card">
      <div class="stat-label">我的錢包</div>
      <p class="muted" style="margin: 0">查看目前餘額</p>
    </router-link>
    <router-link to="/operation-logs" class="card">
      <div class="stat-label">操作日誌</div>
      <p class="muted" style="margin: 0">追蹤誰在什麼時候改了什麼</p>
    </router-link>
    <router-link to="/me/password" class="card">
      <div class="stat-label">修改密碼</div>
      <p class="muted" style="margin: 0">變更後需要重新登入</p>
    </router-link>
  </div>
</template>
