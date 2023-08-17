import { getProfile } from '@/features/user/api'
import { useProfile, useSecurity } from '@/storage/user'
import { defineComponent, onMounted } from 'vue'
import { useRouter } from 'vue-router'
export default defineComponent({
  setup() {
    const router = useRouter()
    const { setUserInfo } = useProfile()

    onMounted(async () => {
      let path
      const { data } = await getProfile()
      if (data) {
        setUserInfo(data)
        const security = await useSecurity()
        if (security) {
          path = '/password'
        } else {
          path = '/generatorsecurity'
        }
      } else {
        path = '/login'
      }
      router.push(path)
    })
    return () => <div></div>
  },
})
