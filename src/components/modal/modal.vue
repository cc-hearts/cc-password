<script setup lang="ts">
import { noop } from '@cc-heart/utils'
import { PropType, computed } from 'vue'

const props = defineProps({
  title: [String, Function] as PropType<string | (() => string)>,
  visible: Boolean,
  okText: {
    type: String,
    default: '确定',
  },
  cancelText: {
    type: String,
    default: '取消',
  },
  onCancel: {
    type: Function as PropType<() => any>,
    default: noop,
  },
})

const getModalTitle = computed(() => {
  return props.title instanceof Function ? props.title() : props.title
})
const handleOk = () => {
  emits('ok')
}
const emits = defineEmits(['update:visible', 'ok'])
const handleCancel = () => {
  emits('update:visible', false)
}
</script>
<template>
  <a-modal
    :open="props.visible"
    :title="getModalTitle"
    :ok-text="props.okText"
    :cancel-text="props.cancelText"
    @ok="handleOk"
    @cancel="handleCancel"
  >
    <slot />
  </a-modal>
</template>
<style lang="scss"></style>
