<script setup lang='ts'>
import { InputPassword } from 'ant-design-vue'
import { ref } from 'vue';
const value = ref<string>('')
</script>
<template>
  <div class="h-full w-full flex justify-center items-center">
    <div class="w-80">
      <InputPassword v-model="value" />
    </div>
  </div>
</template>
<style lang='scss'></style>