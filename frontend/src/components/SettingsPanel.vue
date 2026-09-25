<template>
  <div style="max-width:600px">
    <h2 style="color:#F7931A;margin-bottom:24px">Settings</h2>
    <n-form :model="form" label-placement="left" label-width="160px">
      <n-form-item label="OpenAI API Key">
        <n-input v-model:value="form.openAiKey" type="password" show-password-on="click" placeholder="sk-..." />
      </n-form-item>
      <n-form-item label="API Base URL">
        <n-input v-model:value="form.openAiBase" placeholder="https://api.openai.com/v1" />
      </n-form-item>
      <n-form-item label="Model">
        <n-input v-model:value="form.openAiModel" placeholder="gpt-4o-mini" />
      </n-form-item>
      <n-form-item label="Currency">
        <n-select v-model:value="form.currency" :options="currencyOptions" />
      </n-form-item>
      <n-form-item label="Refresh (seconds)">
        <n-input-number v-model:value="form.refreshSecs" :min="10" :max="300" />
      </n-form-item>
      <n-form-item>
        <n-button type="primary" @click="save">Save Settings</n-button>
      </n-form-item>
    </n-form>
  </div>
</template>

<script setup lang="ts">
import { ref, onMounted } from 'vue'
import { NForm, NFormItem, NInput, NInputNumber, NSelect, NButton, useMessage } from 'naive-ui'
declare const window: any

const message = useMessage()
const form = ref({ openAiKey: '', openAiBase: 'https://api.openai.com/v1', openAiModel: 'gpt-4o-mini', currency: 'usd', refreshSecs: 30 })
const currencyOptions = [
  { label: 'USD ($)', value: 'usd' },
  { label: 'EUR (€)', value: 'eur' },
  { label: 'CNY (¥)', value: 'cny' },
  { label: 'BTC (₿)', value: 'btc' },
]

async function load() {
  try {
    const s = await window.go.main.App.GetSettings()
    if (s) Object.assign(form.value, s)
  } catch(e) { console.error(e) }
}

async function save() {
  try {
    await window.go.main.App.SaveSettings(form.value)
    message.success('Settings saved!')
  } catch(e) { message.error('Failed to save: ' + e) }
}

onMounted(load)
</script>
