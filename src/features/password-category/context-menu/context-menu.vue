<script setup lang="ts">
import { useCssNamespace } from '@/hooks/use-css-namespace'
import { removePasswordCategory } from '@/model/password-category'
import { successMsg } from '@/utils/message'
import { isNull, noop } from '@cc-heart/utils'
import { Modal } from 'ant-design-vue'
import { onMounted, onUnmounted, reactive } from 'vue'
import { useI18n } from 'vue-i18n'
import ContextMenuContent from './context-menu-content.vue'

const props = defineProps({
  id: {
    type: Number,
    default: null,
  },
  onRefreshCategoryList: {
    type: Function,
    default: noop,
  },
})

const emits = defineEmits<{
  (event: 'refreshCategoryList'): void
}>()

const { t } = useI18n()
const cssNs = useCssNamespace('context-menu')

const menuTeleportTarget = reactive({
  visible: false,
  style: {
    left: '0',
    top: '0',
  },
})

const handleContextMenuEvent = (e: MouseEvent) => {
  menuTeleportTarget.visible = true
  const { pageX, pageY } = e
  menuTeleportTarget.style.left = `${pageX}px`
  menuTeleportTarget.style.top = `${pageY}px`
}
const cancelContextMenu = () => {
  if (menuTeleportTarget.visible === false) return
  menuTeleportTarget.visible = false
}

const handleMenuClick = (e: MouseEvent) => {
  e.stopPropagation()
}

onMounted(() => {
  window.addEventListener('click', cancelContextMenu)
})

onUnmounted(() => {
  window.removeEventListener('click', cancelContextMenu)
})

async function removeCategory() {
  if (isNull(props.id) && Number(props.id)) return
  await removePasswordCategory(props.id)
  successMsg(t('pages-password.removeCategorySuccessMessage'))
  props.onRefreshCategoryList()
}

const removeCategoryConfirm = async () => {
  Modal.confirm({
    title: () => t('pages-password.removeCategoryConfirmTitle'),
    onOk: removeCategory,
  })
}
</script>
<template>
  <div :class="[cssNs.cls]" @contextmenu="handleContextMenuEvent">
    <slot></slot>
  </div>
  <Teleport to="body">
    <!-- TODO: use v-lazy-show to replace v-if -->
    <ul
      class="absolute"
      v-if="menuTeleportTarget.visible"
      :class="[cssNs.e('menu')]"
      :style="menuTeleportTarget.style"
      @click="handleMenuClick"
    >
      <ContextMenuContent @remove-directory="removeCategoryConfirm" />
    </ul>
  </Teleport>
</template>
<style lang="scss">
@use '@/assets/scss/common/function.scss' as *;
@use '@/assets/scss/common/mixins.scss' as *;
@use '@/assets/scss/var/variable.scss' as *;

@include b('context-menu') {
  display: contents;
  user-select: none;
}
</style>
