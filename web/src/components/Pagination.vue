<script setup>
import { computed } from 'vue'

const props = defineProps({
  pageNum: { type: Number, required: true },
  pageSize: { type: Number, required: true },
  total: { type: Number, required: true },
})
const emit = defineEmits(['update:pageNum'])

const totalPages = computed(() => Math.max(1, Math.ceil(props.total / props.pageSize)))
</script>

<template>
  <div class="pagination">
    <button
      class="secondary"
      :disabled="pageNum <= 1"
      @click="emit('update:pageNum', pageNum - 1)"
    >
      上一頁
    </button>
    <span>第 {{ pageNum }} / {{ totalPages }} 頁，共 {{ total }} 筆</span>
    <button
      class="secondary"
      :disabled="pageNum >= totalPages"
      @click="emit('update:pageNum', pageNum + 1)"
    >
      下一頁
    </button>
  </div>
</template>
