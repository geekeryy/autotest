/**
 * 断言编辑 Composable
 * 从 CaseRun.vue 中提取的断言编辑逻辑
 */
import { ref } from 'vue'
import type { Assertion, AssertionType, ComparisonOperator } from '@/types/scenario'

const DEFAULT_OPERATORS: Record<AssertionType, ComparisonOperator> = {
  status_code: 'equals',
  jsonpath: 'equals',
  header: 'equals',
  body_contains: 'contains',
  response_time: 'lte',
  script: 'equals',
  schema: 'equals',
}

export function useAssertion() {
  const assertions = ref<Assertion[]>([])

  function addAssertion(type: AssertionType) {
    assertions.value.push({
      type,
      operator: DEFAULT_OPERATORS[type],
      expression: '',
      expected: undefined,
      script: type === 'script' ? '// pm.test("test name", function() {\n//   pm.expect(pm.response.status).to.equal(200);\n// });' : undefined,
    })
  }

  function removeAssertion(index: number) {
    assertions.value.splice(index, 1)
  }

  function updateAssertion(index: number, patch: Partial<Assertion>) {
    if (index >= 0 && index < assertions.value.length) {
      assertions.value[index] = { ...assertions.value[index], ...patch }
    }
  }

  function moveAssertion(from: number, to: number) {
    if (from < 0 || from >= assertions.value.length) return
    if (to < 0 || to >= assertions.value.length) return
    const [item] = assertions.value.splice(from, 1)
    assertions.value.splice(to, 0, item)
  }

  function setAssertions(items: Assertion[]) {
    assertions.value = [...items]
  }

  function clear() {
    assertions.value = []
  }

  function toJSON(): string {
    return JSON.stringify(assertions.value)
  }

  return {
    assertions,
    addAssertion,
    removeAssertion,
    updateAssertion,
    moveAssertion,
    setAssertions,
    clear,
    toJSON,
  }
}
