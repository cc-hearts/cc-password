<script setup lang="ts">
import { Form, FormExpose } from '@/components/form'
import { Modal, defineModal } from '@/components/modal'
import { useModalFormExpose } from '@/hooks/use-modal-form-expose'
import { addPasswordCategory } from '@/model/password-category'
import { errorTips, successTips } from '@/utils/message'
import { noop } from '@cc-heart/utils'
import { computed, ref, toRaw } from 'vue'
import { useI18n } from 'vue-i18n'

const { t } = useI18n()
const modalProps = defineModal({
  title: () => t('side.addCategoryModal.modalTitle'),
})
const props = defineProps({
  onRefresh: {
    type: Function,
    default: noop,
  },
})
const formRef = ref<FormExpose>()
const formRules = {
  category: [
    {
      required: true,
      message: t('side.addCategoryModal.categoryFieldPlaceholder'),
    },
  ],
}
const formColumn = computed(() => {
  return [
    {
      label: t('side.addCategoryModal.categoryName'),
      name: 'category',
      type: 'input',
    },
  ]
})

const onOpen = () => {
  modalProps.visible = true
}

const modalFormExposeResult = useModalFormExpose(formRef)

defineExpose({ onOpen, ...modalFormExposeResult })

const handleAddCategory = async () => {
  if (!formRef.value) return
  const bool = await formRef.value.validate()
  if (!bool) return
  const params = {
    ...toRaw(formRef.value.getFieldsValue<{ category: string }>()),
  }
  if (!(params.category = params.category?.trim())) {
    errorTips(t('side.addCategoryModal.errorCategoryName'))
    return
  }
  await addPasswordCategory(params)
  successTips(t('side.addCategoryModal.successMsg'))
  formRef.value.resetFields()
  props.onRefresh()
  modalProps.visible = false
}
</script>
<template>
  <Modal
    v-bind="modalProps"
    v-model:visible="modalProps.visible"
    @ok="handleAddCategory"
  >
    <Form ref="formRef" :columns="formColumn" :rules="formRules"> </Form>
  </Modal>
</template>
