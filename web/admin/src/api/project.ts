/**
 * 项目、服务、环境相关 API
 */
import request, { asList } from './request'
import type { Project, Service, Environment } from '@/types/api'

// 项目
export const listProjects = (): Promise<Project[]> =>
  request.get('/projects').then(asList<Project>())

export const createProject = (data: { name: string; description?: string }): Promise<Project> =>
  request.post('/projects', data)

export const deleteProject = (id: string) =>
  request.delete(`/projects/${id}`)

// 项目成员
export const listProjectMembers = (projectId: string) =>
  request.get(`/projects/${projectId}/members`).then(asList())

export const addProjectMember = (projectId: string, data: Record<string, unknown>) =>
  request.post(`/projects/${projectId}/members`, data)

export const updateProjectMember = (projectId: string, userId: string, data: Record<string, unknown>) =>
  request.put(`/projects/${projectId}/members/${userId}`, data)

export const removeProjectMember = (projectId: string, userId: string) =>
  request.delete(`/projects/${projectId}/members/${userId}`)

// 服务
export const listServices = (projectId: string): Promise<Service[]> =>
  request.get(`/projects/${projectId}/services`).then(asList<Service>())

export const createService = (projectId: string, data: Record<string, unknown>): Promise<Service> =>
  request.post(`/projects/${projectId}/services`, data)

export const updateService = (projectId: string, serviceId: string, data: Record<string, unknown>): Promise<Service> =>
  request.put(`/projects/${projectId}/services/${serviceId}`, data)

export const deleteService = (projectId: string, serviceId: string) =>
  request.delete(`/projects/${projectId}/services/${serviceId}`)

// 环境（服务级）
export const listServiceEnvironments = (projectId: string, serviceId: string): Promise<Environment[]> =>
  request.get(`/projects/${projectId}/services/${serviceId}/environments`).then(asList<Environment>())

export const createServiceEnvironment = (projectId: string, serviceId: string, data: Record<string, unknown>): Promise<Environment> =>
  request.post(`/projects/${projectId}/services/${serviceId}/environments`, data)

export const updateServiceEnvironment = (projectId: string, serviceId: string, environmentId: string, data: Record<string, unknown>): Promise<Environment> =>
  request.put(`/projects/${projectId}/services/${serviceId}/environments/${environmentId}`, data)

export const deleteServiceEnvironment = (projectId: string, serviceId: string, environmentId: string) =>
  request.delete(`/projects/${projectId}/services/${serviceId}/environments/${environmentId}`)

// 环境（项目级）
export const listEnvironments = (projectId: string): Promise<Environment[]> =>
  request.get(`/projects/${projectId}/environments`).then(asList<Environment>())

export const createEnvironment = (projectId: string, data: Record<string, unknown>): Promise<Environment> =>
  request.post(`/projects/${projectId}/environments`, data)

export const updateEnvironment = (projectId: string, environmentId: string, data: Record<string, unknown>): Promise<Environment> =>
  request.put(`/projects/${projectId}/environments/${environmentId}`, data)

export const deleteEnvironment = (projectId: string, environmentId: string) =>
  request.delete(`/projects/${projectId}/environments/${environmentId}`)
