/**
 * API 层统一导出
 * 按模块拆分后的 barrel 文件，保持向后兼容
 *
 * 新代码建议直接从具体模块导入：
 *   import { listScenarios } from '@/api/scenario'
 *
 * 旧代码可以继续使用：
 *   import { listScenarios } from '@/api'
 */
export { apiClient, asList } from './request'

// 认证
export * from './auth'

// 项目/服务/环境
export * from './project'

// Spec
export * from './spec'

// 用例
export * from './case'

// 场景
export * from './scenario'

// Mock
export * from './mock'

// 测试数据
export * from './testdata'

// AI
export * from './ai'

// 报告
export * from './report'

// 管理
export * from './admin'
