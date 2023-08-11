<script setup lang='ts'>
import { InputPassword, Button } from 'ant-design-vue'
import { ref } from 'vue';
const value = ref<string>('')
</script>
<template>
  <div class="h-full w-full flex justify-center items-center">
    <div class="w-80 text-center">
      <InputPassword v-model="value" />
      <Button class="m-t-6">Submit</Button>
    </div>
  </div>
</template>
<style lang='scss'></style>