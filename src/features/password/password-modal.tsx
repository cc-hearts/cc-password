import { Modal } from 'ant-design-vue'

import { Form, FormExpose } from '@/components/form'

import { addPassWord, changePasswordDescription } from '@/model/password'
import { searchPasswordCategory } from '@/model/password-category'
import { useCategory } from '@/storage/category'
import { GetPromiseReturns } from '@/types/utils'
import { encodeAes } from '@/utils/crypto'
import { errorMsg, successMsg } from '@/utils/message'
import type { getArrayChildItem } from '@cc-heart/utils/helper'
import { computed, defineComponent, nextTick, reactive, ref } from 'vue'
import { useI18n } from 'vue-i18n'

export default defineComponent({
  name: 'PasswordModal',
  props: {
    visible: {
      type: Boolean,
      default: false,
    },
    status: {
      type: String,
      default: 'add',
      validate: (val: string) => ['add', 'edit'].includes(val),
    },
    id: {
      type: Number,
      default: null,
    },
  },
  emits: ['cancel', 'refresh'],
  setup(props, { emit, expose }) {
    interface IModalRef {
      username: string
      cid: string | number
      url: string
      password: string
      title: string
      description: string
    }
    type IData = GetPromiseReturns<typeof searchPasswordCategory>

    const { t } = useI18n()
    const formRef = ref<FormExpose>()
    const { category } = useCategory<IData>()
    const compCategory = computed(() => category.value.slice(1))
    const formRules = reactive({
      username: [
        {
          required: true,
          message: t('pages-password.rulesRef.usernameRequireMessage'),
        },
      ],
      cid: [
        {
          required: true,
          message: t('pages-password.rulesRef.cidRequireMessage'),
        },
      ],
      password: [
        {
          required: true,
          message: t('pages-password.rulesRef.passwordRequireMessage'),
        },
      ],
    })

    const formColumn = computed(() => {
      return [
        {
          label: 'category',
          name: 'cid',
          type: 'select',
          extra: {
            options: compCategory.value.map(
              (item: getArrayChildItem<IData>) => {
                return { label: item.category, value: item.id }
              }
            ),
          },
        },
        {
          label: 'title',
          name: 'title',
          type: 'input',
        },
        { label: 'url', name: 'url', type: 'input' },
        { label: 'description', name: 'description', type: 'input' },
        { label: 'username', name: 'username', type: 'input' },
        {
          label: 'password',
          name: 'password',
          type: 'input',
          slot: { name: 'password' },
        },
      ]
    })

    const handleCancel = () => {
      formRef.value?.resetFields()
      emit('cancel')
    }
    const handleSubmit = async () => {
      const bool = await formRef.value?.validate()
      if (!bool) return
      const modalRef = formRef.value?.getFieldsValue<IModalRef>()
      if (!modalRef) return
      const password = await encodeAes(modalRef.password)
      if (!password) return
      const isAdd = props.status === 'add'
      if (isAdd) {
        await addPassWord({
          ...modalRef,
          password,
          cid: Number(modalRef.cid) || 0,
        })
      } else {
        if (!props.id) {
          errorMsg(t('add-password-modal.submitErrorMsg'))
          return
        }
        await changePasswordDescription(props.id, {
          ...modalRef,
          password,
          createdAt: undefined,
          updatedAt: undefined,
          cid: Number(modalRef.cid) || 0,
        })
      }
      successMsg(t('add-password-modal.successMsg'))
      emit('refresh')
      handleCancel()
    }

    const setFieldsValue = async (data: Partial<IModalRef>) => {
      await nextTick()
      formRef.value?.setFieldsValue(data)
    }
    expose({
      setFieldsValue,
    })
    return () => (
      <Modal
        open={props.visible}
        title={t(
          [
            'pages-password',
            props.id ? 'editPasswordModalTitle' : 'addPasswordModalTitle',
          ].join('.')
        )}
        onCancel={handleCancel}
        onOk={handleSubmit}
        okText={t('pages-password.submitButtonText')}
        cancelText={t('pages-password.cancelButtonText')}
      >
        <Form
          ref={(e) => (formRef.value = e as unknown as FormExpose)}
          columns={formColumn.value}
          rules={formRules}
        >
          {{
            password: ({ formState }: { formState: IModalRef }) => (
              <a-input-password
                value={formState.password}
                onChange={(e: any) => {
                  console.log(formState)
                  formState.password = e.target.value
                }}
                placeholder={t('pages-password.passwordPlaceholder')}
              />
            ),
          }}
        </Form>
      </Modal>
    )
  },
})
