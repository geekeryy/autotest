<!--
  MarkdownView
  ============
  Renders trusted (model-generated) Markdown into safe HTML. Used by AI
  analysis dialogs and the future global AI assistant. Implementation notes:

  - marked.parse() is synchronous (we explicitly disable async via the
    constructor options) so the rendered HTML is immediately available
    inside computed properties — no flicker between parsing states.
  - DOMPurify strips any unexpected tags / attributes the model may emit.
    We deliberately keep DOMPurify's default profile rather than allowing
    arbitrary HTML: the inputs come from an LLM and may try to embed
    iframes or scripts on prompt injection attempts.
  - Styling is intentionally narrow-scoped via :deep(...) so it can drop
    into any dialog without polluting global CSS.
-->
<template>
  <div class="markdown-view" v-html="html"></div>
</template>

<script>
// marked v18 把 Marked 类升成顶层 named export，旧版的 `new marked.Marked()`
// 形式会抛 "marked.Marked is not a constructor"。直接 import Marked 即可。
import { Marked } from 'marked'
import DOMPurify from 'dompurify'

const renderer = new Marked({
  async: false,
  gfm: true,
  breaks: false,
  pedantic: false
})

export default {
  name: 'MarkdownView',
  props: {
    source: { type: String, default: '' }
  },
  computed: {
    html() {
      const raw = this.source || ''
      if (!raw.trim()) return ''
      const rendered = renderer.parse(raw)
      return DOMPurify.sanitize(rendered, {
        // Disable target=_blank rewriting; we already render plain anchors
        // and Element Plus' parent dialog handles scroll containment.
        USE_PROFILES: { html: true }
      })
    }
  }
}
</script>

<style scoped>
.markdown-view {
  font-size: var(--app-font-size-base, 14px);
  line-height: 1.7;
  color: var(--el-text-color-primary, #303133);
  word-break: break-word;
}

.markdown-view :deep(h1),
.markdown-view :deep(h2),
.markdown-view :deep(h3),
.markdown-view :deep(h4),
.markdown-view :deep(h5),
.markdown-view :deep(h6) {
  font-weight: 600;
  line-height: 1.4;
  margin: 18px 0 8px;
  color: var(--el-text-color-primary, #303133);
}

.markdown-view :deep(h1) { font-size: 1.4em; }
.markdown-view :deep(h2) { font-size: 1.25em; padding-bottom: 4px; border-bottom: 1px solid var(--el-border-color-lighter, #ebeef5); }
.markdown-view :deep(h3) { font-size: 1.1em; }
.markdown-view :deep(h4),
.markdown-view :deep(h5),
.markdown-view :deep(h6) { font-size: 1em; }

.markdown-view :deep(p) {
  margin: 8px 0;
}

.markdown-view :deep(ul),
.markdown-view :deep(ol) {
  margin: 8px 0;
  padding-left: 24px;
}

.markdown-view :deep(li) {
  margin: 4px 0;
}

.markdown-view :deep(li > p) {
  margin: 4px 0;
}

.markdown-view :deep(code) {
  background: var(--el-fill-color-light, #f5f7fa);
  border: 1px solid var(--el-border-color-lighter, #ebeef5);
  border-radius: 4px;
  padding: 1px 6px;
  font-family: ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, 'Liberation Mono', 'Courier New', monospace;
  font-size: 0.92em;
  color: var(--el-color-danger, #f56c6c);
}

.markdown-view :deep(pre) {
  background: var(--el-fill-color-lighter, #fafafa);
  border: 1px solid var(--el-border-color-lighter, #ebeef5);
  border-radius: 6px;
  padding: 12px 14px;
  overflow: auto;
  margin: 10px 0;
}

.markdown-view :deep(pre code) {
  background: transparent;
  border: 0;
  padding: 0;
  color: var(--el-text-color-primary, #303133);
  font-size: 0.92em;
}

.markdown-view :deep(blockquote) {
  margin: 10px 0;
  padding: 6px 12px;
  border-left: 3px solid var(--el-border-color, #dcdfe6);
  background: var(--el-fill-color-lighter, #fafafa);
  color: var(--el-text-color-regular, #606266);
}

.markdown-view :deep(table) {
  border-collapse: collapse;
  width: 100%;
  margin: 10px 0;
  font-size: 0.95em;
}

.markdown-view :deep(th),
.markdown-view :deep(td) {
  border: 1px solid var(--el-border-color-lighter, #ebeef5);
  padding: 6px 10px;
  vertical-align: top;
}

.markdown-view :deep(th) {
  background: var(--el-fill-color-light, #f5f7fa);
  font-weight: 600;
}

.markdown-view :deep(a) {
  color: var(--el-color-primary, #409eff);
  text-decoration: none;
}

.markdown-view :deep(a:hover) {
  text-decoration: underline;
}

.markdown-view :deep(hr) {
  border: 0;
  border-top: 1px solid var(--el-border-color-lighter, #ebeef5);
  margin: 12px 0;
}

.markdown-view :deep(strong) {
  font-weight: 600;
}
</style>
