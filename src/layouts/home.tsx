import { defineComponent } from 'vue'
import { ConfigProvider, theme } from 'ant-design-vue'
import { isDark } from '@/configs';
export default defineComponent({
  setup() {
    return () => (
      <main class="text-gray-700 dark:text-gray-200 flex-1">
        <ConfigProvider theme={{ algorithm: isDark.value ? theme.darkAlgorithm : theme.defaultAlgorithm }}>
          <router-view />
        </ConfigProvider>
      </main>
    )
  },
})
