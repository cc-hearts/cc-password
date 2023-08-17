import { isNull } from '@cc-heart/utils'
import { UnwrapNestedRefs, computed, reactive } from 'vue'
import type { Profile } from '@/features/user/types'
import { findSecurity } from '@/model/security'

let userProfile: null | UnwrapNestedRefs<{
  user: null | Profile
}> = null

function useProfile() {
  if (isNull(userProfile)) {
    userProfile = reactive({
      user: null,
      security: null,
    })
  }
  const profile = computed(() => userProfile?.user)
  function setUserInfo(user: Profile) {
    if (userProfile) userProfile.user = user
  }

  return { profile, setUserInfo }
}

const securityState: UnwrapNestedRefs<{
  security: null | { key: string; iv: string }
}> = reactive({
  security: null,
})

async function useSecurity() {
  const security = computed(() => securityState.security)
  if (security.value) {
    return security
  }
  const { profile } = useProfile()
  if (profile.value?.uid) {
    const entity = await findSecurity(profile.value.uid)
    if (entity) {
      securityState.security = { ...entity, key: entity.security }
      return security
    }
  }
  return null
}

export { useProfile, useSecurity }
