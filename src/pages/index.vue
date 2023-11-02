<script setup lang="ts">
import Headers from '@/components/header/headers.vue'
import GeneratorPasswordModal from '@/features/password/generator-password-modal'
import { useCssNamespace } from '@/hooks'
import { reactive } from 'vue'
import { useI18n } from 'vue-i18n'
import LogOut from '@/icons/log-out.vue'
import IPopover from '@/components/header/i-popover'
import { Modal, Popover } from 'ant-design-vue'
import { MoreOutlined } from '@ant-design/icons-vue'
import { clearRefreshToken, clearToken } from '@/storage'
import { useRouter } from 'vue-router'
import CategorySide from '@/features/components/side'

import TagsIcon from '@/icons/tags.vue'
const { t } = useI18n()
const router = useRouter()
const ns = useCssNamespace('header')
const generaPasswordProps = reactive({
  visible: false,
  onCancel: () => {
    generaPasswordProps.visible = false
  },
})

const openPasswordModal = () => {
  generaPasswordProps.visible = true
}

const handleLogout = () => {
  Modal.confirm({
    title: t('headers.title'),
    content: t('headers.content'),
    okText: t('headers.okText'),
    cancelText: t('headers.cancelText'),
    onOk() {
      clearToken()
      clearRefreshToken()
      router.push('/login')
    },
  })
}
</script>
<template>
  <div class="h-full w-full flex">
    <CategorySide />
    <div class="flex-1">
      <Headers>
        <template #right-icon>
          <IPopover :content="t('headers.generateText')">
            <TagsIcon @click="openPasswordModal" />
          </IPopover>
        </template>
        <template #right>
          <Popover
            :overlayClassName="ns.e('custom-popover')"
            placement="bottomRight"
          >
            <template #content>
              <div
                class="flex items-center"
                :class="[ns.e('popover')]"
                @click="handleLogout"
              >
                <LogOut />
                <div class="popover__split"></div>
                <span>{{ $t('logout') }}</span>
              </div>
            </template>
            <MoreOutlined />
          </Popover>
        </template>
      </Headers>
      <main
        class="text-gray-700 dark:text-gray-200 flex-1 flex flex-col min-h-0"
      >
        <router-view />
      </main>
    </div>
  </div>

  <GeneratorPasswordModal
    :visible="generaPasswordProps.visible"
    @cancel="generaPasswordProps.onCancel"
  />
</template>
<style lang="scss"></style>
