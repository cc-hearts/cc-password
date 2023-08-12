import { isNull } from '@cc-heart/utils'
import { UnwrapNestedRefs, computed, reactive } from 'vue'
import type { Profile } from '@/features/user/types'

let state: null | UnwrapNestedRefs<{
  user: null | Profile
  security: null | { key: string; iv: string }
}> = null

function useProfile() {
  if (isNull(state)) {
    state = reactive({
      user: null,
      security: null,
    })
  }
  const profile = computed(() => state?.user)
  const security = computed(() => state?.security)
  function setUserInfo(user: Profile) {
    if (state) state.user = user
  }

  function setSecurity<T extends { key: string; iv: string }>(security: T) {
    if (state) state.security = security
  }
  return { profile, setUserInfo, security, setSecurity }
}

export { useProfile }
