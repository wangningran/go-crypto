<template>
  <div>
    <div style="display:flex;justify-content:space-between;align-items:center;margin-bottom:16px">
      <h2 style="color:#F7931A">Crypto News</h2>
      <n-button @click="load">🔄 Refresh</n-button>
    </div>
    <n-spin :show="loading">
      <n-list bordered>
        <n-list-item v-for="item in news" :key="item.url">
          <n-thing :title="item.title" :description="item.source + ' · ' + item.publishedAt">
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
import { NList, NListItem, NThing, NSpin, NButton } from 'naive-ui'
declare const window: any

interface NewsItem { title: string; url: string; source: string; publishedAt: string; summary: string }
const news = ref<NewsItem[]>([])
const loading = ref(false)

async function load() {
  loading.value = true
  try { news.value = await window.go.main.App.GetNews() }
  catch(e) { console.error(e) }
  finally { loading.value = false }
}

onMounted(load)
</script>
