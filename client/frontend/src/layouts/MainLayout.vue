<template>
  <div class="app-layout">
    <AppSidebar />
    <div class="main">
      <AppHeader :title="pageTitle" />
      <div class="content">
        <router-view />
      </div>
      <AppFooter />
    </div>
  </div>
</template>

<script setup>
import AppHeader from '../components/AppHeader.vue'
import AppSidebar from '../components/AppSidebar.vue'
import AppFooter from '../components/AppFooter.vue'
import { useRoute } from 'vue-router'
import { computed } from 'vue'

const route = useRoute()
const pageTitle = computed(() => route.path.replace('/', '') || 'overview')
</script>
