export function getListLoadErrorMessage(err, fallback = '加载失败，请重试') {
  return err?.response?.data?.error || err?.message || fallback
}
