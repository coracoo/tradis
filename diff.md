# diff.md

## 1. 通用内容
- 全局样式基线来自 `client/frontend/src/assets/css/layout.css`，包含 `.filter-bar`、`.filter-left/.filter-right`、`.content-wrapper/.table-wrapper`、`.pagination-bar`、`.el-button` 与主题变量体系
- 第一类页面普遍采用统一容器结构：`height: 100%` + `display: flex` + `padding: 12px 16px` + `gap: 12px` + `background-color: var(--clay-bg)`，其中 `Compose/Images/Networks` 还显式声明 `width: 100%`
- 第一类页面顶部操作区统一为 `filter-bar clay-surface`，按钮多为 `size="medium"`，配套 `:deep(.el-button--medium){ padding:10px 20px; height:36px; }`（分散在各页面）
- 内容区在第一类页面中分两类：`table-wrapper`（表格类）或 `content-wrapper + scroll-container`（列表/卡片类）

## 2. 整体要求
- 详情页需完全复用全局 `filter-bar` 体系（布局、间距、圆角、阴影），禁止再次引入 `header-bar/header-left/header-right` 的本地样式
- 详情页按钮布局需要遵循第一类页面的“左标题右操作”结构，按钮为同一层级的 `medium` 按钮，避免混用 `square-btn` 或非标准尺寸
- 详情页内容区需对齐第一类页面的内容容器节奏（有滚动的页面需具备统一 padding，避免“无内边距贴边”）
- 中等按钮尺寸、`more-btn` 以及 `filter-bar` 规则建议回收至公共样式，减少重复定义

## 3. 已完成内容
- `DockerDetail.vue`/`ProjectDetail.vue` 顶部栏已切换为 `filter-bar clay-surface`，移除 `header-bar` 体系
- 详情页操作按钮统一为 `size="medium"` 并添加图标，与 Compose/Images/Networks 的按钮规格一致
- 详情页按钮尺寸补齐为 `36px` 高度，贴合第一类页面中 `:deep(.el-button--medium)` 的基线规格

## 4. to-do 列表
- 将第一类页面分散的 `:deep(.el-button--medium)` 迁移到 `layout.css`，减少重复
- 统一详情页内容区内边距策略（引入与 AppStore/Navigation/Overview 类似的 `scroll-container` 或固定 padding 规则）
- 收敛 `filter-bar` 的局部重复定义（例如 Ports/Compose/Projects/Images 的本地覆盖）

---

# 差异点清单（第一类 vs 第二类）

## 页面布局差异
- 第一类：普遍存在 `scroll-container` 或 `table-wrapper` 作为主内容滚动容器；第二类：`content-wrapper` 内直接 `el-tabs`，缺乏统一的滚动容器与内边距
- 第一类：容器多为 `width: 100%`（Compose/Images/Networks）；第二类：仅 `height: 100%`，宽度未显式声明
- 第一类：`filter-bar` 样式统一从 layout.css 抽取；第二类：历史上存在 `header-bar` 私有样式（现已移除，但仍需避免回流）

## 按钮布局差异
- 第一类：按钮多为分组或统一 action 区（`filter-right` + `button-group`），间距标准化为 `gap: 16px`
- 第二类：按钮同排但更依赖页面级控制（无 group 包裹），与第一类主区结构不同步

## 按钮样式差异
- 第一类：多处声明 `:deep(.el-button--medium)` 统一高度/内边距
- 第二类：此前缺少统一的 medium 尺寸定义，导致视觉重量偏大（现已补齐但仍是局部定义）

## 整体样式差异
- 第一类：内容区普遍存在统一 padding（如 AppStore 18px、Navigation 24px、Overview 16px），空间节奏一致
- 第二类：内容区 padding 不明显或由子组件决定（如 logs/yaml 自带 padding），整体节奏偏散
- 第一类：在 layout.css 内统一了 `clay-surface` 与 `content-wrapper/table-wrapper` 的阴影、边框和圆角基线
- 第二类：历史上存在本地 `header-bar`/`square-btn` 圆角规则，与全局基线冲突（已清理但需防止回归）
