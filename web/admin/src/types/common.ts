/** 通用分页参数 */
export interface PaginationParams {
  page: number
  pageSize: number
}

/** 通用分页结果 */
export interface PaginatedResult<T> {
  items: T[]
  total: number
  page: number
  pageSize: number
}

/** 通用列表筛选 */
export interface ListFilter {
  projectId?: string
  serviceId?: string
  keyword?: string
}

/** 时间范围筛选 */
export interface DateRangeFilter {
  startDate?: string
  endDate?: string
}

/** UUID 字符串别名 */
export type UUID = string

/** JSON 原始值 */
export type JsonValue = string | number | boolean | null | JsonValue[] | { [key: string]: JsonValue }

/** HTTP 方法 */
export type HttpMethod = 'GET' | 'POST' | 'PUT' | 'PATCH' | 'DELETE' | 'HEAD' | 'OPTIONS'

/** 通用操作结果 */
export interface OperationResult {
  success: boolean
  message?: string
}

/** 排序方向 */
export type SortDirection = 'asc' | 'desc'

/** 排序参数 */
export interface SortParams {
  field: string
  direction: SortDirection
}
