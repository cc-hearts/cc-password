<script setup lang="ts">
import Headers from '@/components/header/headers.vue'
import IPopover from '@/components/header/i-popover'
import CategorySide from '@/features/password-category/side.tsx'
import GeneratorPasswordModal from '@/features/password/generator-password-modal'
import { useCssNamespace } from '@/hooks'
import LogOut from '@/icons/log-out.vue'
import { searchAllPassword } from '@/model/password'
import { clearRefreshToken, clearToken, useProfile } from '@/storage'
import { decodeAes } from '@/utils/crypto'
import {
MoreOutlined,
ToTopOutlined,
ToolOutlined,
} from '@ant-design/icons-vue'
import { Modal, Popover } from 'ant-design-vue'
import { reactive, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { useRouter } from 'vue-router'
import ExportDataModal from './export-data-modal.vue'
import { ExportDataForm } from '@/features/password/types'
import * as path from 'path'
import * as fs from 'fs'
import { successTips } from '@/utils/message'

// @ts-ignore
const { join } = path.default
// @ts-ignore
const { writeFileSync } = fs.default

const { t } = useI18n()
const router = useRouter()
const exportDataModalRef = ref()
const ns = useCssNamespace('header')
const generaPasswordProps = reactive({
  visible: false,
  onCancel: () => {
    generaPasswordProps.visible = false
  },
})

const openPasswordModal = () => {
  generaPasswordProps.visible = true
}
const openExportDataModal = () => {
  exportDataModalRef.value?.onOpen()
}
const { profile } = useProfile()
const handleExportData = async (formData: ExportDataForm) => {
  const passwordList = await searchAllPassword()
  const exportDataPendingList =  passwordList.map(async passwordInfo => {
    const { cid, uid, id, ...restPasswordInfo } = passwordInfo
    return {
      ...restPasswordInfo,
      password: await decodeAes(passwordInfo.password),
    }
  })
  const exportDataList = await Promise.all(exportDataPendingList)
  const filePath = join(formData.exportSelectPath, formData.exportName)
  writeFileSync(filePath, JSON.stringify({
    username: profile.value?.username,
    data: exportDataList
  }))
  localStorage.setItem('exportDataPath', formData.exportSelectPath)
  successTips(t('exportDataModal.exportSuccess'))
  exportDataModalRef.value?.onCancel()
}

const handleLogout = () => {
  Modal.confirm({
    title: t('headers.title'),
    content: t('headers.content'),
    okText: t('headers.okText'),
    cancelText: t('headers.cancelText'),
    onOk() {
      clearToken()
      clearRefreshToken()
      router.push('/login')
    },
  })
}
</script>
<template>
  <div class="h-full w-full flex">
    <CategorySide />
    <div class="flex-1">
      <Headers>
        <template #right-icon>
          <IPopover :content="t('headers.generateText')">
            <ToolOutlined @click="openPasswordModal" />
          </IPopover>
        </template>
        <template #right>
          <Popover :overlayClassName="ns.e('custom-popover')" placement="bottomRight">
            <template #content>
              <div class="flex items-center" :class="[ns.e('popover')]" @click="openExportDataModal">
                <ToTopOutlined />
                <div class="popover__split"></div>
                <span class="flex-1 text-center">{{ $t('export') }}</span>
              </div>
              <div class="flex items-center" :class="[ns.e('popover')]" @click="handleLogout">
                <LogOut />
                <div class="popover__split"></div>
                <span class="flex-1 text-center">{{ $t('logout') }}</span>
              </div>
            </template>
            <MoreOutlined />
          </Popover>
        </template>
      </Headers>
      <main class="text-gray-700 dark:text-gray-200 flex-1 flex flex-col min-h-0">
        <router-view />
      </main>
    </div>
  </div>
  <ExportDataModal ref="exportDataModalRef" @ok="handleExportData" />
  <GeneratorPasswordModal :visible="generaPasswordProps.visible" @cancel="generaPasswordProps.onCancel" />
</template>
<style lang="scss"></style>
