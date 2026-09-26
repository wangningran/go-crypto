<template>
  <span style="display:none" />
</template>

<script setup lang="ts">
// Registers app-wide backend event listeners exactly once. It lives inside
// App.vue (which never unmounts), so switching tabs can't duplicate listeners.
import { onMounted, onUnmounted } from 'vue'
import { useNotification } from 'naive-ui'
import { EventsOn } from '../../wailsjs/runtime/runtime'

const notification = useNotification()
const offs: Array<() => void> = []
let lastDataError = 0

onMounted(() => {
  offs.push(EventsOn('price-alert', (data: { symbol: string; message: string }) => {
    notification.warning({
      title: '🔔 Price alert: ' + data.symbol,
      content: data.message + '. The alert is now paused; resume it on the Alerts page.',
      duration: 0, // stays until dismissed
    })
  }))
  offs.push(EventsOn('data-error', (msg: string) => {
    // At most one data error toast per minute.
    if (Date.now() - lastDataError < 60_000) return
    lastDataError = Date.now()
    notification.error({ title: 'Price refresh failed', content: msg, duration: 8000 })
  }))
})

onUnmounted(() => offs.forEach((off) => off()))
</script>
