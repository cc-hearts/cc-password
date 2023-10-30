import { getProfile } from '@/features/user/api'
import { getToken } from '@/storage'
import { useProfile, useSecurity } from '@/storage/user'
import { defineComponent, onMounted } from 'vue'
import { useRouter } from 'vue-router'
export default defineComponent({
  setup() {
    const router = useRouter()
    const { setUserInfo } = useProfile()
    const routerGuard = async () => {
      let path = '/login'
      if (!getToken()) {
        router.push(path)
        return
      }
      const { data } = await getProfile()
      if (data) {
        setUserInfo(data)
        const security = await useSecurity()
        path = security ? '/password' : "/generatorsecurity"
      }
      router.push(path)
    }
    onMounted(() => {
      routerGuard()
    })
    return () => <div></div>
  },
})
