import { getProfile } from '@/features/user/api'
import { findSecurity } from '@/model/security'
import { useProfile } from '@/storage/user'
import { defineComponent, onMounted } from 'vue'
import { useRouter } from 'vue-router'
export default defineComponent({
  setup() {
    const router = useRouter()
    const { setUserInfo, setSecurity } = useProfile()

    onMounted(async () => {
      let path
      const { data } = await getProfile()
      if (data) {
        setUserInfo(data)
        const entity = await findSecurity(data.uid)
        if (entity) {
          setSecurity({ ...entity, key: entity.security })
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
