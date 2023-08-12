<script setup lang="ts">
import { InputPassword, Button, Input } from 'ant-design-vue'
import { reactive } from 'vue'
import { getProfile, loginApi } from '@/features/user/api'
import { setToken, setRefreshToken } from '@/storage'
import { useRouter } from 'vue-router'
import { findSecurity } from '@/model/security'
import { useProfile } from '@/storage/user'
import { isNull } from '@cc-heart/utils'
const router = useRouter()
const userInfo = reactive({
  username: '',
  password: '',
})
const handleSubmit = async () => {
  const { username, password } = userInfo
  const { data } = await loginApi({ username, password: password })
  if (data) {
    const { accessToken, refreshToken } = data
    setToken(accessToken)
    setRefreshToken(refreshToken)
    const { data: profile } = await getProfile()
    const { setUserInfo } = useProfile()
    if (profile) {
      setUserInfo(profile)
      const security = await findSecurity(profile.uid)
      if (isNull(security)) {
        router.push('/generatorsecurity')
        return
      }
      router.push({ path: '/password' })
    }
  }
}
</script>
<template>
  <div class="h-full w-full flex justify-center items-center">
    <div class="w-80 text-center">
      <Input
        class="m-b-6"
        v-model:value="userInfo.username"
        :placeholder="$t('pages-index.usernamePlaceholder')"
      />
      <InputPassword
        v-model:value="userInfo.password"
        :placeholder="$t('pages-index.passwordPlaceholder')"
      />
      <Button class="m-t-6" @click="handleSubmit">{{
        $t('pages-index.submitButton')
      }}</Button>
    </div>
  </div>
</template>
<style lang="scss"></style>
@/features/user/api
