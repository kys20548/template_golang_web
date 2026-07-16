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
      <div class="stat-label">前台使用者</div>
      <p class="muted" style="margin: 0">查詢帳號、瀏覽分頁列表</p>
    </router-link>
    <router-link to="/admin-users" class="card">
      <div class="stat-label">後台使用者</div>
      <p class="muted" style="margin: 0">可登入本後台的帳號</p>
    </router-link>
    <router-link to="/wallets" class="card">
      <div class="stat-label">錢包</div>
      <p class="muted" style="margin: 0">所有前台使用者的餘額</p>
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
