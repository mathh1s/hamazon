import { reactive } from 'vue'

export interface TooltipLine {
  text: string
  color?: string
  icon?: string
}

export const tooltip = reactive<{ visible: boolean; x: number; y: number; title: string; lines: TooltipLine[] }>({
  visible: false,
  x: 0,
  y: 0,
  title: '',
  lines: [],
})

export function showTooltip(e: MouseEvent, title: string, lines: TooltipLine[] = []) {
  tooltip.visible = true
  tooltip.title = title
  tooltip.lines = lines
  moveTooltip(e)
}

export function moveTooltip(e: MouseEvent) {
  tooltip.x = e.clientX
  tooltip.y = e.clientY
}

export function hideTooltip() {
  tooltip.visible = false
}
