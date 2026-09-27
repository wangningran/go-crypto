<template>
  <div>
    <div style="display:flex; justify-content:space-between; align-items:center; margin-bottom:16px">
      <h2 style="color:#F7931A">Price Alerts</h2>
      <n-button @click="load">🔄 Refresh</n-button>
    </div>
    <p style="color:#888;font-size:13px;margin-bottom:16px">
      An alert fires once when the price crosses a target, then pauses. Resume it to arm it again.
    </p>

    <n-empty v-if="!loading && alerts.length === 0" description="No alerts set. Add alerts from the Watchlist screen." style="margin-top:60px" />

    <n-data-table v-else :columns="columns" :data="alerts" :loading="loading" striped />
  </div>
</template>

<script setup lang="ts">
import { ref, onMounted, onUnmounted, h } from 'vue'
import { NDataTable, NButton, NEmpty, NTag, useMessage } from 'naive-ui'
import type { DataTableColumns } from 'naive-ui'
import { GetAlerts, DeleteAlert, SetAlertEnabled } from '../../wailsjs/go/main/App'
import type { models } from '../../wailsjs/go/models'
import { EventsOn } from '../../wailsjs/runtime/runtime'
import { formatPrice, formatTime, errorText } from '../utils/format'
import { appState } from '../state'

const message = useMessage()
const alerts = ref<models.PriceAlert[]>([])
const loading = ref(false)

function status(row: models.PriceAlert) {
  if (row.enabled) return h(NTag, { type: 'info', size: 'small' }, { default: () => 'Active' })
  if (row.triggeredAt) return h(NTag, { type: 'warning', size: 'small' }, { default: () => 'Triggered ' + formatTime(row.triggeredAt) })
  return h(NTag, { size: 'small' }, { default: () => 'Paused' })
}

const columns: DataTableColumns<models.PriceAlert> = [
  { title: 'Coin', key: 'symbol', render: (row) => h('span', { style: 'font-weight:bold' }, row.symbol) },
  {
    title: 'High', key: 'highPrice',
    render: (row) => row.highPrice > 0 ? h(NTag, { type: 'error', size: 'small' }, { default: () => '▲ ' + formatPrice(row.highPrice, row.currency) }) : h('span', { style: 'color:#555' }, '—'),
  },
  {
    title: 'Low', key: 'lowPrice',
    render: (row) => row.lowPrice > 0 ? h(NTag, { type: 'success', size: 'small' }, { default: () => '▼ ' + formatPrice(row.lowPrice, row.currency) }) : h('span', { style: 'color:#555' }, '—'),
  },
  {
    title: 'Currency', key: 'currency',
    render: (row) => row.currency === appState.currency
      ? row.currency.toUpperCase()
      : h('span', { style: 'color:#e67e22', title: 'Only checked while this currency is selected in Settings' }, row.currency.toUpperCase() + ' (inactive)'),
  },
  { title: 'Status', key: 'enabled', render: status },
  {
    title: 'Actions', key: 'actions',
    render: (row) => h('div', { style: 'display:flex;gap:8px' }, [
      h(NButton, { size: 'small', onClick: () => toggle(row) }, { default: () => (row.enabled ? 'Pause' : 'Resume') }),
      h(NButton, { size: 'small', type: 'error', onClick: () => remove(row.id) }, { default: () => 'Delete' }),
    ]),
  },
]

async function load() {
  loading.value = true
  try { alerts.value = await GetAlerts() }
  catch (e) { message.error(errorText(e)) }
  finally { loading.value = false }
}

async function toggle(row: models.PriceAlert) {
  await SetAlertEnabled(row.id, !row.enabled)
  load()
}

async function remove(id: number) {
  await DeleteAlert(id)
  message.success('Alert deleted')
  load()
}

let off: (() => void) | undefined
onMounted(() => {
  load()
  off = EventsOn('price-alert', load) // reflect "Triggered" status immediately
})
onUnmounted(() => off?.())
</script>
