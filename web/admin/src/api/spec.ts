/**
 * OpenAPI Spec 相关 API
 */
import request from './request'
import type { APISpec, Endpoint, ImportSummary } from '@/types/api'

export const importSpec = (
  projectId: string,
  serviceId: string,
  content: string | Blob,
  syncMode: 'merge' | 'overwrite' = 'merge'
): Promise<ImportSummary> => {
  const params = syncMode !== 'merge' ? { sync: syncMode } : {}
  return request.post(`/projects/${projectId}/services/${serviceId}/specs/import`, content, {
    headers: { 'Content-Type': 'application/yaml' },
    params,
  })
}

export const listSpecs = (projectId: string, serviceId: string): Promise<APISpec[]> =>
  request.get(`/projects/${projectId}/services/${serviceId}/specs`)

export const listEndpoints = (projectId: string, serviceId: string): Promise<Endpoint[]> =>
  request.get(`/projects/${projectId}/services/${serviceId}/endpoints`)
