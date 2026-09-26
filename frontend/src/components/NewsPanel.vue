<template>
  <div>
    <div style="display:flex;justify-content:space-between;align-items:center;margin-bottom:16px">
      <h2 style="color:#F7931A">Crypto News</h2>
      <n-button @click="load">🔄 Refresh</n-button>
    </div>
    <n-alert v-if="error" type="error" style="margin-bottom:16px">{{ error }}</n-alert>
    <n-empty v-else-if="!loading && news.length === 0" description="No news right now." style="margin-top:60px" />
    <n-spin :show="loading">
      <n-list bordered>
        <n-list-item v-for="item in news" :key="item.url">
          <n-thing :title="item.title">
            <template #description>
              <span style="color:#888;font-size:12px">{{ item.source }} · {{ item.publishedAt }}</span>
              <div style="margin-top:4px;font-size:13px;color:#ccc">{{ item.summary }}</div>
            </template>
            <template #action>
              <n-button text tag="a" :href="item.url" target="_blank" type="primary">Read more →</n-button>
            </template>
          </n-thing>
        </n-list-item>
      </n-list>
    </n-spin>
  </div>
</template>

<script setup lang="ts">
import { ref, onMounted } from 'vue'
import { NList, NListItem, NThing, NSpin, NButton, NAlert, NEmpty } from 'naive-ui'
import { GetNews } from '../../wailsjs/go/main/App'
import type { models } from '../../wailsjs/go/models'
import { errorText } from '../utils/format'

const news = ref<models.NewsItem[]>([])
const loading = ref(false)
const error = ref('')

async function load() {
  loading.value = true
  error.value = ''
  try { news.value = await GetNews() }
  catch (e) { error.value = errorText(e) }
  finally { loading.value = false }
}

onMounted(load)
</script>
