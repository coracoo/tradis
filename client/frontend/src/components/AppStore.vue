<!-- frontend/src/components/AppStore.vue -->
<template>
    <el-row :gutter="20">
      <el-col v-for="app in apps" :key="app.id" :span="6">
        <el-card class="app-card">
          <template #header>
            <h3>{{ app.name }}</h3>
          </template>
          <img :src="app.logo" class="app-logo"/>
          <p>{{ app.description }}</p>
          <el-button @click="deployApp(app)">一键部署</el-button>
        </el-card>
      </el-col>
    </el-row>
  </template>