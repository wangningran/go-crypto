<template>
  <div>
    <div style="display:flex; justify-content:space-between; align-items:center; margin-bottom:8px">
      <h2 style="color:#F7931A">AI Track Record</h2>
      <n-space>
        <n-button @click="evaluateNow" :loading="evaluating">Score due analyses</n-button>
        <n-button @click="load">🔄 Refresh</n-button>
      </n-space>
    </div>
    <p style="color:#888;font-size:13px;margin-bottom:16px">
      Every AI analysis makes a 24h call. 24 hours later the app checks the real price:
      bullish is right if the price rose, bearish if it fell, neutral if it moved less than ±{{ stats?.neutralBandPct ?? 1 }}%.
    </p>

    <n-grid :cols="4" :x-gap="16" style="margin-bottom:24px" v-if="stats">
      <n-grid-item><n-statistic label="Direction accuracy" :value="stats.evaluated ? pct(stats.accuracy) : '—'" /></n-grid-item>
      <n-grid-item><n-statistic label="Scored" :value="stats.evaluated" /></n-grid-item>
      <n-grid-item>
        <n-statistic label="Waiting for 24h" :value="stats.pending" />
        <div v-if="stats.unscorable" style="color:#888;font-size:12px">+ {{ stats.unscorable }} unscorable (price unavailable)</div>
      </n-grid-item>
      <n-grid-item>
        <n-statistic label="Avg confidence: right / wrong"
          :value="stats.evaluated ? pct(stats.avgConfidenceCorrect) + ' / ' + pct(stats.avgConfidenceWrong) : '—'" />
      </n-grid-item>
    </n-grid>

    <n-data-table v-if="stats && stats.evaluated" :columns="dirColumns" :data="stats.byDirection" size="small" style="margin-bottom:24px;max-width:520px" />

    <h3 style="margin-bottom:12px;color:#aaa">History</h3>
    <n-empty v-if="!loading && history.length === 0" description="No analyses yet. Run one from the Watchlist (🤖 AI)." style="margin-top:40px" />
    <n-data-table v-else :columns="columns" :data="history" :loading="loading" striped :max-height="480" />
  </div>
</template>

<script setup lang="ts">
import { ref, onMounted, h } from 'vue'
import { NButton, NDataTable, NGrid, NGridItem, NStatistic, NEmpty, NSpace, NTag, useMessage } from 'naive-ui'
import type { DataTableColumns } from 'naive-ui'
import { GetAnalysisHistory, GetEvalStats, EvaluateNow } from '../../wailsjs/go/main/App'
import type { models } from '../../wailsjs/go/models'
import { formatPrice, formatPct, formatTime, changeColor, errorText } from '../utils/format'

const message = useMessage()
const stats = ref<models.EvalStats | null>(null)
const history = ref<models.AnalysisRecord[]>([])
const loading = ref(false)
const evaluating = ref(false)

const pct = (x: number) => Math.round(x * 100) + '%'
const scored = (r: models.AnalysisRecord) => r.correct !== undefined && r.correct !== null

const dirColumns: DataTableColumns<models.DirectionStats> = [
  { title: 'Call', key: 'direction' },
  { title: 'Scored', key: 'evaluated' },
  { title: 'Right', key: 'correct' },
  { title: 'Accuracy', key: 'accuracy', render: (r) => (r.evaluated ? pct(r.accuracy) : '—') },
]

const columns: DataTableColumns<models.AnalysisRecord> = [
  { title: 'Time', key: 'createdAt', render: (r) => formatTime(r.createdAt) },
  { title: 'Coin', key: 'symbol' },
  {
    title: 'Call', key: 'direction',
    render: (r) => h(NTag, { size: 'small', type: r.direction === 'bullish' ? 'success' : r.direction === 'bearish' ? 'error' : 'default' },
      { default: () => r.direction }),
  },
  { title: 'Conf.', key: 'confidence', render: (r) => pct(r.confidence) },
  { title: 'Price then', key: 'priceAtAnalysis', render: (r) => formatPrice(r.priceAtAnalysis, r.currency) },
  { title: 'Price +24h', key: 'priceAfter24h', render: (r) => (scored(r) ? formatPrice(r.priceAfter24h, r.currency) : '—') },
  { title: 'Move', key: 'returnPct', render: (r) => (scored(r) ? h('span', { style: `color:${changeColor(r.returnPct)}` }, formatPct(r.returnPct)) : '—') },
  {
    title: 'Result', key: 'correct',
    render: (r) => r.correct === undefined || r.correct === null
      ? (r.evaluatedAt
        ? h('span', { style: 'color:#888', title: r.evalError }, 'unscorable')
        : h('span', { style: 'color:#888', title: r.evalError || '' }, r.evalAttempts ? `pending (retry ${r.evalAttempts})` : 'pending'))
      : h(NTag, { size: 'small', type: r.correct ? 'success' : 'error' }, { default: () => (r.correct ? '✓ right' : '✗ wrong') }),
  },
  { title: 'Model', key: 'model', render: (r) => h('small', { style: 'color:#888' }, r.model) },
]

async function load() {
  loading.value = true
  try {
    ;[stats.value, history.value] = await Promise.all([GetEvalStats(), GetAnalysisHistory(200)])
  } catch (e) {
    message.error(errorText(e))
  } finally {
    loading.value = false
  }
}

async function evaluateNow() {
  evaluating.value = true
  try {
    stats.value = await EvaluateNow()
    history.value = await GetAnalysisHistory(200)
  } catch (e) {
    message.error(errorText(e))
  } finally {
    evaluating.value = false
  }
}

onMounted(load)
</script>
