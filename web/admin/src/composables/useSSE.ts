/**
 * SSE 流式通信 Composable
 * 替代 utils/aiStream.js，使用 Composition API 重构
 *
 * 使用 fetch + ReadableStream 实现 SSE（因为需要 POST + Authorization header）
 */
import { ref } from 'vue'

export interface SSECallbacks {
  onEvent?: (event: string, data: unknown) => void
  onText?: (delta: string) => void
  onThinking?: (delta: string) => void
  onToolCall?: (toolCallId: string, name: string, args: Record<string, unknown>) => void
  onToolResult?: (toolCallId: string, output: string, isError: boolean) => void
  onDone?: (tokenUsage?: unknown) => void
  onError?: (error: string) => void
}

export function useSSE() {
  const isStreaming = ref(false)
  const error = ref<string>('')
  let abortController: AbortController | null = null

  /**
   * 发起 SSE 流式请求
   */
  async function startStream(
    url: string,
    body: Record<string, unknown>,
    callbacks: SSECallbacks
  ): Promise<void> {
    // 中止之前的请求
    abort()

    abortController = new AbortController()
    isStreaming.value = true
    error.value = ''

    try {
      const token = localStorage.getItem('autotest_admin_token')
      const response = await fetch(url, {
        method: 'POST',
        headers: {
          'Content-Type': 'application/json',
          ...(token ? { Authorization: `Bearer ${token}` } : {}),
        },
        body: JSON.stringify(body),
        signal: abortController.signal,
      })

      if (!response.ok) {
        const errorText = await response.text()
        throw new Error(errorText || `HTTP ${response.status}`)
      }

      const reader = response.body?.getReader()
      if (!reader) throw new Error('No response body')

      const decoder = new TextDecoder()
      let buffer = ''

      while (true) {
        const { done, value } = await reader.read()
        if (done) break

        buffer += decoder.decode(value, { stream: true })
        const lines = buffer.split('\n')
        buffer = lines.pop() || '' // 保留未完成的行

        let currentEvent = ''
        for (const line of lines) {
          if (line.startsWith(':')) continue // 心跳帧
          if (line.startsWith('event: ')) {
            currentEvent = line.slice(7).trim()
          } else if (line.startsWith('data: ')) {
            const dataStr = line.slice(6)
            try {
              const data = JSON.parse(dataStr)
              handleEvent(currentEvent, data, callbacks)
            } catch {
              // 非 JSON data，直接传递
              handleEvent(currentEvent, dataStr, callbacks)
            }
          }
        }
      }
    } catch (err: unknown) {
      if (err instanceof Error && err.name !== 'AbortError') {
        error.value = err.message
        callbacks.onError?.(err.message)
      }
    } finally {
      isStreaming.value = false
      abortController = null
    }
  }

  /**
   * 分发 SSE 事件
   */
  function handleEvent(event: string, data: unknown, callbacks: SSECallbacks) {
    callbacks.onEvent?.(event, data)

    const parsed = typeof data === 'string' ? { data } : data as Record<string, unknown>

    switch (event) {
      case 'text_delta':
        callbacks.onText?.(parsed.delta as string || '')
        break
      case 'thinking_delta':
        callbacks.onThinking?.(parsed.delta as string || '')
        break
      case 'tool_call':
        callbacks.onToolCall?.(
          parsed.toolCallId as string,
          parsed.name as string,
          parsed.arguments as Record<string, unknown>
        )
        break
      case 'tool_result':
        callbacks.onToolResult?.(
          parsed.toolCallId as string,
          parsed.output as string,
          parsed.isError as boolean
        )
        break
      case 'done':
        callbacks.onDone?.(parsed.tokenUsage)
        break
      case 'error':
        error.value = parsed.error as string || 'Unknown error'
        callbacks.onError?.(parsed.error as string || 'Unknown error')
        break
    }
  }

  /**
   * 中止当前流
   */
  function abort() {
    if (abortController) {
      abortController.abort()
      abortController = null
      isStreaming.value = false
    }
  }

  return {
    isStreaming,
    error,
    startStream,
    abort,
  }
}
