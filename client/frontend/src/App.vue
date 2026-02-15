<template>
  <router-view></router-view>
</template>

<script setup>
import { onMounted } from 'vue'

onMounted(() => {
  const applyTheme = () => {
    // 1. Handle Dark Mode
    const theme = localStorage.getItem('theme') || 'auto'
    const isDark = theme === 'dark' || (theme === 'auto' && window.matchMedia('(prefers-color-scheme: dark)').matches)
    
    if (isDark) {
      document.documentElement.classList.add('dark')
    } else {
      document.documentElement.classList.remove('dark')
    }

    // 2. Handle UI Style Theme (Clay/Modern/Retro)
    const uiTheme = localStorage.getItem('ui-theme') || 'clay'
    // Remove all possible theme classes
    document.documentElement.classList.remove('theme-clay', 'theme-modern', 'theme-retro')
    // Add current theme class
    document.documentElement.classList.add(`theme-${uiTheme}`)
  }

  // 初始化主题
  applyTheme()

  // 监听自定义主题变更事件
  window.addEventListener('theme-change', applyTheme)
  window.addEventListener('ui-theme-change', applyTheme)
  
  // 监听系统主题变更
  window.matchMedia('(prefers-color-scheme: dark)').addEventListener('change', () => {
    if (localStorage.getItem('theme') === 'auto') {
      applyTheme()
    }
  })
})
</script>
