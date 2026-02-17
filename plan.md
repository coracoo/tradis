# 项目进度与计划

## 当前阶段：镜像更新与设置页优化

### 已完成任务
- [x] **设置页重构**
  - [x] 改为左右分栏布局（左侧导航/状态，右侧内容）
  - [x] 端口管理集成到设置页主区
  - [x] 保持 Clay 拟态风格与响应式设计
  - [x] 修复左侧栏空白问题（对齐与全宽）
- [x] **镜像更新功能**
  - [x] 后端：Compose 项目更新检测与 SSE 事件流
  - [x] 后端：容器更新检测与 SSE 事件流（支持回滚）
  - [x] 前端：Compose 页面增加更新提醒与一键更新（SSE 日志弹窗）
  - [x] 前端：Docker 容器列表页增加更新提醒与一键更新（SSE 日志弹窗）
  - [x] 前端：Images 页面优化更新交互（可视化进度条、允许更新使用中镜像、SSE 弹窗）
  - [x] 数据库：集成 `image_updates` 表用于状态追踪
  - [x] 前端：Compose 列表同步 updateAvailable/updateCount 映射
  - [x] 后端：Compose 列表补充解析 compose 文件以匹配更新镜像
  - [x] 前端：Compose 页面为独立容器显示更新提醒并支持一键更新
  - [x] 前端：Compose 列表新增更新提醒列
  - [x] 前端：Compose 名称列移除重复更新徽标
- [x] **侧边栏版本提醒修复**
  - [x] 本地版本空值显示与“有新版”提示条件修正
- [x] 前端：修复 Element Plus size="medium" 控制台告警（统一改为 size="default"）
- [x] 稳定性：高负载自动降载（容器列表 Inspect 降级 + 概览页刷新自适应）
- [x] 前端：轮询/延迟刷新统一切换标准（采集后端建议间隔并全局复用）
- [x] 运行时：修复后端旧进程导致容器更新标记缺失
- [x] **卷备份（docker-volume-backup）**
  - [x] 设置页增加卷备份开关与环境变量配置
  - [x] 设置页提示本地归档目录需提前创建，并增加每日备份 Cron 配置
  - [x] 后端托管备份容器（拉取镜像/挂载卷/写入消息中心通知）
- [x] **卷文件浏览器**
  - [x] Volumes 页面新增“浏览文件”按钮并打开新窗口
  - [x] 后端创建临时容器挂载卷并反向代理 filebrowser
  - [x] 心跳 + 关闭自动清理，消息中心记录启停
  - [x] 静态资源访问改为支持 Cookie Token 认证
  - [x] 路径改写支持 /api 与 favicon 免登录访问
  - [x] 自动销毁清理按 session 校验并周期清理
- [x] **忽略规则整理**
  - [x] 统一更新项目内 .gitignore 与 .dockerignore
- [x] **上传脚本优化**
  - [x] upload_to_github.sh 增强参数、身份与 SSH 处理
- [x] **模板变量方案完善**
  - [x] 单文件 YAML + 多 env_file + secrets 的 schema/env 统一方案

### 待办任务
- [ ] **系统测试**
  - [ ] 验证全链路镜像更新流程
  - [ ] 验证设置页在移动端的表现

## 当前阶段：模板部署链路优化

### 已完成任务
- [x] **变量解析补齐**
  - [x] Compose 变量引用识别支持 $VAR，覆盖 healthcheck/labels 等非 environment 区域
- [x] **模板变量落地（第一期）**
  - [x] vars 响应增加 params（env/secret 统一清单）
  - [x] 部署支持多 env_file 固定落盘结构并重写路径
  - [x] 部署支持 file secret 写盘与 external secret 提示
  - [x] Settings 增加高级模式开关并在 YAML 保存入口加门禁
  - [x] AppDeploy 接入 params 并增加 secrets 输入
- [x] **商城请求降载（第一步）**
  - [x] Client 后端 AppStore 列表接口加入 TTL 缓存与条件请求（ETag/Last-Modified）

### 待办任务
- [x] **后端统一生成变量清单（schema/refs）**
  - [x] Client 后端新增 /api/appstore/apps/:id/vars
  - [x] Server 后端新增 /api/templates/parse-vars
- [x] **部署页右侧变量栏（集中编辑 .env）**
  - [x] 部署页改用 vars 接口拉取变量清单
- [x] 部署页 schema/env 左右布局（schema 左，env 右）
- [x] **部署页请求与状态查询减载（SWR/节流/复用结果）**
  - [x] Client 后端应用详情加入 TTL 缓存与条件请求

## 当前阶段：模板批量导入与 MCP 工作流

### 已完成任务
- [x] **MCP 批量导入接口**
  - [x] 新增 `POST /api/mcp/templates/import`，支持 `dryRun` 与 `upsert_by_name/create_only`
  - [x] 支持缺省 schema 时自动从 compose 解析补齐
- [x] **白名单与 Token 动态维护**
  - [x] `ADMIN_ALLOWLIST` 从“仅环境变量”升级为“DB（ServerKV）优先 + env 回退”，支持热更新
  - [x] 新增 MCP 白名单与 MCP Token（可选）配置，支持落库与热更新
  - [x] 新增管理接口：`/api/admin/allowlist`、`/api/admin/mcp-allowlist`、`/api/admin/mcp-token`
- [x] **可视化界面**
  - [x] server 模板管理页新增“安全设置”弹窗，用于维护白名单与 token
- [x] **文档交付**
  - [x] 提供 AI/工作流可用的 MCP 调用文档与 OpenAPI 子集（见 `docs/`）

## 下一阶段：Client 端首页导航优化（AI 增强）

### 待办任务
- [x] **全局 AI 配置**
  - [x] 后端：扩展 `global_settings` 支持 AI 参数（URL, Key, Model, Temp, Prompt, Enabled）
  - [x] 后端：实现 AI 连接性测试接口 `POST /api/ai/test`
  - [x] 前端：设置页新增 "AI 助手" 配置卡片
  - [x] 规则：最终调用地址固定为 `${BaseURL}/chat/completions`
- [x] **AI 操作可观测性**
  - [x] 新增 `GET /api/ai/logs` 查看 AI 识别与写入日志
  - [x] 新增 `POST /api/ai/navigation/enrich` 手动触发导航补全
  - [x] 识别日志同步写入“消息中心”（复用通知模块）
- [x] **智能发现逻辑增强**
  - [x] 后端：改造 `discovery.go`，增加主动探测筛选 Web 端口
  - [x] 后端：集成 OpenAI 兼容 Client，对未知服务自动生成 Title/Icon/Category
  - [x] 数据库：`navigation_items` 表新增 `ai_generated` 字段
- [x] **前端展示优化**
  - [x] 导航页：支持展示 AI 标记
  - [x] 交互：保留现有回收站逻辑，支持手动修正 AI 生成项
  - [x] 交互：导航“重新识别”触发 AI 强制重识别
- [x] 交互：AI enrich 前清理孤儿 auto 导航，避免容器不存在仍被识别
- [x] 稳定性：AI enrich 增加同 navId 并发锁，避免竞态重复识别
- [x] 稳定性：端口探测增加延迟重试，避免启动期误判隐藏
- [x] 稳定性：重识别期间抑制自动 AI enrich/backfill，减少重复识别噪音
- [x] 稳定性：AI enrich 批处理互斥，避免并发重复调用
- [x] 交互：回收站内支持“彻底删除”，删除语义调整为“隐藏”
- [x] 交互：导航卡片支持单项“AI 重新识别”（按 navId 定向触发）
- [x] 优化：自动 AI 识别仅对新导航项首次补全（禁用启动时全量 backfill）
- [x] 优化：AI 导航提示词与写入策略收敛为一轮输出且避免部分写入
- [x] **MCP（受限图标能力）**
  - [x] 新增 `GET /api/mcp/icons/clay` 供 AI 查询可用图标
  - [x] 新增 `POST /api/mcp/navigation/:id/icon` 仅允许写入 icon 字段（不暴露数据库权限）
- [x] **图标格式增强**
  - [x] 上传支持 ico/avif/bmp/tif/tiff
  - [x] 前端提示同步支持列表

## 下一阶段：监控与告警（规划中）
- [ ] 集成 Prometheus 指标导出
- [ ] 增加系统资源告警配置
