<script setup lang="ts">
import { useCssNamespace } from '@/hooks'

const cssNs = useCssNamespace('context-menu-content')

const emits = defineEmits<{
  (event: 'removeDirectory'): void
}>()

const emitRemoveDirectory = () => {
  emits('removeDirectory')
}
</script>
<template>
  <ul :class="[cssNs.cls, 'p-2 border border-solid rounded']">
    <li @click="emitRemoveDirectory">
      {{ $t('side-context-menu.remove-type') }}
    </li>
  </ul>
</template>
<style lang="scss">
@use '@/assets/scss/common/function.scss' as *;
@use '@/assets/scss/common/mixins.scss' as *;
@use '@/assets/scss/var/variable.scss' as *;

@include b('context-menu-content') {
  background-color: #424242;
  border-color: #7d6b6b;
  font-size: 14px;
  user-select: none;
  & > li {
    padding: 2px 8px;
    cursor: pointer;
    border-radius: 4px;
    background-color: transparent;
    transition: all 0.3s;

    &:hover {
      background-color: #5c5656;
    }
  }
}
</style>
