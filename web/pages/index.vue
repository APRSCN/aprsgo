<script setup lang="ts">
import type { Status, Stats, Series } from '~/types/api'

const api = useApi()
const { t, locale, locales, setLocale } = useI18n()

const status = ref<Status | null>(null)
const stats = ref<Stats | null>(null)
const error = ref<string>('')
const loading = ref(true)
let statusTimer: ReturnType<typeof setInterval> | null = null
let statsTimer: ReturnType<typeof setInterval> | null = null

async function refreshStatus() {
  try {
    status.value = await api.getStatus()
    error.value = ''
  } catch (e: any) {
    error.value = e?.message || 'failed to load status'
  } finally {
    loading.value = false
  }
}

async function refreshStats() {
  try {
    stats.value = await api.getStats()
  } catch {
    /* charts are best-effort */
  }
}

onMounted(() => {
  refreshStatus()
  refreshStats()
  statusTimer = setInterval(refreshStatus, 3000)
  statsTimer = setInterval(refreshStats, 60000)
})

onUnmounted(() => {
  if (statusTimer) clearInterval(statusTimer)
  if (statsTimer) clearInterval(statsTimer)
})

const server = computed(() => status.value?.server)
const uplink = computed(() => status.value?.uplink)
const peers = computed(() => status.value?.peers ?? [])
const listeners = computed(() => status.value?.listeners ?? [])
const clients = computed(() => status.value?.clients ?? [])
const totals = computed(() => status.value?.totals)
const totalClients = computed(() =>
  listeners.value.reduce((s, l) => s + (l.online_client || 0), 0),
)

const memoryRows = computed(() => {
  const m = server.value?.memory
  if (!m) return []
  return [
    { k: t('memory.systemTotal'), v: formatMB(m.total) },
    { k: t('memory.systemUsed'), v: formatMB(m.used) },
    { k: t('memory.process'), v: formatMB(m.self) },
    { k: t('memory.heap'), v: formatMB(m.heap) },
    { k: t('memory.currentAlloc'), v: formatMB(m.current_allocated) },
    { k: t('memory.totalAlloc'), v: formatMB(m.total_allocated) },
    { k: t('memory.nextGC'), v: formatMB(m.next_gc) },
    { k: t('memory.gcCount'), v: formatNumber(m.num_gc) },
    { k: t('memory.mallocs'), v: formatNumber(m.malloc) },
    { k: t('memory.frees'), v: formatNumber(m.free) },
    { k: t('memory.gcPause'), v: `${(m.pause_total_sec || 0).toFixed(4)} s` },
  ]
})

type GraphKey = 'memory' | 'uplink_packet_rx' | 'uplink_packet_tx' | 'uplink_bytes_rx' | 'uplink_bytes_tx'
const graphKey = ref<GraphKey>('uplink_packet_rx')
const graphOptions = computed(() => [
  { value: 'uplink_packet_rx' as GraphKey, label: t('stats.uplinkPacketRX') },
  { value: 'uplink_packet_tx' as GraphKey, label: t('stats.uplinkPacketTX') },
  { value: 'uplink_bytes_rx' as GraphKey, label: t('stats.uplinkBytesRX') },
  { value: 'uplink_bytes_tx' as GraphKey, label: t('stats.uplinkBytesTX') },
  { value: 'memory' as GraphKey, label: t('stats.memory') },
])
const graphData = computed<Series>(() => (stats.value ? stats.value[graphKey.value] : []) as Series)
const graphFormatter = computed(() => {
  if (graphKey.value.includes('bytes')) return (v: number) => formatBytes(v)
  if (graphKey.value === 'memory') return (v: number) => formatMB(v)
  return (v: number) => formatNumber(v)
})
const graphTitle = computed(() => graphOptions.value.find((o) => o.value === graphKey.value)?.label ?? '')

function onLocaleChange(code: string) {
  setLocale(code as any)
}
</script>

<template>
  <div class="mx-auto max-w-[1600px] px-4 py-6">
    <!-- Header -->
    <header class="mb-6 flex items-center justify-between">
      <div class="flex items-center gap-3">
        <img src="/logo.svg" alt="logo" class="h-10 w-10" @error="(e: any) => (e.target.style.display = 'none')" />
        <div>
          <h1 class="text-2xl font-semibold">{{ t('app.title') }}</h1>
          <p class="text-sm text-gray-500">{{ server?.software }} {{ server?.codename }}</p>
        </div>
      </div>
      <div class="flex items-center gap-2">
        <el-select :model-value="locale" size="small" style="width: 120px" @update:model-value="onLocaleChange">
          <el-option v-for="l in (locales as any[])" :key="l.code" :label="l.name" :value="l.code" />
        </el-select>
        <el-tag v-if="uplink?.up" type="success" effect="dark">{{ t('header.uplinkOnline') }}</el-tag>
        <el-tag v-else type="info" effect="dark">{{ t('header.noUplink') }}</el-tag>
        <el-tag v-if="peers.length" type="warning" effect="dark">{{ t('header.peers', { n: peers.length }) }}</el-tag>
      </div>
    </header>

    <el-alert v-if="error" :title="error" type="error" show-icon class="mb-4" />

    <div v-loading="loading">
      <!-- Summary cards -->
      <section v-if="server" class="mb-6 grid grid-cols-1 gap-4 sm:grid-cols-2 lg:grid-cols-4">
        <StatCard :label="t('card.serverId')" :value="server.id" />
        <StatCard :label="t('card.uptime')" :value="formatUptime(server.uptime)" />
        <StatCard :label="t('card.onlineClients')" :value="String(totalClients)" />
        <StatCard :label="t('card.cpu')" :value="`${server.percent.toFixed(1)} %`" />
      </section>

      <!-- Totals cards -->
      <section v-if="totals" class="mb-6 grid grid-cols-2 gap-4 sm:grid-cols-3 lg:grid-cols-6">
        <StatCard :label="t('totals.packetsRx')" :value="formatNumber(totals.packet_rx)" />
        <StatCard :label="t('totals.packetsTx')" :value="formatNumber(totals.packet_tx)" />
        <StatCard :label="t('totals.bytesRx')" :value="formatBytes(totals.bytes_rx)" />
        <StatCard :label="t('totals.bytesTx')" :value="formatBytes(totals.bytes_tx)" />
        <StatCard :label="t('totals.dupes')" :value="formatNumber(totals.dupes)" />
        <StatCard :label="t('totals.positionCache')" :value="formatNumber(totals.position_cache)" />
      </section>

      <!-- Server + Memory -->
      <div class="mb-6 grid grid-cols-1 gap-6 lg:grid-cols-2">
        <el-card v-if="server" shadow="never">
          <template #header><span class="font-medium">{{ t('server.title') }}</span></template>
          <el-descriptions :column="1" border size="small">
            <el-descriptions-item :label="t('server.version')">{{ server.software }} {{ server.version }} {{ server.codename }}</el-descriptions-item>
            <el-descriptions-item :label="t('server.admin')">{{ server.admin }}</el-descriptions-item>
            <el-descriptions-item :label="t('server.email')">{{ server.email }}</el-descriptions-item>
            <el-descriptions-item :label="t('server.osArch')">{{ server.os }} / {{ server.arch }}</el-descriptions-item>
            <el-descriptions-item :label="t('server.cpuModel')">{{ server.model }}</el-descriptions-item>
            <el-descriptions-item :label="t('server.time')">{{ formatTime(server.now) }}</el-descriptions-item>
          </el-descriptions>
        </el-card>

        <el-card v-if="server" shadow="never">
          <template #header><span class="font-medium">{{ t('memory.title') }}</span></template>
          <el-descriptions :column="2" border size="small">
            <el-descriptions-item v-for="row in memoryRows" :key="row.k" :label="row.k">{{ row.v }}</el-descriptions-item>
          </el-descriptions>
        </el-card>
      </div>

      <!-- Uplink -->
      <el-card class="mb-6" shadow="never">
        <template #header>
          <span class="font-medium">{{ t('uplink.title') }}</span>
          <span class="ml-2 text-xs text-gray-400">{{ t('uplink.subtitle') }}</span>
        </template>
        <div v-if="uplink">
          <el-descriptions :column="3" border size="small">
            <el-descriptions-item :label="t('uplink.serverId')">{{ uplink.server_id || '-' }}</el-descriptions-item>
            <el-descriptions-item :label="t('uplink.software')">{{ uplink.server || '-' }}</el-descriptions-item>
            <el-descriptions-item :label="t('uplink.configuredHost')">{{ uplink.host }}:{{ uplink.port }}</el-descriptions-item>
            <el-descriptions-item :label="t('uplink.realAddr')">
              <a v-if="uplink.real_addr" :href="serverStatusLink(uplink.real_addr)" target="_blank" class="text-blue-600 hover:underline">{{ uplink.real_addr }}</a>
              <span v-else>-</span>
            </el-descriptions-item>
            <el-descriptions-item :label="t('uplink.protocolMode')">{{ uplink.protocol }} / {{ uplink.mode }}</el-descriptions-item>
            <el-descriptions-item :label="t('uplink.connectedSince')">{{ formatTime(uplink.uptime) }}</el-descriptions-item>
            <el-descriptions-item :label="t('uplink.lastRx')">{{ timeAgo(uplink.last) }}</el-descriptions-item>
            <el-descriptions-item :label="t('uplink.packetsRxTx')">{{ formatNumber(uplink.packet_rx) }} / {{ formatNumber(uplink.packet_tx) }}</el-descriptions-item>
            <el-descriptions-item :label="t('uplink.rxDupErr')">{{ formatNumber(uplink.packet_rx_dup) }} / {{ formatNumber(uplink.packet_rx_err) }}</el-descriptions-item>
            <el-descriptions-item :label="t('uplink.bytesRate')">{{ formatRate(uplink.bytes_rx_rate) }} / {{ formatRate(uplink.bytes_tx_rate) }}</el-descriptions-item>
          </el-descriptions>
        </div>
        <el-empty v-else :description="t('uplink.none')" :image-size="60" />
      </el-card>

      <!-- Core peers -->
      <el-card v-if="peers.length" class="mb-6" shadow="never">
        <template #header>
          <span class="font-medium">{{ t('peers.title') }}</span>
          <span class="ml-2 text-xs text-gray-400">{{ t('peers.subtitle') }}</span>
        </template>
        <el-table :data="peers" size="small" stripe>
          <el-table-column prop="name" :label="t('peers.name')" min-width="160" />
          <el-table-column prop="id" :label="t('peers.serverId')" min-width="120" />
          <el-table-column :label="t('peers.address')" min-width="200">
            <template #default="{ row }">
              <a :href="serverStatusLink(row.addr)" target="_blank" class="text-blue-600 hover:underline">{{ row.addr }}</a>
            </template>
          </el-table-column>
        </el-table>
      </el-card>

      <!-- Stats charts -->
      <el-card class="mb-6" shadow="never">
        <template #header>
          <div class="flex items-center justify-between">
            <span class="font-medium">{{ t('stats.title') }}</span>
            <el-select v-model="graphKey" size="small" style="width: 220px">
              <el-option v-for="o in graphOptions" :key="o.value" :label="o.label" :value="o.value" />
            </el-select>
          </div>
        </template>
        <StatsChart :title="graphTitle" :data="graphData" :value-formatter="graphFormatter" />
      </el-card>

      <!-- Listeners -->
      <el-card class="mb-6" shadow="never">
        <template #header><span class="font-medium">{{ t('listeners.title') }}</span></template>
        <el-table :data="listeners" size="small" stripe>
          <el-table-column prop="name" :label="t('listeners.name')" min-width="150" />
          <el-table-column prop="mode" :label="t('listeners.mode')" width="90" />
          <el-table-column prop="protocol" :label="t('listeners.proto')" width="70" />
          <el-table-column :label="t('listeners.address')" min-width="160">
            <template #default="{ row }">{{ row.host }}:{{ row.port }}</template>
          </el-table-column>
          <el-table-column prop="filter" :label="t('listeners.filter')" min-width="120" />
          <el-table-column :label="t('listeners.clients')" width="100">
            <template #default="{ row }">{{ row.online_client }} / {{ row.peak_client }}</template>
          </el-table-column>
          <el-table-column :label="t('listeners.pktsTxRx')" width="140">
            <template #default="{ row }">{{ formatNumber(row.packet_tx) }} / {{ formatNumber(row.packet_rx) }}</template>
          </el-table-column>
          <el-table-column :label="t('listeners.bytesTxRx')" width="150">
            <template #default="{ row }">{{ formatBytes(row.bytes_tx) }} / {{ formatBytes(row.bytes_rx) }}</template>
          </el-table-column>
          <el-table-column :label="t('listeners.txRxRate')" width="150">
            <template #default="{ row }">{{ formatRate(row.bytes_tx_rate) }} / {{ formatRate(row.bytes_rx_rate) }}</template>
          </el-table-column>
        </el-table>
      </el-card>

      <!-- Clients -->
      <el-card shadow="never">
        <template #header><span class="font-medium">{{ t('clients.title', { n: clients.length }) }}</span></template>
        <el-table :data="clients" size="small" stripe max-height="600">
          <el-table-column :label="t('clients.port')" width="70" prop="port" />
          <el-table-column :label="t('clients.username')" min-width="120">
            <template #default="{ row }">
              <a v-if="isServerSoftware(row.software)" :href="serverStatusLink(row.addr)" target="_blank" class="text-blue-600 hover:underline">{{ row.id }}</a>
              <a v-else :href="callsignLink(row.id)" target="_blank" class="text-gray-700 hover:underline">{{ row.id }}</a>
            </template>
          </el-table-column>
          <el-table-column prop="addr" :label="t('clients.address')" min-width="160" />
          <el-table-column :label="t('clients.verified')" width="90">
            <template #default="{ row }">
              <el-tag :type="row.verified ? 'success' : 'info'" size="small">{{ row.verified ? t('clients.yes') : t('clients.no') }}</el-tag>
            </template>
          </el-table-column>
          <el-table-column :label="t('clients.up')" width="100">
            <template #default="{ row }">{{ timeAgo(row.uptime) }}</template>
          </el-table-column>
          <el-table-column :label="t('clients.lastIn')" width="100">
            <template #default="{ row }">{{ timeAgo(row.last) }}</template>
          </el-table-column>
          <el-table-column :label="t('clients.lastOut')" width="100">
            <template #default="{ row }">{{ timeAgo(row.last_tx) }}</template>
          </el-table-column>
          <el-table-column :label="t('clients.software')" min-width="150">
            <template #default="{ row }">
              <span :class="{ 'font-medium text-blue-700': isServerSoftware(row.software) }">{{ row.software }} {{ row.version }}</span>
            </template>
          </el-table-column>
          <el-table-column :label="t('clients.pktsTx')" width="90">
            <template #default="{ row }">{{ formatNumber(row.packet_tx) }}</template>
          </el-table-column>
          <el-table-column :label="t('clients.pktsRxDupErr')" width="150">
            <template #default="{ row }">{{ formatNumber(row.packet_rx) }} / {{ formatNumber(row.packet_rx_dup) }} / {{ formatNumber(row.packet_rx_err) }}</template>
          </el-table-column>
          <el-table-column :label="t('clients.bytesTx')" width="100">
            <template #default="{ row }">{{ formatBytes(row.bytes_tx) }}</template>
          </el-table-column>
          <el-table-column :label="t('clients.bytesRx')" width="100">
            <template #default="{ row }">{{ formatBytes(row.bytes_rx) }}</template>
          </el-table-column>
          <el-table-column :label="t('clients.txRxRate')" width="150">
            <template #default="{ row }">{{ formatRate(row.bytes_tx_rate) }} / {{ formatRate(row.bytes_rx_rate) }}</template>
          </el-table-column>
          <el-table-column :label="t('clients.outQ')" width="80" prop="out_q" />
          <el-table-column :label="t('clients.msgRcpts')" width="90" prop="msg_rcpts" />
          <el-table-column prop="filter" :label="t('clients.filter')" min-width="140" />
        </el-table>
      </el-card>
    </div>

    <footer class="mt-8 text-center text-xs text-gray-400">{{ t('footer.text') }}</footer>
  </div>
</template>
