<template>
  <el-container class="app-container">
    <el-header class="app-header">
      <div class="header-content">
        <el-icon class="logo-icon"><Platform /></el-icon>
        <h1>Docker 模板管理后台</h1>
      </div>
    </el-header>
    <el-main class="app-main">
      <router-view></router-view>
    </el-main>
  </el-container>
</template>

<style>
body {
  margin: 0;
  background-color: #f5f7fa;
  font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, 'Helvetica Neue', Arial, sans-serif;
}

.app-container {
  min-height: 100vh;
}

.app-header {
  background-color: #fff;
  border-bottom: 1px solid #e6e6e6;
  display: flex;
  align-items: center;
  box-shadow: 0 1px 4px rgba(0,0,0,0.05);
  z-index: 10;
}

.header-content {
  display: flex;
  align-items: center;
  gap: 12px;
}

.logo-icon {
  font-size: 24px;
  color: #409eff;
}

.app-header h1 {
  margin: 0;
  font-size: 18px;
  color: #303133;
  font-weight: 600;
}

.app-main {
  padding: 20px;
  max-width: 1200px;
  margin: 0 auto;
  width: 100%;
}
</style>