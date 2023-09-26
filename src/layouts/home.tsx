import { defineComponent } from 'vue'

export default defineComponent({
  setup() {
    return () => (
      <main class="text-gray-700 dark:text-gray-200 flex-1 flex flex-col min-h-0">
        <router-view />
      </main>
    )
  },
})
