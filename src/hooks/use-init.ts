import { router } from '@/modules/router'
import { getProfile } from '@/features/user/api'
import { getToken } from '@/storage'
import { useProfile, useSecurity } from '@/storage/user'
export async function useInit() {
  let path = '/login'
  if (!getToken()) {
    router.push(path)
    return
  }
  const { setUserInfo } = useProfile()

  const { data } = await getProfile()
  if (data) {
    setUserInfo(data)
    const security = await useSecurity()
    path = security ? '/password' : '/generatorsecurity'
  }
  console.log(router)
  router.push(path)
}
