<template>
  <div>
    <div style="display:flex; justify-content:space-between; align-items:center; margin-bottom:16px">
      <h2 style="color:#F7931A">Price Alerts</h2>
      <n-button @click="load">🔄 Refresh</n-button>
    </div>

    <n-empty v-if="!loading && alerts.length === 0" description="No alerts set. Add alerts from the Watchlist screen." style="margin-top:60px" />

    <n-data-table v-else :columns="columns" :data="alerts" :loading="loading" striped />
  </div>
</template>

<script setup lang="ts">
import { ref, onMounted, h } from 'vue'
import { NDataTable, NButton, NEmpty, NTag, useMessage } from 'naive-ui'
import type { DataTableColumns } from 'naive-ui'

declare const window: any
const message = useMessage()

interface PriceAlert {
  id: number; coinId: string; symbol: string
  highPrice: number; lowPrice: number; enabled: boolean; updatedAt: string
}

const alerts = ref<PriceAlert[]>([])
const loading = ref(false)

const columns: DataTableColumns<PriceAlert> = [
  { title: 'Coin', key: 'symbol', render: (row) => h('span', { style: 'font-weight:bold' }, row.symbol) },
  {
    title: 'High Alert',
    key: 'highPrice',
    render: (row) => row.highPrice > 0
      ? h(NTag, { type: 'error', size: 'small' }, { default: () => '🔴 $' + row.highPrice.toLocaleString() })
      : h('span', { style: 'color:#555' }, '—')
  },
  {
    title: 'Low Alert',
    key: 'lowPrice',
    render: (row) => row.lowPrice > 0
      ? h(NTag, { type: 'success', size: 'small' }, { default: () => '🟢 $' + row.lowPrice.toLocaleString() })
      : h('span', { style: 'color:#555' }, '—')
  },
  {
    title: 'Status',
    key: 'enabled',
    render: (row) => h(NTag, { type: row.enabled ? 'info' : 'default', size: 'small' }, { default: () => row.enabled ? 'Active' : 'Paused' })
  },
  {
    title: 'Actions',
    key: 'actions',
    render: (row) => h('div', { style: 'display:flex;gap:8px' }, [
      h(NButton, { size: 'small', type: 'error', onClick: () => remove(row.id) }, { default: () => '🗑 Delete' }),
    ])
  }
]

async function load() {
  loading.value = true
  try { alerts.value = await window.go.main.App.GetAlerts() }
  catch (e) { console.error(e) }
  finally { loading.value = false }
}

async function remove(id: number) {
  await window.go.main.App.DeleteAlert(id)
  message.success('Alert deleted')
  load()
}

onMounted(load)
</script>
