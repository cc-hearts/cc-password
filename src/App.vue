<script setup lang="ts">
import Headers from '@/components/header/headers.vue'
import Home from '@/layouts/home'
import { ipcRenderer } from 'electron'
import { onMounted } from 'vue'

onMounted(() => {
  console.log('123')
  ipcRenderer.invoke('showWindow')
})
</script>

<template>
  <Headers />
  <Home />
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
