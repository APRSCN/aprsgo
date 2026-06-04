<script setup lang="ts">
import type { Series } from '~/types/api'

const props = defineProps<{
  title: string
  data: Series
  // valueFormatter formats the y value for tooltip (e.g. bytes).
  valueFormatter?: (v: number) => string
}>()

const el = ref<HTMLElement | null>(null)
let chart: any = null
let echarts: any = null

function buildOption() {
  const seriesData = (props.data || [])
    .map((d) => [new Date(d[0] * 1000), d[1]] as [Date, number])
    .sort((a, b) => a[0].getTime() - b[0].getTime())

  const fmt = props.valueFormatter
  return {
    tooltip: {
      trigger: 'axis',
      formatter: fmt
        ? (params: any) => {
            const p = params[0]
            return `${p.axisValueLabel}<br/>${p.marker}${props.title}: ${fmt(p.value[1])}`
          }
        : undefined,
    },
    grid: { left: '3%', right: '4%', bottom: '3%', containLabel: true },
    xAxis: { type: 'time', boundaryGap: false },
    yAxis: { type: 'value', boundaryGap: [0, '100%'] },
    series: [
      {
        name: props.title,
        type: 'line',
        smooth: true,
        symbol: 'none',
        areaStyle: {
          color: {
            type: 'linear',
            x: 0, y: 0, x2: 0, y2: 1,
            colorStops: [
              { offset: 0, color: 'rgba(64, 158, 255, 0.3)' },
              { offset: 1, color: 'rgba(64, 158, 255, 0.05)' },
            ],
          },
        },
        lineStyle: { color: '#409EFF' },
        data: seriesData,
      },
    ],
  }
}

async function ensureChart() {
  // Runs only on the client (called from onMounted / watchers).
  if (typeof window === 'undefined') return
  if (!el.value) {
    // The container may not be laid out yet on the very first tick; retry.
    await nextTick()
    if (!el.value) return
  }
  if (!echarts) {
    echarts = await import('echarts')
  }
  if (!chart) {
    chart = echarts.init(el.value)
    window.addEventListener('resize', onResize)
  }
  chart.setOption(buildOption())
}

function onResize() {
  chart?.resize()
}

onMounted(() => {
  ensureChart()
})

// Re-render (and lazily initialise) whenever the data changes — this also
// covers the case where data arrives after the component has mounted.
watch(
  () => props.data,
  () => {
    ensureChart()
  },
  { deep: true },
)

onUnmounted(() => {
  window.removeEventListener('resize', onResize)
  chart?.dispose()
  chart = null
})
</script>

<template>
  <!-- Rendered always; ECharts itself is imported client-side only. -->
  <div ref="el" class="h-64 w-full"></div>
</template>
