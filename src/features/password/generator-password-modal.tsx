import { Button, Input, Modal } from 'ant-design-vue'
import { defineComponent, ref, watch } from 'vue'
import { generatorPassword } from '^/utils/crypto'
import { CopyOutlined, RedoOutlined } from '@ant-design/icons-vue'
import { useI18n } from 'vue-i18n'
import * as electron from 'electron'
import { successTips } from '@/utils/message'

const { clipboard } = electron
export default defineComponent({
  name: 'GeneratorPasswordModal',
  props: {
    visible: {
      type: Boolean,
      default: false,
    },
  },
  emits: ['cancel'],
  setup(props, { emit }) {
    const password = ref('')
    const { t } = useI18n()
    const handleRefreshGeneratorPassword = () => {
      password.value = generatorPassword()
    }
    const handleCancel = () => {
      emit('cancel')
    }

    watch(
      () => props.visible,
      (bool) => {
        if (bool) handleRefreshGeneratorPassword()
      },
      { immediate: true }
    )

    const handleCopyPassword = () => {
      clipboard.writeText(password.value)
      successTips(t('generaPasswordPage.copySuccessMsg'))
    }

    return () => (
      <Modal
        open={props.visible}
        onCancel={handleCancel}
        title={t('generaPasswordPage.modalTitle')}
      >
        {{
          footer: () => (
            <Button onClick={handleCancel}>
              {t('generaPasswordPage.cancelButtonText')}
            </Button>
          ),
          default: () => (
            <Input value={password.value} disabled>
              {{
                addonBefore: () => (
                  <RedoOutlined
                    class="cursor-pointer"
                    onClick={handleRefreshGeneratorPassword}
                  />
                ),
                addonAfter: () => (
                  <CopyOutlined
                    class="cursor-pointer"
                    onClick={handleCopyPassword}
                  />
                ),
              }}
            </Input>
          ),
        }}
      </Modal>
    )
  },
})
