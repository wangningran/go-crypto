<template>
  <n-config-provider :theme="darkTheme" :theme-overrides="themeOverrides">
    <n-message-provider>
      <n-notification-provider>
        <div class="app-container">
          <!-- Top bar -->
          <div class="top-bar">
            <div class="logo">🪙 go-crypto</div>
            <div class="market-stats" v-if="overview">
              <span>Total Cap: {{ formatLargeNum(overview.totalMarketCap) }}</span>
              <span :class="overview.marketCapChange24h >= 0 ? 'up' : 'down'">
                {{ overview.marketCapChange24h >= 0 ? '▲' : '▼' }} {{ Math.abs(overview.marketCapChange24h).toFixed(2) }}%
              </span>
              <span>BTC Dom: {{ overview.btcDominance?.toFixed(1) }}%</span>
            </div>
          </div>

          <!-- Main layout -->
          <div class="main-layout">
            <!-- Sidebar -->
            <n-menu
              class="sidebar"
              :collapsed="false"
              :options="menuOptions"
              v-model:value="activeMenu"
              :indent="16"
            />

            <!-- Content -->
            <div class="content">
              <Watchlist v-if="activeMenu === 'watchlist'" />
              <MarketOverview v-else-if="activeMenu === 'market'" />
              <NewsPanel v-else-if="activeMenu === 'news'" />
              <AlertsPanel v-else-if="activeMenu === 'alerts'" />
              <SettingsPanel v-else-if="activeMenu === 'settings'" />
            </div>
          </div>
        </div>
      </n-notification-provider>
    </n-message-provider>
  </n-config-provider>
</template>

<script setup lang="ts">
import { ref, onMounted } from 'vue'
import { NConfigProvider, NMenu, NMessageProvider, NNotificationProvider, darkTheme } from 'naive-ui'
import type { MenuOption } from 'naive-ui'
import Watchlist from './components/Watchlist.vue'
import MarketOverview from './components/MarketOverview.vue'
import NewsPanel from './components/NewsPanel.vue'
import AlertsPanel from './components/AlertsPanel.vue'
import SettingsPanel from './components/SettingsPanel.vue'

declare const window: any

const activeMenu = ref('watchlist')
const overview = ref<any>(null)

const themeOverrides = {
  common: { primaryColor: '#F7931A', primaryColorHover: '#F7A840' }
}

const menuOptions: MenuOption[] = [
  { label: '📋 Watchlist', key: 'watchlist' },
  { label: '📊 Market', key: 'market' },
  { label: '📰 News', key: 'news' },
  { label: '🔔 Alerts', key: 'alerts' },
  { label: '⚙️ Settings', key: 'settings' },
]

function formatLargeNum(n: number): string {
  if (!n) return '$0'
  if (n >= 1e12) return '$' + (n / 1e12).toFixed(2) + 'T'
  if (n >= 1e9) return '$' + (n / 1e9).toFixed(2) + 'B'
  if (n >= 1e6) return '$' + (n / 1e6).toFixed(2) + 'M'
  return '$' + n.toFixed(2)
}

async function loadOverview() {
  try {
    overview.value = await window.go.main.App.GetMarketOverview()
  } catch (e) { console.error(e) }
}

onMounted(() => {
  loadOverview()
  setInterval(loadOverview, 60000)
})
</script>

<style scoped>
.app-container { display: flex; flex-direction: column; height: 100vh; background: #121212; }
.top-bar { display: flex; align-items: center; justify-content: space-between; padding: 8px 20px; background: #1a1a1a; border-bottom: 1px solid #333; height: 48px; }
.logo { font-size: 18px; font-weight: bold; color: #F7931A; }
.market-stats { display: flex; gap: 20px; font-size: 13px; color: #aaa; }
.market-stats .up { color: #2ecc71; }
.market-stats .down { color: #e74c3c; }
.main-layout { display: flex; flex: 1; overflow: hidden; }
.sidebar { width: 180px; min-width: 180px; background: #1a1a1a; border-right: 1px solid #333; }
.content { flex: 1; overflow-y: auto; padding: 20px; }
</style>
