<template>
  <!-- 删除 footer，或根据设计图要求修改 -->
  <!-- 如果需要保留，可以简化为不可见或非常简单的样式 -->
  <!-- 这里根据参考图，似乎没有明显的全局 footer，通常在 dashboard 下方 -->
  <footer class="footer" v-if="false">
    <div>v1.0.0 · Vue3 + Vite · ElementPlus</div>
    <div>© {{ year }} Docker Manager</div>
  </footer>
</template>

<script setup>
const year = new Date().getFullYear()
</script>

<style scoped>
/* 如果需要显示 footer，添加样式 */
</style>