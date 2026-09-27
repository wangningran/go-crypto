import { reactive } from 'vue'

// App-wide UI state shared between components.
export const appState = reactive({
  currency: 'usd',
})
