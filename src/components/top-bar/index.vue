<script setup lang="ts">
import { useCssNamespace } from '@/hooks'
import {
  CloseOutlined,
  MinusOutlined,
  PlusOutlined,
} from '@ant-design/icons-vue'
import { ipcRenderer } from 'electron'
const cssNs = useCssNamespace('top-bar')

const handleCloseWindow = () => {
  ipcRenderer.invoke('hidden-window')
}

const handleChangeWindowSize = () => {
  ipcRenderer.invoke('change-window-size')
}
</script>
<template>
  <div class="flex gap-1.5" :class="[cssNs.cls]">
    <span @click="handleCloseWindow" class="app-no-drag">
      <CloseOutlined />
    </span>
    <span @click="handleCloseWindow" class="app-no-drag">
      <MinusOutlined />
    </span>
    <span @click="handleChangeWindowSize" class="app-no-drag">
      <PlusOutlined />
    </span>
  </div>
</template>
<style lang="scss">
@use '@/assets/scss/var/variable.scss' as *;
@use '@/assets/scss/common/mixins.scss' as *;
@use '@/assets/scss/common/function.scss' as *;

@include b('top-bar') {
  &:hover {
    svg {
      opacity: 1;
    }
  }

  & > span {
    width: 12px;
    height: 12px;
    font-size: 8px;
    color: #000;
    border-radius: 50%;
    display: flex;
    align-items: center;
    justify-content: center;

    svg {
      opacity: 0;
    }

    &:nth-child(1) {
      background-color: #e65f52;
    }

    &:nth-child(2) {
      background-color: #f5bc42;
    }

    &:nth-child(3) {
      background-color: #5cb85c;
    }
  }
}
</style>
