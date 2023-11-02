<script setup lang="ts">
import { ipcRenderer } from 'electron'
import { onMounted } from 'vue'
import { ConfigProvider, theme } from 'ant-design-vue'
import { isDark } from '@/configs'

onMounted(() => {
  ipcRenderer.invoke('show-window')
})
</script>

<template>
  <ConfigProvider
    :theme="{
      algorithm: isDark ? theme.darkAlgorithm : theme.defaultAlgorithm,
    }"
  >
    <router-view />
  </ConfigProvider>
</template>

<style lang="scss">
* {
  margin: 0;
  padding: 0;
  box-sizing: border-box;
}

body,
html,
#app {
  width: 100%;
  height: 100%;
}

#app {
  display: flex;
  flex-direction: column;
}
</style>
