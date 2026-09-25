<template>
  <div>
    <div style="display:flex; justify-content:space-between; align-items:center; margin-bottom:16px">
      <h2 style="color:#F7931A">My Watchlist</h2>
      <n-button type="primary" @click="showSearch = true">+ Add Coin</n-button>
    </div>

    <n-data-table :columns="columns" :data="prices" :loading="loading" striped />

    <!-- Add coin modal -->
    <n-modal v-model:show="showSearch" title="Search Coins" preset="card" style="width:500px">
      <n-input v-model:value="searchQuery" placeholder="Search by name or symbol..." clearable @input="onSearch" />
      <n-list style="margin-top:12px">
        <n-list-item v-for="coin in searchResults" :key="coin.id" @click="addCoin(coin)" style="cursor:pointer">
          <n-thing :title="coin.name" :description="coin.symbol" />
        </n-list-item>
      </n-list>
    </n-modal>

    <!-- AI Analysis drawer -->
    <n-drawer v-model:show="showAnalysis" width="480" placement="right">
      <n-drawer-content :title="'AI Analysis: ' + selectedCoin">
        <div v-if="analysisLoading" style="text-align:center; padding:40px">
          <n-spin size="large" />
        </div>
        <div v-else style="white-space:pre-wrap; line-height:1.6">{{ analysisText }}</div>
      </n-drawer-content>
    </n-drawer>

    <!-- Alert modal -->
    <n-modal v-model:show="showAlert" title="Set Price Alert" preset="card" style="width:400px">
      <n-form>
        <n-form-item label="High Price Alert ($)">
          <n-input-number v-model:value="alertHigh" :min="0" placeholder="0 = disabled" style="width:100%" />
        </n-form-item>
        <n-form-item label="Low Price Alert ($)">
          <n-input-number v-model:value="alertLow" :min="0" placeholder="0 = disabled" style="width:100%" />
        </n-form-item>
        <n-button type="primary" @click="saveAlert" style="width:100%">Save Alert</n-button>
      </n-form>
    </n-modal>
  </div>
</template>

<script setup lang="ts">
import { ref, onMounted, h } from 'vue'
import { NButton, NDataTable, NModal, NInput, NList, NListItem, NThing,
         NDrawer, NDrawerContent, NSpin, NForm, NFormItem, NInputNumber, useMessage } from 'naive-ui'
import type { DataTableColumns } from 'naive-ui'

declare const window: any
const message = useMessage()

interface CachedPrice {
  coinId: string; symbol: string; name: string; price: number
  change24h: number; volume24h: number; marketCap: number
}
interface CoinResult { id: string; symbol: string; name: string; thumb: string }

const prices = ref<CachedPrice[]>([])
const loading = ref(false)
const showSearch = ref(false)
const searchQuery = ref('')
const searchResults = ref<CoinResult[]>([])
const showAnalysis = ref(false)
const analysisText = ref('')
const analysisLoading = ref(false)
const selectedCoin = ref('')
const showAlert = ref(false)
const alertCoinId = ref('')
const alertSymbol = ref('')
const alertHigh = ref(0)
const alertLow = ref(0)

const columns: DataTableColumns<CachedPrice> = [
  { title: 'Coin', key: 'name', render: (row) => h('span', { style: 'font-weight:bold' }, [row.name, ' ', h('small', { style: 'color:#888' }, row.symbol)]) },
  { title: 'Price', key: 'price', render: (row) => '$' + (row.price < 1 ? row.price.toFixed(6) : row.price.toLocaleString('en', { minimumFractionDigits: 2, maximumFractionDigits: 2 })) },
  { title: '24h %', key: 'change24h', render: (row) => h('span', { style: `color:${row.change24h >= 0 ? '#2ecc71' : '#e74c3c'}` }, (row.change24h >= 0 ? '+' : '') + row.change24h?.toFixed(2) + '%') },
  { title: 'Volume', key: 'volume24h', render: (row) => formatLarge(row.volume24h) },
  { title: 'Market Cap', key: 'marketCap', render: (row) => formatLarge(row.marketCap) },
  {
    title: 'Actions', key: 'actions', render: (row) => h('div', { style: 'display:flex;gap:8px' }, [
      h(NButton, { size: 'small', onClick: () => analyze(row.coinId, row.name) }, { default: () => '🤖 AI' }),
      h(NButton, { size: 'small', onClick: () => openAlert(row) }, { default: () => '🔔 Alert' }),
      h(NButton, { size: 'small', type: 'error', onClick: () => remove(row.coinId) }, { default: () => '✕' }),
    ])
  }
]

function formatLarge(n: number): string {
  if (!n) return '$0'
  if (n >= 1e12) return '$' + (n / 1e12).toFixed(2) + 'T'
  if (n >= 1e9) return '$' + (n / 1e9).toFixed(2) + 'B'
  if (n >= 1e6) return '$' + (n / 1e6).toFixed(2) + 'M'
  return '$' + n.toFixed(2)
}

async function loadPrices() {
  loading.value = true
  try { prices.value = await window.go.main.App.GetWatchlistPrices() }
  catch (e) { console.error(e) }
  finally { loading.value = false }
}

let searchTimer: any
async function onSearch() {
  clearTimeout(searchTimer)
  if (!searchQuery.value) { searchResults.value = []; return }
  searchTimer = setTimeout(async () => {
    searchResults.value = await window.go.main.App.SearchCoins(searchQuery.value)
  }, 400)
}

async function addCoin(coin: CoinResult) {
  await window.go.main.App.AddToWatchlist(coin.id, coin.symbol, coin.name)
  showSearch.value = false
  searchQuery.value = ''
  searchResults.value = []
  message.success('Added ' + coin.name + ' to watchlist')
  loadPrices()
}

async function remove(coinId: string) {
  await window.go.main.App.RemoveFromWatchlist(coinId)
  loadPrices()
}

async function analyze(coinId: string, name: string) {
  selectedCoin.value = name
  showAnalysis.value = true
  analysisLoading.value = true
  analysisText.value = ''
  try { analysisText.value = await window.go.main.App.AnalyzeCoin(coinId) }
  catch (e) { analysisText.value = 'Error: ' + e }
  finally { analysisLoading.value = false }
}

function openAlert(row: CachedPrice) {
  alertCoinId.value = row.coinId
  alertSymbol.value = row.symbol
  alertHigh.value = 0
  alertLow.value = 0
  showAlert.value = true
}

async function saveAlert() {
  await window.go.main.App.SetAlert(alertCoinId.value, alertSymbol.value, alertHigh.value, alertLow.value)
  showAlert.value = false
  message.success('Alert saved')
}

onMounted(() => {
  loadPrices()
  setInterval(loadPrices, 30000)
  window.runtime?.EventsOn('prices-updated', loadPrices)
  window.runtime?.EventsOn('price-alert', (data: any) => {
    message.warning(data.message)
  })
})
</script>
