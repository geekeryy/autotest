const MAX_IMAGES = 4
const MAX_BYTES = 5 * 1024 * 1024
const ALLOWED_TYPES = new Set(['image/jpeg', 'image/png', 'image/gif', 'image/webp'])

export function parseMessageAttachments(raw) {
  if (!raw) return []
  try {
    const data = typeof raw === 'string' ? JSON.parse(raw) : raw
    if (!Array.isArray(data)) return []
    return data.filter((item) => item?.type === 'image' && String(item.url || '').trim())
  } catch {
    return []
  }
}

export function readImageFileAsDataURL(file) {
  return new Promise((resolve, reject) => {
    if (!file || !ALLOWED_TYPES.has(file.type)) {
      reject(new Error('仅支持 JPEG、PNG、GIF、WebP 图片'))
      return
    }
    if (file.size > MAX_BYTES) {
      reject(new Error('单张图片不能超过 5MB'))
      return
    }
    const reader = new FileReader()
    reader.onload = () => {
      const url = String(reader.result || '')
      if (!url.startsWith('data:image/')) {
        reject(new Error('图片读取失败'))
        return
      }
      resolve({ url, name: file.name || 'image' })
    }
    reader.onerror = () => reject(new Error('图片读取失败'))
    reader.readAsDataURL(file)
  })
}

export async function pickImageAttachments(existingCount, files) {
  const list = Array.from(files || [])
  if (!list.length) return []
  const remaining = MAX_IMAGES - (existingCount || 0)
  if (remaining <= 0) {
    throw new Error(`最多上传 ${MAX_IMAGES} 张图片`)
  }
  const picked = list.slice(0, remaining)
  const out = []
  for (const file of picked) {
    out.push(await readImageFileAsDataURL(file))
  }
  return out
}
