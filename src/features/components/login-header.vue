<script setup lang="ts">
import Headers from '@/components/header/headers.vue'
import IPopover from '@/components/header/i-popover'
import { useI18n } from 'vue-i18n'
import GeneratorPasswordModal from '@/features/password/generator-password-modal'
import { reactive } from 'vue'

const { t } = useI18n()

const generaPasswordProps = reactive({
  visible: false,
  onCancel: () => {
    generaPasswordProps.visible = false
  },
})

const openPasswordModal = () => {
  generaPasswordProps.visible = true
}
</script>
<template>
  <Headers>
    <template #right-icon>
      <IPopover :content="t('headers.generateText')">
        <TagsIcon @click="openPasswordModal" />
      </IPopover>
    </template>
  </Headers>
  <GeneratorPasswordModal
    :visible="generaPasswordProps.visible"
    @cancel="generaPasswordProps.onCancel"
  />
</template>
<style lang="scss"></style>
