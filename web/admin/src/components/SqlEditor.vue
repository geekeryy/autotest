<template>
  <div class="sql-editor-root" :class="{ 'is-focused': focused }">
    <div class="sql-editor-toolbar">
      <span class="sql-editor-hint">{{ schemaHint }}</span>
      <el-button size="small" :disabled="!modelValue" @click="format">格式化</el-button>
    </div>
    <div ref="container" class="sql-editor-cm"></div>
  </div>
</template>

<script>
import {
  EditorView,
  keymap,
  lineNumbers,
  highlightActiveLine,
  highlightActiveLineGutter,
  placeholder as cmPlaceholder
} from '@codemirror/view'
import { EditorState } from '@codemirror/state'
import { sql, PostgreSQL } from '@codemirror/lang-sql'
import { autocompletion, completionKeymap, closeBrackets, closeBracketsKeymap } from '@codemirror/autocomplete'
import { defaultKeymap, history, historyKeymap, indentWithTab } from '@codemirror/commands'
import { indentOnInput, syntaxHighlighting, defaultHighlightStyle, bracketMatching } from '@codemirror/language'
import { format as sqlFormat } from 'sql-formatter'

export default {
  name: 'SqlEditor',
  props: {
    modelValue: { type: String, default: '' },
    placeholder: { type: String, default: 'select id, name from users where status = $1 limit 10' },
    schemaTables: { type: Array, default: () => [] },
    minRows: { type: Number, default: 6 },
    maxRows: { type: Number, default: 22 }
  },
  emits: ['update:modelValue'],
  data() {
    return {
      view: null,
      focused: false
    }
  },
  computed: {
    minHeight() {
      // line-height 21px + 16px padding
      return this.minRows * 21 + 16
    },
    maxHeight() {
      return this.maxRows * 21 + 16
    },
    schemaHint() {
      if (!this.schemaTables.length) return ''
      return `已加载 ${this.schemaTables.length} 张表`
    }
  },
  watch: {
    modelValue(val) {
      if (!this.view) return
      const current = this.view.state.doc.toString()
      if (val !== current) {
        this.view.dispatch({
          changes: { from: 0, to: current.length, insert: val ?? '' }
        })
      }
    },
    schemaTables() {
      this.rebuildEditor()
    }
  },
  mounted() {
    this.buildEditor()
  },
  beforeUnmount() {
    this.view?.destroy()
  },
  methods: {
    format() {
      const raw = this.modelValue || ''
      if (!raw.trim()) return
      try {
        const formatted = sqlFormat(raw, {
          language: 'postgresql',
          tabWidth: 2,
          keywordCase: 'lower',
          linesBetweenQueries: 1
        })
        this.$emit('update:modelValue', formatted)
        // Sync to editor
        if (this.view) {
          this.view.dispatch({
            changes: { from: 0, to: this.view.state.doc.length, insert: formatted }
          })
        }
      } catch {
        // 格式化失败时保留原文
      }
    },
    buildSchema() {
      if (!this.schemaTables?.length) return undefined
      const schema = {}
      for (const t of this.schemaTables) {
        const key = t.schema === 'public' ? t.name : `${t.schema}.${t.name}`
        schema[key] = t.columns || []
      }
      return schema
    },
    buildExtensions() {
      const schema = this.buildSchema()
      return [
        lineNumbers(),
        highlightActiveLine(),
        highlightActiveLineGutter(),
        history(),
        indentOnInput(),
        bracketMatching(),
        closeBrackets(),
        sql({ dialect: PostgreSQL, schema, upperCaseKeywords: false }),
        autocompletion({ defaultKeymap: true }),
        keymap.of([
          ...closeBracketsKeymap,
          ...defaultKeymap,
          ...historyKeymap,
          ...completionKeymap,
          indentWithTab
        ]),
        syntaxHighlighting(defaultHighlightStyle, { fallback: true }),
        cmPlaceholder(this.placeholder),
        EditorView.updateListener.of((update) => {
          if (update.docChanged) {
            this.$emit('update:modelValue', update.state.doc.toString())
          }
          if (update.focusChanged) {
            this.focused = update.view.hasFocus
          }
        }),
        EditorView.theme({
          '&': {
            fontSize: '13px',
            minHeight: this.minHeight + 'px'
          },
          '.cm-scroller': {
            fontFamily: 'ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, monospace',
            lineHeight: '1.6',
            minHeight: this.minHeight + 'px',
            maxHeight: this.maxHeight + 'px',
            overflowY: 'auto'
          },
          '.cm-content': { padding: '8px 0', minHeight: this.minHeight + 'px' },
          '.cm-focused': { outline: 'none' },
          '&.cm-focused': { outline: 'none' },
          '.cm-placeholder': {
            color: 'var(--el-text-color-placeholder)',
            fontStyle: 'italic'
          },
          '.cm-tooltip.cm-tooltip-autocomplete': { zIndex: 9999 }
        })
      ]
    },
    buildEditor() {
      this.view?.destroy()
      this.view = null
      const state = EditorState.create({
        doc: this.modelValue || '',
        extensions: this.buildExtensions()
      })
      this.view = new EditorView({ state, parent: this.$refs.container })
    },
    rebuildEditor() {
      this.buildEditor()
    }
  }
}
</script>

<style scoped>
.sql-editor-root {
  width: 100%;
  border: 1px solid var(--el-border-color);
  border-radius: 4px;
  transition: border-color 0.2s;
  background: var(--el-fill-color-blank);
}

.sql-editor-root.is-focused {
  border-color: var(--el-color-primary);
}

.sql-editor-toolbar {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 4px 8px;
  border-bottom: 1px solid var(--el-border-color-lighter);
  background: var(--el-fill-color-light);
  border-radius: 4px 4px 0 0;
}

.sql-editor-hint {
  font-size: 12px;
  color: var(--el-text-color-secondary);
}

.sql-editor-cm {
  width: 100%;
}

.sql-editor-cm :deep(.cm-editor) {
  width: 100%;
  border-radius: 0 0 4px 4px;
}

.sql-editor-cm :deep(.cm-editor.cm-focused) {
  outline: none;
}

.sql-editor-cm :deep(.cm-gutters) {
  border-right: 1px solid var(--el-border-color-lighter);
  background: var(--el-fill-color-light);
  color: var(--el-text-color-placeholder);
  border-radius: 0 0 0 4px;
}
</style>
