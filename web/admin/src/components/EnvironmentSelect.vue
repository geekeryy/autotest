<template>
  <el-select
    :model-value="modelValue"
    :placeholder="placeholder"
    :disabled="disabled"
    :filterable="filterable"
    :class="selectClass"
    popper-class="environment-select-dropdown"
    @update:model-value="$emit('update:modelValue', $event)"
    @change="$emit('change', $event)"
  >
    <el-option v-for="env in environments" :key="env.id" :label="env.name" :value="env.id">
      <div class="env-option-row">
        <span class="env-option-label">{{ env.name }}</span>
        <el-tooltip content="编辑环境" placement="right">
          <el-button
            class="env-option-edit-btn"
            type="primary"
            link
            @mousedown.stop
            @click.stop="$emit('edit', env)"
          >
            <el-icon>
              <EditPen />
            </el-icon>
          </el-button>
        </el-tooltip>
      </div>
    </el-option>
  </el-select>
</template>

<script>
import { EditPen } from '@element-plus/icons-vue'

export default {
  name: 'EnvironmentSelect',
  components: { EditPen },
  props: {
    modelValue: {
      type: [String, Number],
      default: ''
    },
    environments: {
      type: Array,
      default: () => []
    },
    placeholder: {
      type: String,
      default: '选择运行环境'
    },
    disabled: {
      type: Boolean,
      default: false
    },
    filterable: {
      type: Boolean,
      default: true
    },
    selectClass: {
      type: [String, Array, Object],
      default: ''
    }
  },
  emits: ['update:modelValue', 'change', 'edit']
}
</script>
