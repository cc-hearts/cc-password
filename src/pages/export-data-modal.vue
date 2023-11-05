<script setup lang="ts">
import { Form, FormExpose } from '@/components/form'
import { Modal, defineModal } from '@/components/modal'
import { ExportDataForm } from '@/features/password/types'
import { useModalFormExpose } from '@/hooks/use-modal-form-expose'
import { useProfile } from '@/storage'
import { getCurrentTimeISOString } from '@cc-heart/utils'
import { ipcRenderer } from 'electron'
import { computed, onMounted, onUnmounted, ref, nextTick } from 'vue'
import { useI18n } from 'vue-i18n'


const { t } = useI18n()
const formRef = ref<FormExpose>()

const modalProps = defineModal({
  title: () => t('exportDataModal.modalTitle'),
})

const emits = defineEmits<{
  (event: 'ok', args: ExportDataForm): void
}>()

const { profile } = useProfile()

const onOpen = async () => {
  modalProps.visible = true
  const currentTime = getCurrentTimeISOString()
  const timerSlice = currentTime.split(/[T.]/)
  const [date, time] = timerSlice
  const exportName = `${profile.value?.name}-${date}-${time.replace(/:/g, '-')}.json`
  await nextTick()
  const exportSelectPath = localStorage.getItem('exportDataPath') || undefined
  formRef.value?.setFieldsValue({ exportName, exportSelectPath })
}

const onCancel = () => {
  formRef.value?.resetFields()
  modalProps.visible = false
}

const modalFormExposeResult = useModalFormExpose(formRef)
defineExpose({ onOpen, onCancel, ...modalFormExposeResult })

const formRules = {
  exportName: [
    {
      required: true,
      message: t('exportDataModal.exportNamePlaceholder'),
    }

  ],
  exportSelectPath: [
    {
      required: true,
      message: t('exportDataModal.exportSelectPathPlaceholder'),
    }
  ]
}

const formColumn = computed(() => {
  return [
    {
      label: t('exportDataModal.exportName'),
      name: 'exportName',
      type: 'input',
    },
    {
      label: t('exportDataModal.exportSelectPath'),
      name: 'exportSelectPath',
      type: 'select',
      slot: { name: 'selectPath' }
    }
  ]
})

const openSearchDirPath = () => {
  ipcRenderer.invoke('open-search-dir-path')
}
const setSelectDirPath = (_: any, arg: string) => {
  formRef.value?.setFieldsValue({ exportSelectPath: arg })
}

const validateAndEmitFormData = () => {
  if (!formRef.value) return
  const bool = formRef.value?.validate()
  if (!bool) return
  const formData = formRef.value.getFieldsValue<ExportDataForm>()
  emits('ok', formData)
}
onMounted(() => {
  ipcRenderer.on('selected-dir-path', setSelectDirPath)
})

onUnmounted(() => {
  ipcRenderer.off('selected-dir-path', setSelectDirPath)
})


</script>
<template>
  <Modal v-bind="modalProps" v-model:visible="modalProps.visible" @ok="validateAndEmitFormData">
    <Form ref="formRef" :columns="formColumn" :rules="formRules">
      <template #selectPath="{ formState }">
        <a-input-search :value="formState.exportSelectPath" @search="openSearchDirPath" />
      </template>
    </Form>
  </Modal>
</template>