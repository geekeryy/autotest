/**
 * 测试数据、数据源、SQL 参数源相关 API
 */
import request, { asList } from './request'
import type { DataSource, SQLParameterSource } from '@/types/api'

// 数据源
export const listDataSources = (): Promise<DataSource[]> =>
  request.get('/data-sources').then(asList<DataSource>())

export const createDataSource = (data: Record<string, unknown>): Promise<DataSource> =>
  request.post('/data-sources', data)

export const updateDataSource = (id: string, data: Record<string, unknown>): Promise<DataSource> =>
  request.put(`/data-sources/${id}`, data)

export const deleteDataSource = (id: string) =>
  request.delete(`/data-sources/${id}`)

export const testDataSource = (id: string) =>
  request.post(`/data-sources/${id}/test`)

export const getDataSourceSchema = (id: string) =>
  request.get(`/data-sources/${id}/schema`).then(asList())

// SQL 参数源
export const listSQLParameterSources = (params: Record<string, unknown> = {}): Promise<SQLParameterSource[]> =>
  request.get('/sql-parameter-sources', { params }).then(asList<SQLParameterSource>())

export const createSQLParameterSource = (data: Record<string, unknown>): Promise<SQLParameterSource> =>
  request.post('/sql-parameter-sources', data)

export const updateSQLParameterSource = (id: string, data: Record<string, unknown>): Promise<SQLParameterSource> =>
  request.put(`/sql-parameter-sources/${id}`, data)

export const deleteSQLParameterSource = (id: string) =>
  request.delete(`/sql-parameter-sources/${id}`)

export const previewSQLParameterSource = (id: string, data: Record<string, unknown> = {}) =>
  request.post(`/sql-parameter-sources/${id}/preview`, data)

export const previewSQLParameterSourceDraft = (data: Record<string, unknown>) =>
  request.post('/sql-parameter-sources/preview', data)

// 测试数据表
export const listTestDataTables = (projectId: string) =>
  request.get(`/projects/${projectId}/test-data-tables`).then(asList())

export const getTestDataTable = (projectId: string, tableId: string) =>
  request.get(`/projects/${projectId}/test-data-tables/${tableId}`)

export const createTestDataTable = (projectId: string, data: Record<string, unknown>) =>
  request.post(`/projects/${projectId}/test-data-tables`, data)

export const updateTestDataTable = (projectId: string, tableId: string, data: Record<string, unknown>) =>
  request.put(`/projects/${projectId}/test-data-tables/${tableId}`, data)

export const deleteTestDataTable = (projectId: string, tableId: string) =>
  request.delete(`/projects/${projectId}/test-data-tables/${tableId}`)

export const listTestDataRows = (projectId: string, tableId: string) =>
  request.get(`/projects/${projectId}/test-data-tables/${tableId}/rows`).then(asList())

export const replaceTestDataRows = (projectId: string, tableId: string, rows: unknown[]) =>
  request.put(`/projects/${projectId}/test-data-tables/${tableId}/rows`, { rows })

export const generateTestDataRows = (projectId: string, tableId: string, data: { rowCount: number; preserveManual?: boolean }) =>
  request.post(`/projects/${projectId}/test-data-tables/${tableId}/rows/generate`, {
    rowCount: Number(data?.rowCount) || 1,
    preserveManual: !!data?.preserveManual,
  }, { timeout: 120000 })

export const regenerateTestDataCells = (projectId: string, tableId: string, rowId: string, data: { columnKeys: string[] }) =>
  request.post(`/projects/${projectId}/test-data-tables/${tableId}/rows/${rowId}/regenerate`, {
    columnKeys: Array.isArray(data?.columnKeys) ? data.columnKeys.slice() : [],
  }, { timeout: 120000 })

export const listMockHelpers = () =>
  request.get('/test-data/mock-helpers').then(asList())
