<template>
  <span
    class="ai-sparkle-icon"
    :style="{ fontSize }"
    role="img"
    aria-label="AI"
    aria-hidden="true"
  >✨</span>
</template>

<script>
export default {
  name: 'AiSparkleIcon',
  props: {
    size: {
      type: [String, Number],
      default: '1.3em'
    }
  },
  computed: {
    fontSize() {
      if (this.size === null || this.size === undefined || this.size === '') return '1.5em'
      return typeof this.size === 'number' ? `${this.size}px` : String(this.size)
    }
  }
}
</script>

<style scoped>
/*
  使用系统 ✨ emoji（U+2728）作为 AI 生成入口的图标。
  emoji 颜色由系统字体决定（macOS/iOS 彩色、Windows 彩色、Linux 视字体而定），
  不再受按钮主色 currentColor 影响——这是符合行业惯例的视觉选择
  （Apple Intelligence、Notion AI 等都用此 emoji 作为 AI 入口）。
  line-height: 1 防止 emoji 把按钮撑高；display: inline-flex + align-items: center 保证与按钮文本基线对齐。
*/
.ai-sparkle-icon {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  line-height: 1;
  vertical-align: -0.05em;
  font-family:
    "Apple Color Emoji",
    "Segoe UI Emoji",
    "Noto Color Emoji",
    "Twemoji Mozilla",
    "EmojiOne Color",
    "Android Emoji",
    sans-serif;
  /* emoji 默认会跟随父级 color；强制 initial 让彩色 emoji 字形不被按钮主色覆盖 */
  color: initial;
  font-style: normal;
  font-weight: normal;
  user-select: none;
}
</style>
