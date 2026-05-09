<template>
  <div class="json-body-editor" :class="{ 'is-focused': focused }">
    <div ref="container" class="json-body-editor-cm"></div>
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
import { closeBrackets, closeBracketsKeymap } from '@codemirror/autocomplete'
import { defaultKeymap, history, historyKeymap, indentWithTab } from '@codemirror/commands'
import {
  bracketMatching,
  defaultHighlightStyle,
  foldGutter,
  indentOnInput,
  syntaxHighlighting
} from '@codemirror/language'
import { json } from '@codemirror/lang-json'

export default {
  name: 'JsonBodyEditor',
  props: {
    modelValue: { type: String, default: '' },
    placeholder: { type: String, default: '{\n  "name": "value"\n}' },
    minRows: { type: Number, default: 12 },
    maxRows: { type: Number, default: 24 }
  },
  emits: ['update:modelValue', 'blur'],
  data() {
    return {
      view: null,
      focused: false
    }
  },
  computed: {
    minHeight() {
      return this.minRows * 21 + 28
    },
    maxHeight() {
      return this.maxRows * 21 + 28
    }
  },
  watch: {
    modelValue(val) {
      if (!this.view) return
      const current = this.view.state.doc.toString()
      if ((val ?? '') !== current) {
        this.view.dispatch({
          changes: { from: 0, to: current.length, insert: val ?? '' }
        })
      }
    },
    minRows() {
      this.rebuildEditor()
    },
    maxRows() {
      this.rebuildEditor()
    }
  },
  mounted() {
    this.buildEditor()
  },
  beforeUnmount() {
    this.view?.destroy()
    this.view = null
  },
  methods: {
    buildExtensions() {
      return [
        lineNumbers(),
        foldGutter(),
        highlightActiveLine(),
        highlightActiveLineGutter(),
        history(),
        indentOnInput(),
        bracketMatching(),
        closeBrackets(),
        json(),
        EditorView.lineWrapping,
        keymap.of([
          ...closeBracketsKeymap,
          ...defaultKeymap,
          ...historyKeymap,
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
            if (!update.view.hasFocus) this.$emit('blur')
          }
        }),
        EditorView.theme({
          '&': {
            minHeight: `${this.minHeight}px`,
            fontSize: 'var(--app-font-size-small, 13px)'
          },
          '.cm-scroller': {
            minHeight: `${this.minHeight}px`,
            maxHeight: `${this.maxHeight}px`,
            overflowY: 'auto',
            overflowX: 'hidden',
            fontFamily: 'ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, monospace',
            lineHeight: '1.6'
          },
          '.cm-content': {
            minHeight: `${this.minHeight}px`,
            padding: '14px 0'
          },
          '.cm-line': {
            padding: '0 12px'
          },
          '.cm-gutters': {
            borderRight: '1px solid var(--app-border-color)',
            background: 'color-mix(in srgb, var(--app-code-bg) 92%, #ffffff)',
            color: 'var(--el-text-color-placeholder)'
          },
          '.cm-foldGutter .cm-gutterElement': {
            cursor: 'pointer'
          },
          '.cm-activeLine, .cm-activeLineGutter': {
            backgroundColor: 'rgba(64, 158, 255, 0.08)'
          },
          '.cm-placeholder': {
            color: 'var(--el-text-color-placeholder)',
            fontStyle: 'italic'
          },
          '&.cm-focused': {
            outline: 'none'
          }
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
    },
    focus() {
      this.view?.focus()
    }
  }
}
</script>

<style scoped>
.json-body-editor {
  min-width: 0;
  border: 1px solid var(--app-border-color);
  border-radius: 10px;
  background: var(--app-code-bg);
  transition: border-color 0.16s ease, box-shadow 0.16s ease;
}

.json-body-editor.is-focused {
  border-color: var(--el-color-primary-light-3);
  box-shadow: 0 0 0 1px var(--el-color-primary-light-7);
}

.json-body-editor-cm {
  width: 100%;
}

.json-body-editor-cm :deep(.cm-editor) {
  width: 100%;
  border-radius: 10px;
  background: transparent;
}

.json-body-editor-cm :deep(.cm-editor.cm-focused) {
  outline: none;
}

.json-body-editor-cm :deep(.cm-gutters) {
  border-radius: 10px 0 0 10px;
}
</style>
