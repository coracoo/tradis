# client 主题推进计划（主题plan）

## 背景与目标
- 在设置页提供“主题”选项，允许用户在多套 CSS 主题之间切换
- 主题切换对全局布局、Element Plus 组件、以及各页面自定义样式同时生效
- 落地方式以“主题变量（tokens）+ 主题类名切换”为主，避免重复维护两套完整样式

## 现有实现（已具备的基础）
- 主题入口：通过 [App.vue](file:///home/cherry/tradis/client/frontend/src/App.vue) 读取 localStorage.theme 并切换 html.dark / html.theme-<name>
- 样式入口：在 [main.js](file:///home/cherry/tradis/client/frontend/src/main.js) 引入 Element Plus 样式、dark css-vars、以及 [layout.css](file:///home/cherry/tradis/client/frontend/src/assets/css/layout.css)
- 全局主题变量：主要集中在 [layout.css](file:///home/cherry/tradis/client/frontend/src/assets/css/layout.css)

## 主题方案
### 主题模型
- 基于 html 上的 class 进行切换：html.theme-<name>
- 兼容 Dark Mode：html.dark 可与 html.theme-<name> 共存（或由主题自身定义 dark 变体）
- 核心变量集（tokens）：颜色、阴影、圆角、边框、间距

### 已实现主题
1. **Claymorphism (拟态粘土)** - *Default*
   - 特征：浮动、大圆角、柔和双向阴影、渐变背景
2. **SaaS Modern**
   - 特征：扁平、清晰边框、高对比度、小圆角、Tailwind 风格
3. **Retro/Vintage**
   - 特征：复古纸张背景、Solarized 配色、硬边框、无圆角、等宽字体

## 主题推进进度

### 1. 基础架构 (Common)
- [x] 改造 layout.css 支持多主题变量定义
- [x] 更新 App.vue 实现主题 class 切换与持久化
- [x] Settings.vue 增加主题切换 UI
- [x] 修复 logs 目录问题与 Docker socket 权限过滤
- [x] 统一状态颜色变量 (--status-idle/active/used/free)
- [x] 统一统计图标颜色变量 (--stat-icon-color)

### 2. Claymorphism (拟态粘土)
- [x] 核心变量定义 (:root)
- [x] 基础组件适配 (.clay-surface)
- [x] Dark Mode 适配

### 3. SaaS Modern
- [x] 定义主题变量 (html.theme-modern)
- [x] 适配 layout 布局变量
- [x] 适配 Element Plus 覆盖样式

### 4. Retro/Vintage
- [x] 定义主题变量 (html.theme-retro)
- [x] 适配 layout 布局变量
- [x] 适配 Element Plus 覆盖样式

### 5. 页面适配进度 (Page Adaptation)
- [x] **AppStore.vue**: 适配背景、阴影、Filter Bar
- [x] **Docker.vue**: 适配进度条、状态点、图标背景
- [x] **Images.vue**: 适配图标背景、状态颜色、Tags
- [x] **Networks.vue**: 适配网络图标、分页栏
- [x] **Volumes.vue**: 适配卷图标、状态点
- [x] **Compose.vue**: 适配状态点、表格样式
- [x] **Overview.vue**: 适配统计卡片、资源进度条、日志列表
- [x] **Projects.vue**: 适配项目列表图标、状态
- [x] **AppDeploy.vue**: 适配部署表单、教程区域
- [x] **Ports.vue**: 适配端口状态指示灯
- [x] **DockerDetail.vue**: 适配详情页图表、日志
- [x] **ProjectDetail.vue**: 适配项目详情日志、YAML 编辑器
- [x] **Navigation.vue**: 适配导航卡片、图标背景
- [x] **Login.vue**: 适配登录卡片、输入框样式
- [x] **ContainerTerminal.vue**: 适配终端背景、边框 (scoped)
- [x] **ContainerEdit.vue**: 移除全局污染，适配表单样式
- [x] **DockerSettings.vue**: 移除全局污染，适配设置弹窗

### 6. 待优化项 (Optimization)
- [x] **公用样式抽取**: 将 filter-bar, pagination-bar, icon-wrapper 等重复样式抽取到全局 [layout.css](file:///home/cherry/tradis/client/frontend/src/assets/css/layout.css)
- [x] **Overview.vue**: 移除废弃的 filter-bar 样式
- [x] **Global Style 治理**: 修复 ContainerTerminal/Edit/Settings 中的全局样式污染
- [ ] **图标资源适配**: 考虑将 icons/clay/*.jpg 等静态资源改为 CSS 背景或 SVG 以适应不同主题

## 文件与职责（建议分层）
- tokens（主题变量层）：定义颜色/阴影/边框/圆角/间距等变量集合
- base（基础布局层）：app-layout/sidebar/topbar/footer 等通用结构样式
- components（组件覆盖层）：Element Plus 深度选择器的覆盖（:deep / html.xxx .el-*）
- pages（页面层）：每个页面保留“结构 + 布局”差异，颜色与风格尽量走 tokens

## 样式重构建议
### 已抽取的公用样式 (layout.css)
- .filter-bar / .filter-left / .filter-right
- .pagination-bar
- .icon-wrapper / .app-icon-wrapper
- .content-wrapper / .table-wrapper
- .search-input

### 需隔离的 Global 样式 (已完成)
- ContainerTerminal.vue (Added scoped)
- ContainerEdit.vue (Removed global styles)
- DockerSettings.vue (Removed global styles)
