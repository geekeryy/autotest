/**
 * 脚本库模板分组与按场景筛选。内置模板由后端写入数据库并通过列表接口返回。
 * @param {'assertion'|'scenario'} variant
 * @param {Array<{scopes?: string[], name?: string}>} templates 列表接口返回的全部模板（内置 + 当前项目自定义）
 */
export function listTemplatesForVariant(variant, templates = []) {
  const v = variant === 'scenario' ? 'scenario' : 'assertion'
  const list = templates || []
  return list.filter((t) => (t.scopes || []).includes(v))
}

export function groupTemplatesByCategory(templates) {
  const map = new Map()
  for (const t of templates) {
    const cat = t.category || '其他'
    if (!map.has(cat)) map.set(cat, [])
    map.get(cat).push(t)
  }
  return Array.from(map.entries())
}
