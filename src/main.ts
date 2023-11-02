import { createApp } from 'vue'
import App from './App.vue'
import 'uno.css'
import '@/assets/scss/theme.scss'
import './main.css'
import './modules/i18n'
import { useInitTheme } from '@/hooks'
import { registerDirective } from './directives'
import { useInit } from './hooks/use-init'
;(async () => {
  useInitTheme()
  const app = createApp(App)
  Object.entries(import.meta.glob('./modules/*.ts', { eager: true })).forEach(
    ([, Module]) => {
      // eslint-disable-next-line @typescript-eslint/ban-ts-comment
      // @ts-ignore
      Module.setup?.({ app })
    }
  )

  registerDirective(app)
  useInit().finally(() => {
    app.mount('#app')
  })
})()
