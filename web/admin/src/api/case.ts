/**
 * 用例管理相关 API
 */
import request, { asList } from './request'
import type {
  TestCase,
  CaseListFilter,
  CreateManualInput,
  PatchCaseInput,
  GeneratedParams,
  InferredAssertion,
  ApplyAssertionsInput,
  InferAssertionsOutput,
} from '@/types/case'

export const listCases = (params: CaseListFilter = {} as CaseListFilter): Promise<TestCase[]> =>
  request.get('/cases', { params }).then(asList<TestCase>())

export const getCase = (caseId: string): Promise<TestCase> =>
  request.get(`/cases/${caseId}`)

export const patchCase = (caseId: string, data: PatchCaseInput): Promise<TestCase> =>
  request.patch(`/cases/${caseId}`, data)

export const createCase = (data: CreateManualInput): Promise<TestCase> =>
  request.post('/cases', data)

export const createManualCase = (data: CreateManualInput): Promise<TestCase> =>
  request.post('/cases', data)

export const listSavedCases = (caseId: string): Promise<TestCase[]> =>
  request.get(`/cases/${caseId}/saved-cases`).then(asList<TestCase>())

export const createSavedCase = (caseId: string, data: Record<string, unknown>): Promise<TestCase> =>
  request.post(`/cases/${caseId}/saved-cases`, data)

export const deleteSavedCase = (caseId: string, savedCaseId: string) =>
  request.delete(`/cases/${caseId}/saved-cases/${savedCaseId}`)

export const generateParams = (caseId: string): Promise<GeneratedParams> =>
  request.get(`/cases/${caseId}/generate-params`)

export const generateCaseParams = (caseId: string): Promise<GeneratedParams> =>
  request.get(`/cases/${caseId}/generate-params`)

export const runCase = (caseId: string, data: Record<string, unknown>) =>
  request.post(`/cases/${caseId}/run`, data)

// AI 断言推断
export const inferAssertions = (
  caseId: string,
  data?: { sampleCount?: number; sources?: string[] },
): Promise<InferAssertionsOutput> =>
  request.post(`/cases/${caseId}/infer-assertions`, data ?? {})

export const applyAssertions = (caseId: string, data: ApplyAssertionsInput) =>
  request.post(`/cases/${caseId}/apply-assertions`, data)
