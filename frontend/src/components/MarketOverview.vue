<template>
  <div>
    <div style="display:flex;justify-content:space-between;align-items:center;margin-bottom:16px">
      <h2 style="color:#F7931A">Market Overview</h2>
      <n-button @click="load">🔄 Refresh</n-button>
    </div>

    <n-grid :cols="4" :x-gap="16" :y-gap="16" style="margin-bottom:24px" v-if="overview">
      <n-grid-item>
        <n-statistic label="Total Market Cap" :value="formatLarge(overview.totalMarketCap)" />
      </n-grid-item>
      <n-grid-item>
        <n-statistic label="24h Volume" :value="formatLarge(overview.totalVolume24h)" />
      </n-grid-item>
      <n-grid-item>
        <n-statistic label="BTC Dominance" :value="(overview.btcDominance?.toFixed(1) ?? '0') + '%'" />
      </n-grid-item>
      <n-grid-item>
        <n-statistic label="24h Change" :value="(overview.marketCapChange24h >= 0 ? '+' : '') + overview.marketCapChange24h?.toFixed(2) + '%'"
          :value-style="{ color: overview.marketCapChange24h >= 0 ? '#2ecc71' : '#e74c3c' }" />
      </n-grid-item>
    </n-grid>

    <h3 style="margin-bottom:12px;color:#aaa">Top 50 by Market Cap</h3>
    <n-data-table :columns="columns" :data="topCoins" :loading="loading" striped :max-height="500" />
  </div>
</template>

<script setup lang="ts">
import { ref, onMounted, h } from 'vue'
import { NGrid, NGridItem, NStatistic, NDataTable, NButton } from 'naive-ui'
import type { DataTableColumns } from 'naive-ui'
declare const window: any

interface CachedPrice { coinId: string; symbol: string; name: string; price: number; change24h: number; volume24h: number; marketCap: number }
interface Overview { totalMarketCap: number; totalVolume24h: number; btcDominance: number; ethDominance: number; marketCapChange24h: number }

const topCoins = ref<CachedPrice[]>([])
const overview = ref<Overview | null>(null)
const loading = ref(false)

const columns: DataTableColumns<CachedPrice> = [
  { title: '#', key: 'rank', width: 50, render: (_row, i) => String(i + 1) },
  { title: 'Name', key: 'name', render: (row) => h('span', [row.name, ' ', h('small', { style: 'color:#888' }, row.symbol)]) },
  { title: 'Price', key: 'price', render: (row) => '$' + (row.price < 1 ? row.price.toFixed(6) : row.price.toLocaleString('en', { minimumFractionDigits: 2, maximumFractionDigits: 2 })) },
  { title: '24h %', key: 'change24h', render: (row) => h('span', { style: `color:${row.change24h >= 0 ? '#2ecc71' : '#e74c3c'}` }, (row.change24h >= 0 ? '+' : '') + row.change24h?.toFixed(2) + '%') },
  { title: 'Market Cap', key: 'marketCap', render: (row) => formatLarge(row.marketCap) },
  { title: 'Volume 24h', key: 'volume24h', render: (row) => formatLarge(row.volume24h) },
]

function formatLarge(n: number): string {
  if (!n) return '$0'
  if (n >= 1e12) return '$' + (n / 1e12).toFixed(2) + 'T'
  if (n >= 1e9) return '$' + (n / 1e9).toFixed(2) + 'B'
  if (n >= 1e6) return '$' + (n / 1e6).toFixed(2) + 'M'
  return '$' + n.toFixed(2)
}

async function load() {
  loading.value = true
  try {
    overview.value = await window.go.main.App.GetMarketOverview()
    topCoins.value = await window.go.main.App.GetTopCoins(50)
  } catch(e) { console.error(e) }
  finally { loading.value = false }
}

onMounted(load)
</script>
