<template>
  <div style="max-width:600px">
    <h2 style="color:#F7931A;margin-bottom:24px">Settings</h2>
    <n-form :model="form" label-placement="left" label-width="160px">
      <n-form-item label="LLM API Key">
        <n-input
          v-model:value="newKey"
          type="password"
          show-password-on="click"
          :placeholder="form.hasApiKey ? 'Saved in system keychain (' + (form.apiKeyHint || '••••') + '). Type to replace.' : 'sk-...'"
        />
        <n-button v-if="form.hasApiKey" style="margin-left:8px" @click="clearKey">Remove</n-button>
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
    <p style="color:#777;font-size:12px">
      The model must support tool / function calling (e.g. gpt-4o-mini, deepseek-chat, or a tool-capable Ollama model).
      The API key is stored in your operating system's keychain, not in the app database.
    </p>
  </div>
</template>

<script setup lang="ts">
import { ref, onMounted } from 'vue'
import { NForm, NFormItem, NInput, NInputNumber, NSelect, NButton, useMessage } from 'naive-ui'
import { GetSettings, SaveSettings, ClearAPIKey } from '../../wailsjs/go/main/App'
import { models } from '../../wailsjs/go/models'
import { errorText } from '../utils/format'
import { appState } from '../state'

const message = useMessage()
const form = ref(new models.Settings({
  openAiBase: 'https://api.openai.com/v1', openAiModel: 'gpt-4o-mini', currency: 'usd', refreshSecs: 30,
}))
const newKey = ref('')
const currencyOptions = [
  { label: 'USD ($)', value: 'usd' },
  { label: 'EUR (€)', value: 'eur' },
  { label: 'CNY (¥)', value: 'cny' },
  { label: 'BTC (₿)', value: 'btc' },
]

async function load() {
  try {
    form.value = await GetSettings()
  } catch (e) {
    message.error(errorText(e))
  }
}

async function save() {
  try {
    await SaveSettings(models.Settings.createFrom({ ...form.value, apiKey: newKey.value }))
    newKey.value = ''
    appState.currency = form.value.currency
    message.success('Settings saved!')
    load()
  } catch (e) {
    message.error('Failed to save: ' + errorText(e))
  }
}

async function clearKey() {
  try {
    await ClearAPIKey()
    message.success('API key removed')
    load()
  } catch (e) {
    message.error(errorText(e))
  }
}

onMounted(load)
</script>
