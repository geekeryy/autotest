/**
 * 请求编辑 Composable
 * 从 CaseRun.vue 中提取的请求编辑逻辑
 */
import { ref, computed } from 'vue'
import type { HttpMethod } from '@/types/common'
import type { RequestDefinition } from '@/types/case'

export function useRequestEditor() {
  const method = ref<HttpMethod>('GET')
  const path = ref('')
  const headers = ref<Record<string, string>>({})
  const query = ref<Record<string, string>>({})
  const body = ref<unknown>(null)
  const bodyType = ref<'json' | 'form' | 'raw'>('json')

  const hasBody = computed(() => ['POST', 'PUT', 'PATCH'].includes(method.value))

  function setRequest(req: RequestDefinition) {
    method.value = req.method
    path.value = req.path
    headers.value = { ...req.headers }
    query.value = { ...req.query }
    body.value = req.body
  }

  function getRequest(): RequestDefinition {
    return {
      method: method.value,
      path: path.value,
      headers: { ...headers.value },
      query: { ...query.value },
      body: body.value,
    }
  }

  function addHeader(key: string, value: string) {
    headers.value[key] = value
  }

  function removeHeader(key: string) {
    delete headers.value[key]
  }

  function addQueryParam(key: string, value: string) {
    query.value[key] = value
  }

  function removeQueryParam(key: string) {
    delete query.value[key]
  }

  function setBody(value: unknown) {
    body.value = value
  }

  return {
    method,
    path,
    headers,
    query,
    body,
    bodyType,
    hasBody,
    setRequest,
    getRequest,
    addHeader,
    removeHeader,
    addQueryParam,
    removeQueryParam,
    setBody,
  }
}
