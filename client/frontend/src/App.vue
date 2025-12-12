<template>
  <router-view></router-view>
</template>

<script setup>
import { onMounted } from 'vue'

onMounted(() => {
  const applyTheme = () => {
    const theme = localStorage.getItem('theme') || 'auto'
    const isDark = theme === 'dark' || (theme === 'auto' && window.matchMedia('(prefers-color-scheme: dark)').matches)
    
    if (isDark) {
      document.documentElement.classList.add('dark')
    } else {
      document.documentElement.classList.remove('dark')
    }
  }

  // 初始化主题
  applyTheme()

  // 监听自定义主题变更事件
  window.addEventListener('theme-change', applyTheme)
  
  // 监听系统主题变更
  window.matchMedia('(prefers-color-scheme: dark)').addEventListener('change', () => {
    if (localStorage.getItem('theme') === 'auto') {
      applyTheme()
    }
  })
})
</script>

<style>
/* 全局样式 */
html, body {
  margin: 0;
  padding: 0;
  height: 100%;
}

#app {
  height: 100vh;
  font-family: Arial, sans-serif;
}
</style>