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
    <n-drawer v-model:show="showAnalysis" :width="520" placement="right">
      <n-drawer-content :title="'AI Analysis: ' + selectedCoin" closable>
        <!-- Live agent steps -->
        <div v-if="analysisLoading || steps.length" style="margin-bottom:16px">
          <div style="color:#888;font-size:12px;margin-bottom:6px">Agent steps</div>
          <n-timeline size="medium">
            <n-timeline-item
              v-for="s in steps"
              :key="s.index"
              :type="s.error ? 'error' : 'success'"
              :title="toolLabel(s.tool)"
              :content="s.error ? 'Error: ' + s.error : s.durationMs + ' ms'"
            />
            <n-timeline-item v-if="analysisLoading" type="info" title="Thinking…">
              <n-spin size="small" />
            </n-timeline-item>
          </n-timeline>
        </div>

        <n-alert v-if="analysisError" type="error" :show-icon="true" style="margin-bottom:12px">
          {{ analysisError }}
        </n-alert>

        <div v-if="result">
          <n-space align="center" style="margin-bottom:12px">
            <n-tag :type="directionTag(result.direction)" size="large" round>
              {{ directionLabel(result.direction) }} · next 24h
            </n-tag>
            <span style="color:#aaa">Confidence {{ Math.round(result.confidence * 100) }}%</span>
            <n-tag v-if="result.cached" size="small">cached (&lt; 5 min old)</n-tag>
          </n-space>
          <n-grid :cols="3" :x-gap="12" style="margin-bottom:12px">
            <n-grid-item><n-statistic label="Price at analysis" :value="formatPrice(result.priceAtAnalysis, result.currency)" /></n-grid-item>
            <n-grid-item><n-statistic label="Support" :value="formatPrice(result.support, result.currency)" /></n-grid-item>
            <n-grid-item><n-statistic label="Resistance" :value="formatPrice(result.resistance, result.currency)" /></n-grid-item>
          </n-grid>
          <div style="white-space:pre-wrap; line-height:1.6">{{ result.summary }}</div>
          <div style="margin-top:16px;color:#777;font-size:12px">
            Model: {{ result.model }} · {{ result.promptTokens + result.completionTokens }} tokens.
            This call will be scored against the real price in 24h (see AI Track Record).
          </div>
          <n-alert type="warning" :show-icon="false" style="margin-top:12px;font-size:12px">
            For information only. Not financial advice.
          </n-alert>
        </div>
      </n-drawer-content>
    </n-drawer>

    <!-- Alert modal -->
    <n-modal v-model:show="showAlert" :title="'Price Alert: ' + alertSymbol" preset="card" style="width:400px">
      <div style="color:#888;margin-bottom:12px;font-size:13px">
        Prices in {{ appState.currency.toUpperCase() }}. Leave a field at 0 to disable it.
        The alert fires once, then pauses.
      </div>
      <n-form>
        <n-form-item :label="'High price (' + currencySymbol(appState.currency) + ')'">
          <n-input-number v-model:value="alertHigh" :min="0" style="width:100%" />
        </n-form-item>
        <n-form-item :label="'Low price (' + currencySymbol(appState.currency) + ')'">
          <n-input-number v-model:value="alertLow" :min="0" style="width:100%" />
        </n-form-item>
        <n-button type="primary" @click="saveAlert" style="width:100%">Save Alert</n-button>
      </n-form>
    </n-modal>
  </div>
</template>

<script setup lang="ts">
import { ref, onMounted, onUnmounted, h } from 'vue'
import {
  NButton, NDataTable, NModal, NInput, NList, NListItem, NThing, NDrawer, NDrawerContent, NSpin,
  NForm, NFormItem, NInputNumber, NTimeline, NTimelineItem, NAlert, NTag, NSpace, NGrid, NGridItem,
  NStatistic, useMessage,
} from 'naive-ui'
import type { DataTableColumns } from 'naive-ui'
import {
  GetWatchlistPrices, SearchCoins, AddToWatchlist, RemoveFromWatchlist, AnalyzeCoin, GetAlert, SetAlert,
} from '../../wailsjs/go/main/App'
import type { models } from '../../wailsjs/go/models'
import { EventsOn } from '../../wailsjs/runtime/runtime'
import { formatPrice, formatLarge, formatPct, changeColor, currencySymbol, errorText } from '../utils/format'
import { appState } from '../state'

const message = useMessage()

const prices = ref<models.CachedPrice[]>([])
const loading = ref(false)
const showSearch = ref(false)
const searchQuery = ref('')
const searchResults = ref<models.CoinSearchResult[]>([])

const showAnalysis = ref(false)
const analysisLoading = ref(false)
const analysisError = ref('')
const selectedCoin = ref('')
const selectedCoinId = ref('')
const steps = ref<models.AgentStep[]>([])
const result = ref<models.AnalysisRecord | null>(null)

const showAlert = ref(false)
const alertCoinId = ref('')
const alertSymbol = ref('')
const alertHigh = ref<number | null>(0)
const alertLow = ref<number | null>(0)

const columns: DataTableColumns<models.CachedPrice> = [
  { title: 'Coin', key: 'name', render: (row) => h('span', { style: 'font-weight:bold' }, [row.name, ' ', h('small', { style: 'color:#888' }, row.symbol)]) },
  { title: 'Price', key: 'price', render: (row) => formatPrice(row.price, row.currency) },
  { title: '24h %', key: 'change24h', render: (row) => row.price ? h('span', { style: `color:${changeColor(row.change24h)}` }, formatPct(row.change24h)) : '—' },
  { title: 'Volume', key: 'volume24h', render: (row) => row.price ? formatLarge(row.volume24h, row.currency) : '—' },
  { title: 'Market Cap', key: 'marketCap', render: (row) => row.price ? formatLarge(row.marketCap, row.currency) : '—' },
  {
    title: 'Actions', key: 'actions', render: (row) => h('div', { style: 'display:flex;gap:8px' }, [
      h(NButton, { size: 'small', disabled: !row.price, onClick: () => analyze(row.coinId, row.name) }, { default: () => '🤖 AI' }),
      h(NButton, { size: 'small', disabled: !row.price, onClick: () => openAlert(row) }, { default: () => '🔔 Alert' }),
      h(NButton, { size: 'small', type: 'error', onClick: () => remove(row.coinId, row.name) }, { default: () => '✕' }),
    ])
  }
]

const TOOL_LABELS: Record<string, string> = {
  get_price_snapshot: 'Read current price',
  get_price_history: 'Compute indicators from price history',
  get_market_overview: 'Check overall market',
  get_news: 'Scan news headlines',
}
const toolLabel = (t: string) => TOOL_LABELS[t] ?? t

function directionTag(d: string): 'success' | 'error' | 'default' {
  return d === 'bullish' ? 'success' : d === 'bearish' ? 'error' : 'default'
}
function directionLabel(d: string): string {
  return d === 'bullish' ? '▲ Bullish' : d === 'bearish' ? '▼ Bearish' : '■ Neutral'
}

async function loadPrices() {
  loading.value = prices.value.length === 0
  try {
    prices.value = await GetWatchlistPrices()
  } catch (e) {
    console.error(e)
  } finally {
    loading.value = false
  }
}

let searchTimer: ReturnType<typeof setTimeout> | undefined
function onSearch() {
  clearTimeout(searchTimer)
  if (!searchQuery.value) { searchResults.value = []; return }
  searchTimer = setTimeout(async () => {
    try {
      searchResults.value = await SearchCoins(searchQuery.value)
    } catch (e) {
      message.error(errorText(e))
    }
  }, 400)
}

async function addCoin(coin: models.CoinSearchResult) {
  try {
    await AddToWatchlist(coin.id, coin.symbol, coin.name)
    showSearch.value = false
    searchQuery.value = ''
    searchResults.value = []
    message.success('Added ' + coin.name + ' to watchlist')
    loadPrices()
  } catch (e) {
    message.error(errorText(e))
  }
}

async function remove(coinId: string, name: string) {
  await RemoveFromWatchlist(coinId)
  message.info(`Removed ${name} (and its alert)`)
  loadPrices()
}

async function analyze(coinId: string, name: string) {
  selectedCoin.value = name
  selectedCoinId.value = coinId
  showAnalysis.value = true
  analysisLoading.value = true
  analysisError.value = ''
  steps.value = []
  result.value = null
  try {
    const rec = await AnalyzeCoin(coinId)
    result.value = rec
    if (rec.cached && rec.steps) steps.value = rec.steps
  } catch (e) {
    analysisError.value = errorText(e)
  } finally {
    analysisLoading.value = false
  }
}

async function openAlert(row: models.CachedPrice) {
  alertCoinId.value = row.coinId
  alertSymbol.value = row.symbol
  alertHigh.value = 0
  alertLow.value = 0
  // Pre-fill the existing alert instead of silently overwriting it.
  const existing = await GetAlert(row.coinId)
  if (existing && existing.currency === appState.currency) {
    alertHigh.value = existing.highPrice
    alertLow.value = existing.lowPrice
  }
  showAlert.value = true
}

async function saveAlert() {
  try {
    await SetAlert(alertCoinId.value, alertSymbol.value, alertHigh.value ?? 0, alertLow.value ?? 0)
    showAlert.value = false
    message.success('Alert saved')
  } catch (e) {
    message.error(errorText(e))
  }
}

const offs: Array<() => void> = []
onMounted(() => {
  loadPrices()
  offs.push(EventsOn('prices-updated', loadPrices))
  offs.push(EventsOn('analysis-step', (data: { coinId: string; step: models.AgentStep }) => {
    if (data.coinId === selectedCoinId.value && analysisLoading.value) steps.value.push(data.step)
  }))
})
// Remove listeners when leaving the page, so they never pile up.
onUnmounted(() => {
  offs.forEach((off) => off())
  clearTimeout(searchTimer)
})
</script>
