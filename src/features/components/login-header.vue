<script setup lang="ts">
import Headers from '@/components/header/headers.vue'
import GeneratorPasswordModal from '@/features/password/generator-password-modal'
import { reactive } from 'vue'

const generaPasswordProps = reactive({
  visible: false,
  onCancel: () => {
    generaPasswordProps.visible = false
  },
})
</script>
<template>
  <Headers> </Headers>
  <GeneratorPasswordModal
    :visible="generaPasswordProps.visible"
    @cancel="generaPasswordProps.onCancel"
  />
</template>
<style lang="scss"></style>
