/**
 * 步骤树操作 Composable
 * 从 ScenarioEditor.vue 中提取的步骤树管理逻辑
 */
import { ref, computed } from 'vue'
import type { UUID } from '@/types/common'
import type { ScenarioStep, StepType } from '@/types/scenario'

export interface TreeNode {
  id: string
  label: string
  stepType: StepType
  step: ScenarioStep
  children?: TreeNode[]
  enabled: boolean
}

export function useStepTree(scenarioId: () => UUID) {
  const steps = ref<ScenarioStep[]>([])
  const loading = ref(false)

  /**
   * 将扁平步骤列表构建为树形结构
   * for/condition 步骤的子步骤通过 config 中的 bodyStepSeqs/thenStepSeqs 等关联
   */
  const treeData = computed<TreeNode[]>(() => {
    return buildTree(steps.value)
  })

  function buildTree(flatSteps: ScenarioStep[]): TreeNode[] {
    // 按 stepOrder 排序
    const sorted = [...flatSteps].sort((a, b) => a.stepOrder - b.stepOrder)

    // 建立 stepSeq -> step 映射
    const seqMap = new Map<number, ScenarioStep>()
    for (const step of sorted) {
      seqMap.set(step.stepSeq, step)
    }

    // 找出被控制流引用的子步骤
    const controlledSeqs = new Set<number>()
    for (const step of sorted) {
      if (step.stepType === 'for') {
        const config = step.config as Record<string, unknown>
        const bodySeqs = config?.bodyStepSeqs as number[] || []
        bodySeqs.forEach(s => controlledSeqs.add(s))
      } else if (step.stepType === 'condition') {
        const config = step.config as Record<string, unknown>
        const branches = config?.branches as Array<{ stepSeqs: number[] }> || []
        branches.forEach(b => b.stepSeqs?.forEach(s => controlledSeqs.add(s)))
        const elseSeqs = config?.elseStepSeqs as number[] || []
        elseSeqs.forEach(s => controlledSeqs.add(s))
        const thenSeqs = config?.thenStepSeqs as number[] || []
        thenSeqs.forEach(s => controlledSeqs.add(s))
      }
    }

    // 构建顶层节点（非受控步骤）
    const topLevel = sorted.filter(s => !controlledSeqs.has(s.stepSeq))

    return topLevel.map(step => {
      const node: TreeNode = {
        id: step.id,
        label: step.name || `步骤 ${step.stepOrder}`,
        stepType: step.stepType,
        step,
        enabled: step.enabled,
      }

      // 为控制流步骤附加子节点
      if (step.stepType === 'for') {
        const config = step.config as Record<string, unknown>
        const bodySeqs = config?.bodyStepSeqs as number[] || []
        node.children = bodySeqs
          .map(seq => seqMap.get(seq))
          .filter((s): s is ScenarioStep => !!s)
          .map(s => ({
            id: s.id,
            label: s.name || `步骤 ${s.stepOrder}`,
            stepType: s.stepType,
            step: s,
            enabled: s.enabled,
          }))
      } else if (step.stepType === 'condition') {
        const config = step.config as Record<string, unknown>
        const branches = config?.branches as Array<{ stepSeqs: number[] }> || []
        node.children = []
        // then 分支
        const thenSeqs = config?.thenStepSeqs as number[] || branches[0]?.stepSeqs || []
        for (const seq of thenSeqs) {
          const s = seqMap.get(seq)
          if (s) node.children.push({
            id: s.id, label: s.name || `步骤 ${s.stepOrder}`,
            stepType: s.stepType, step: s, enabled: s.enabled,
          })
        }
        // else 分支
        const elseSeqs = config?.elseStepSeqs as number[] || []
        for (const seq of elseSeqs) {
          const s = seqMap.get(seq)
          if (s) node.children.push({
            id: s.id, label: s.name || `步骤 ${s.stepOrder}`,
            stepType: s.stepType, step: s, enabled: s.enabled,
          })
        }
      }

      return node
    })
  }

  /**
   * 拖拽排序后的处理
   */
  function getStepIdsInOrder(): string[] {
    const ids: string[] = []
    function walk(nodes: TreeNode[]) {
      for (const node of nodes) {
        ids.push(node.id)
        if (node.children) walk(node.children)
      }
    }
    walk(treeData.value)
    return ids
  }

  return {
    steps,
    loading,
    treeData,
    buildTree,
    getStepIdsInOrder,
  }
}
