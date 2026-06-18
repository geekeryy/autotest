/**
 * API 层通用类型定义
 * 与后端 handler 层的请求/响应结构对齐
 */

/** 后端统一错误响应 */
export interface ApiError {
  error: string
  message?: string
  code?: string
}

/** 分页查询参数 */
export interface PaginationQuery {
  page?: number
  pageSize?: number
}

/** 项目 */
export interface Project {
  id: string
  name: string
  description?: string
  createdAt: string
  updatedAt: string
}

/** 服务 */
export interface Service {
  id: string
  projectId: string
  name: string
  description?: string
  mcpEnabled?: boolean
  baseURL?: string
  createdAt: string
  updatedAt: string
}

/** 服务 MCP 接入说明（GET .../services/{id}/mcp-integration） */
export interface McpIntegrationGuide {
  serviceMcpEnabled: boolean
  serverMcpHttpEnabled: boolean
  mcpHttpPath: string
  mcpHttpURL: string
  apiBaseUrl: string
  projectId: string
  serviceId: string
  defaultEnvironmentId?: string
  environments: { id: string; name: string }[]
  requiredScopes: string[]
  cursorHttp: Record<string, unknown> | string
  cursorStdio: Record<string, unknown> | string
  cursorInstallHttpLink?: string
  cursorInstallStdioLink?: string
  cursorServerName?: string
  apiKeyToken?: string
  apiKeyMask?: string
  apiKeyId?: string
  serverEnvHint: string
}

/** 环境认证 Profile */
export interface AuthProfile {
  name: string
  authType: 'none' | 'bearer' | 'basic' | 'api_key' | 'header' | 'query' | 'jwt' | 'oauth2'
  config: Record<string, unknown>
}

/** 环境路径规则 */
export interface PathRule {
  prefix: string
  profileName: string
}

/** 环境 */
export interface Environment {
  id: string
  projectId: string
  serviceId: string
  name: string
  variables: Record<string, unknown>
  auth?: {
    profiles: AuthProfile[]
    defaultProfile?: string
    securitySchemes?: Record<string, string>
    pathRules?: PathRule[]
  }
  createdAt: string
  updatedAt: string
}

/** OpenAPI Spec 版本 */
export interface APISpec {
  id: string
  projectId: string
  serviceId: string
  version: number
  contentHash: string
  createdAt: string
}

/** API 端点 */
export interface Endpoint {
  id: string
  projectId: string
  serviceId: string
  specId?: string
  method: string
  path: string
  summary?: string
  description?: string
  tags?: string[]
  operationId?: string
  requestSchema?: Record<string, unknown>
  responseSchema?: Record<string, unknown>
  security?: Record<string, unknown>[]
  status: 'active' | 'archived'
  createdAt: string
  updatedAt: string
}

/** Spec 导入摘要 */
export interface ImportSummary {
  specId: string
  version: number
  endpointsCreated: number
  endpointsUpdated: number
  endpointsArchived: number
  casesGenerated: number
}

/** 用户 */
export interface User {
  id: string
  username: string
  email?: string
  displayName?: string
  role: string
  status: 'active' | 'pending' | 'disabled'
  createdAt: string
}

/** 角色 */
export interface Role {
  id: string
  name: string
  description?: string
  permissions: string[]
}

/** API Key */
export interface ApiKey {
  id: string
  name: string
  prefix: string
  createdAt: string
  lastUsedAt?: string
}

/** 审计日志 */
export interface AuditLog {
  id: string
  userId: string
  username: string
  action: string
  resourceType: string
  resourceId?: string
  details?: Record<string, unknown>
  createdAt: string
}

/** 通知 */
export interface Notification {
  id: string
  type: string
  title: string
  message: string
  read: boolean
  createdAt: string
}

/** AI 提供商 */
export interface AIProvider {
  id: string
  name: string
  provider: string
  model: string
  isDefault: boolean
  enabled: boolean
  config: Record<string, unknown>
}

/** 数据源 */
export interface DataSource {
  id: string
  name: string
  dbType: string
  host: string
  port: number
  database: string
  username: string
  createdAt: string
}

/** SQL 参数源 */
export interface SQLParameterSource {
  id: string
  projectId: string
  serviceId: string
  dataSourceId: string
  name: string
  key: string
  sql: string
  params?: Record<string, unknown>
  createdAt: string
  updatedAt: string
}
