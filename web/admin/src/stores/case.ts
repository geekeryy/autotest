/**
 * 用例状态 Store
 * 管理用例列表、编辑状态等
 */
import { defineStore } from 'pinia'
import { ref } from 'vue'
import type { UUID } from '@/types/common'
import type {
  TestCase,
  CaseListFilter,
  CreateManualInput,
  PatchCaseInput,
  GeneratedParams,
  InferredAssertion,
} from '@/types/case'
import * as caseApi from '@/api/case'

export const useCaseStore = defineStore('case', () => {
  // 状态
  const cases = ref<TestCase[]>([])
  const loading = ref(false)
  const currentFilter = ref<CaseListFilter | null>(null)

  // ── 用例 CRUD ──────────────────────────────────────────────────
  async function loadCases(filter: CaseListFilter) {
    currentFilter.value = filter
    loading.value = true
    try {
      cases.value = await caseApi.listCases(filter)
    } catch {
      cases.value = []
    } finally {
      loading.value = false
    }
  }

  async function getCase(id: UUID): Promise<TestCase | null> {
    try {
      return await caseApi.getCase(id)
    } catch {
      return null
    }
  }

  async function createManualCase(input: CreateManualInput): Promise<TestCase | null> {
    try {
      const testCase = await caseApi.createManualCase(input)
      cases.value.unshift(testCase)
      return testCase
    } catch {
      return null
    }
  }

  async function updateCase(id: UUID, input: PatchCaseInput): Promise<TestCase | null> {
    try {
      const testCase = await caseApi.patchCase(id, input)
      const idx = cases.value.findIndex(c => c.id === id)
      if (idx >= 0) cases.value[idx] = testCase
      return testCase
    } catch {
      return null
    }
  }

  async function generateParams(id: UUID): Promise<GeneratedParams | null> {
    try {
      return await caseApi.generateParams(id)
    } catch {
      return null
    }
  }

  // ── AI 断言推断 ──────────────────────────────────────────────
  async function inferAssertions(id: UUID): Promise<InferredAssertion[]> {
    try {
      const output = await caseApi.inferAssertions(id)
      return output?.assertions ?? []
    } catch {
      return []
    }
  }

  async function applyAssertions(id: UUID, assertions: InferredAssertion[], mode: 'replace' | 'append'): Promise<boolean> {
    try {
      await caseApi.applyAssertions(id, { assertions, mode })
      // 重新加载用例
      const updated = await caseApi.getCase(id)
      if (updated) {
        const idx = cases.value.findIndex(c => c.id === id)
        if (idx >= 0) cases.value[idx] = updated
      }
      return true
    } catch {
      return false
    }
  }

  return {
    cases,
    loading,
    currentFilter,
    loadCases,
    getCase,
    createManualCase,
    updateCase,
    generateParams,
    inferAssertions,
    applyAssertions,
  }
})
