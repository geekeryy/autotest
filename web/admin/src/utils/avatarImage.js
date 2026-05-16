/**
 * 浏览器端将图片压成 JPEG（最长边 maxDim），返回纯 base64（无 data URL 前缀）。
 * @param {File} file
 * @param {{ maxDim?: number, quality?: number }} [opts]
 * @returns {Promise<string>}
 */
export function compressAvatarToJpegBase64(file, opts = {}) {
  const maxDim = opts.maxDim ?? 384
  const quality = opts.quality ?? 0.82
  return new Promise((resolve, reject) => {
    if (!file || !file.type || !file.type.startsWith('image/')) {
      reject(new Error('请选择图片文件'))
      return
    }
    const reader = new FileReader()
    reader.onerror = () => reject(new Error('读取文件失败'))
    reader.onload = (e) => {
      const url = e.target?.result
      if (typeof url !== 'string') {
        reject(new Error('读取文件失败'))
        return
      }
      const img = new Image()
      img.onload = () => {
        let w = img.naturalWidth || img.width
        let h = img.naturalHeight || img.height
        if (!w || !h) {
          reject(new Error('无效图片'))
          return
        }
        const scale = Math.min(1, maxDim / w, maxDim / h)
        w = Math.max(1, Math.round(w * scale))
        h = Math.max(1, Math.round(h * scale))
        const canvas = document.createElement('canvas')
        canvas.width = w
        canvas.height = h
        const ctx = canvas.getContext('2d')
        if (!ctx) {
          reject(new Error('无法创建画布'))
          return
        }
        ctx.drawImage(img, 0, 0, w, h)
        const dataUrl = canvas.toDataURL('image/jpeg', quality)
        const i = dataUrl.indexOf('base64,')
        if (i < 0) {
          reject(new Error('压缩失败'))
          return
        }
        resolve(dataUrl.slice(i + 7))
      }
      img.onerror = () => reject(new Error('无法解析图片'))
      img.src = url
    }
    reader.readAsDataURL(file)
  })
}
