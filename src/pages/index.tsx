import { getProfile, refreshTokenApi } from '@/features/user/api'
import { getRefreshToken, setRefreshToken, setToken } from '@/storage'
import { useProfile, useSecurity } from '@/storage/user'
import { defineComponent, onMounted } from 'vue'
import { useRouter } from 'vue-router'
export default defineComponent({
  setup() {
    const router = useRouter()
    const { setUserInfo } = useProfile()
    const routerGuard = async () => {
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
        const refreshToken = getRefreshToken()
        if (refreshToken) {
          const { data } = await refreshTokenApi(refreshToken)
          if (data) {
            const { accessToken, refreshToken } = data
            setToken(accessToken)
            setRefreshToken(refreshToken)
            await routerGuard()
            return
          }
        }
        path = '/login'
      }
      router.push(path)
    }
    onMounted(() => {
      routerGuard()
    })
    return () => <div></div>
  },
})
