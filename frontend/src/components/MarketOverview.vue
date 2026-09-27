<template>
  <div>
    <div style="display:flex;justify-content:space-between;align-items:center;margin-bottom:16px">
      <h2 style="color:#F7931A">Market Overview</h2>
      <n-button @click="load">🔄 Refresh</n-button>
    </div>

    <n-grid :cols="4" :x-gap="16" :y-gap="16" style="margin-bottom:24px" v-if="overview">
      <n-grid-item>
        <n-statistic label="Total Market Cap" :value="formatLarge(overview.totalMarketCap, overview.currency)" />
      </n-grid-item>
      <n-grid-item>
        <n-statistic label="24h Volume" :value="formatLarge(overview.totalVolume24h, overview.currency)" />
      </n-grid-item>
      <n-grid-item>
        <n-statistic label="BTC Dominance" :value="(overview.btcDominance?.toFixed(1) ?? '0') + '%'" />
      </n-grid-item>
      <n-grid-item>
        <n-statistic label="24h Change" :value="formatPct(overview.marketCapChange24h)"
          :value-style="{ color: changeColor(overview.marketCapChange24h) }" />
      </n-grid-item>
    </n-grid>

    <n-alert v-if="error" type="error" style="margin-bottom:16px">{{ error }}</n-alert>

    <h3 style="margin-bottom:12px;color:#aaa">Top 50 by Market Cap</h3>
    <n-data-table :columns="columns" :data="topCoins" :loading="loading" striped :max-height="500" />
  </div>
</template>

<script setup lang="ts">
import { ref, onMounted, h } from 'vue'
import { NGrid, NGridItem, NStatistic, NDataTable, NButton, NAlert } from 'naive-ui'
import type { DataTableColumns } from 'naive-ui'
import { GetMarketOverview, GetTopCoins } from '../../wailsjs/go/main/App'
import type { models } from '../../wailsjs/go/models'
import { formatPrice, formatLarge, formatPct, changeColor, errorText } from '../utils/format'

const topCoins = ref<models.CachedPrice[]>([])
const overview = ref<models.MarketOverview | null>(null)
const loading = ref(false)
const error = ref('')

const columns: DataTableColumns<models.CachedPrice> = [
  { title: '#', key: 'rank', width: 50, render: (_row, i) => String(i + 1) },
  { title: 'Name', key: 'name', render: (row) => h('span', [row.name, ' ', h('small', { style: 'color:#888' }, row.symbol)]) },
  { title: 'Price', key: 'price', render: (row) => formatPrice(row.price, row.currency) },
  { title: '24h %', key: 'change24h', render: (row) => h('span', { style: `color:${changeColor(row.change24h)}` }, formatPct(row.change24h)) },
  { title: 'Market Cap', key: 'marketCap', render: (row) => formatLarge(row.marketCap, row.currency) },
  { title: 'Volume 24h', key: 'volume24h', render: (row) => formatLarge(row.volume24h, row.currency) },
]

async function load() {
  loading.value = true
  error.value = ''
  try {
    // Sequential on purpose: the free CoinGecko API rate-limits bursts.
    overview.value = await GetMarketOverview()
    topCoins.value = await GetTopCoins(50)
  } catch (e) {
    error.value = errorText(e)
  } finally {
    loading.value = false
  }
}

onMounted(load)
</script>
