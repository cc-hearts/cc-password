import '@/assets/scss/pages/password.scss'
import PasswordDescription from '@/features/password/password-description'
import PasswordList from '@/features/password/password-list'
import { defineComponent } from 'vue'

export default defineComponent({
  setup() {
    return () => (
      <div class="flex h-full w-full">
        <div class="w-1/3 min-w-xs p-x-2 border-l-0 border-r-1 border-ins border-x-solid shrink-0">
          <PasswordList />
        </div>
        <div class="flex-1 overflow-auto " style="height:calc(100vh - 48px)">
          <PasswordDescription />
        </div>
      </div>
    )
  },
})
