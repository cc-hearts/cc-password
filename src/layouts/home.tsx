import { defineComponent } from 'vue'
import { ConfigProvider, theme } from 'ant-design-vue'
import { useIsDark } from '@/hooks';
export default defineComponent({
  setup() {
    const dark = useIsDark()
    console.log(theme);
    return () => (
      <main class="text-gray-700 dark:text-gray-200 flex-1">
        <ConfigProvider theme={{ algorithm: theme.darkAlgorithm }}>
          <router-view />
        </ConfigProvider>
      </main>
    )
  },
})
