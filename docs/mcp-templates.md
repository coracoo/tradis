# MCP：Templates 批量导入与白名单维护

## 服务端地址
- 模板服务端（server/backend）：`http(s)://<host>:3002`
- 所有 API 均以 `/api` 为前缀

## MCP 能力总览
本项目里 “MCP” 一词有两类能力：
- 模板服务端（server/backend）：对外提供 `templates/import` 批量导入，受 IP 白名单与可选 MCP Token 保护
- 面板后端（client/backend）：仅提供受限能力（Clay 图标清单、写入导航 icon），受面板自身登录鉴权保护

## 鉴权与安全
### IP 白名单
- 管理写接口（模板增删改、上传、同步、白名单维护等）受 **模板写接口白名单** 限制
  - 后端读取顺序：DB（ServerKV: `admin_allowlist`）优先，其次环境变量 `ADMIN_ALLOWLIST`
- MCP 导入接口受 **MCP 白名单** 限制
  - 后端读取顺序：DB（ServerKV: `mcp_allowlist`）优先，其次环境变量 `MCP_ALLOWLIST`（默认回退到 `ADMIN_ALLOWLIST`）

支持两种格式（逗号或换行分隔均可）：\n- 单 IP：`127.0.0.1`\n- CIDR：`192.168.0.0/16`

### MCP Token（可选）
- 当 DB（ServerKV: `mcp_token`）或环境变量 `MCP_TOKEN` 设置了 token 时，调用 MCP 接口必须携带请求头：
  - `X-MCP-Token: <token>`
- 为空时仅依赖 IP 白名单

## 管理接口（仅白名单可访问）
### 读取/更新模板写接口白名单
- `GET /api/admin/allowlist`
- `PUT /api/admin/allowlist`

请求体：
```json
{ "raw": "127.0.0.1,::1,192.168.0.0/16" }
```

### 读取/更新 MCP 白名单
- `GET /api/admin/mcp-allowlist`
- `PUT /api/admin/mcp-allowlist`

### 读取/更新 MCP Token
- `GET /api/admin/mcp-token`
- `PUT /api/admin/mcp-token`

请求体：
```json
{ "token": "your-token" }
```

## MCP 批量导入接口
### 导入（支持 upsert / dry-run）
- `POST /api/mcp/templates/import`

请求头：
- `Content-Type: application/json`
- `X-MCP-Token: <token>`（当服务端配置了 token 时必填）

请求体：
```json
{
  "mode": "upsert_by_name",
  "dryRun": false,
  "templates": [
    {
      "name": "nginx",
      "category": "network",
      "description": "Nginx",
      "version": "latest",
      "website": "https://nginx.org/",
      "logo": "https://nginx.org/img/nginx_logo.svg",
      "tutorial": "",
      "dotenv": "NGINX_HOST=example.com\nNGINX_PORT=80\n",
      "compose": "services:\n  web:\n    image: nginx\n    ports:\n      - \"8080:80\"\n",
      "screenshots": [],
      "enabled": true
    }
  ]
}
```

字段说明：
- `mode`：\n  - `upsert_by_name`：按 name 存在则更新，不存在则创建（默认）\n  - `create_only`：只创建，不覆盖已存在模板\n- `dryRun`：为 `true` 时只返回“会创建/更新/跳过”的统计，不落库\n- 单条模板：\n  - `schema` 可选：为空时服务端会从 compose 解析生成\n  - `dotenv_json` 可选：若 `dotenv` 为空会用它合成 dotenv 文本\n
返回体：
```json
{
  "created": 1,
  "updated": 0,
  "skipped": 0,
  "errors": []
}
```

失败返回（HTTP 400）会附带每条模板的错误明细：
```json
{
  "created": 0,
  "updated": 0,
  "skipped": 0,
  "errors": [
    { "name": "nginx", "error": "YAML 解析失败：..." }
  ]
}
```

## AI 工具定义（建议）
如果你的 MCP 网关/工作流系统支持“HTTP Tool 封装”，可以按以下工具定义封装：\n- `templates_import` -> `POST /api/mcp/templates/import`\n- `templates_list` -> `GET /api/templates`\n- `templates_get` -> `GET /api/templates/:id`\n- `templates_get_vars` -> `GET /api/templates/:id/vars`\n
建议在网关侧把 `baseUrl`、`X-MCP-Token`、以及 IP 白名单策略作为环境配置注入。\n

## 面板后端（client/backend）受限 MCP 能力
### Clay 图标清单
- `GET /api/mcp/icons/clay`
- 返回：
```json
{ "items": [ { "name": "xxx.png", "value": "/icons/clay/xxx.png" } ] }
```

### 写入导航图标
- `POST /api/mcp/navigation/:id/icon`
- 请求体：
```json
{ "icon": "mdi-docker" }
```
- `icon` 允许值：
  - `mdi-xxx`
  - `/icons/clay/<filename>`
  - `http(s)://...`
  - `/data/pic/...` 或 `/uploads/icons/...`
