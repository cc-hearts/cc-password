<script setup lang="ts">
import { isRef, ref } from 'vue'
import formProps from './form'

const props = defineProps(formProps)
const formState = ref({ ...props.defaultValue })

const getFieldsValue = () => {
  return formState.value
}
const setFieldValue = (key: PropertyKey, val: any) => {
  Reflect.set(formState.value, key, val)
}

const setFieldsValue = (target: Record<PropertyKey, any>) => {
  Object.assign(formState.value, target)
}

const formRef = ref()

const validate = async () => {
  try {
    await formRef.value.validate()
  } catch (e) {
    return false
  }
  return true
}

const resetFields = () => {
  formRef.value.resetFields()
}

defineExpose({
  getFieldsValue,
  setFieldValue,
  setFieldsValue,
  resetFields,
  validate,
})
</script>
<template>
  <a-form
    ref="formRef"
    :label-col="props.labelCol"
    :model="formState"
    :name="props.name"
    :rules="props.rules"
  >
    <template v-for="item in props.columns">
      <a-form-item :label="item.label" :name="item.name">
        <template v-if="item.slot">
          <slot :name="item.slot.name" :formState="formState" />
        </template>
        <template v-else-if="item.type === 'input'">
          <a-input
            v-model:value="formState[item.name as keyof typeof formState]"
            :placeholder="'请输入' + item.label"
            v-bind="item.extra"
          />
        </template>
        <template v-else-if="item.type === 'input-number'">
          <a-input-number
            class="w-full"
            v-model:value="formState[item.name as keyof typeof formState]"
            :placeholder="'请输入' + item.label"
            v-bind="item.extra"
          />
        </template>
        <template v-else-if="item.type === 'select'">
          <a-select
            v-model:value="formState[item.name as keyof typeof formState]"
            :placeholder="'请选择' + item.label"
            v-bind="item.extra"
            :options="
              isRef(item.extra?.options)
                ? item.extra?.options.value
                : item.extra?.options || []
            "
          >
          </a-select>
        </template>
        <template v-else-if="item.type === 'radio'">
          <a-radio-group
            v-model:value="formState[item.name as keyof typeof formState]"
            v-bind="item.extra"
          >
            <template v-for="l in item.options" :key="l.value">
              <a-radio :value="l.value">{{ l.label }}</a-radio>
            </template>
          </a-radio-group>
        </template>
      </a-form-item>
    </template>
  </a-form>
</template>
<style lang="scss"></style>
