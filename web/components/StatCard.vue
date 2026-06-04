<script setup lang="ts">
defineProps<{
  label: string
  value: string
}>()
</script>

<template>
  <el-card shadow="hover" class="!border-0">
    <div class="text-xs uppercase tracking-wide text-gray-400">{{ label }}</div>
    <div class="mt-1 text-2xl font-semibold text-gray-800">{{ value }}</div>
  </el-card>
</template>
