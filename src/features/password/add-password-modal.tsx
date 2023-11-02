import {
  Modal,
  Form,
  FormItem,
  Input,
  Select,
  InputPassword,
} from 'ant-design-vue'
import { addPassWord } from '@/model/password'
import { searchPasswordCategory } from '@/model/password-category'
import { changePasswordDescription } from '@/model/password'
import { useCategory } from '@/storage/category'
import { GetPromiseReturns } from '@/types/utils'
import { errorMsg, successMsg } from '@/utils/message'
import { computed, defineComponent, reactive } from 'vue'
import { useI18n } from 'vue-i18n'
import { encodeAes } from '@/utils/crypto'
import type { getArraySubitem } from '@cc-heart/utils/helper'
import type { SelectValue } from 'ant-design-vue/es/select'
import type { ChangeEvent } from 'ant-design-vue/es/_util/EventInterface'

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
    const modalRef = reactive({
      username: '',
      cid: '' as string | number,
      url: '',
      password: '',
      title: '',
      description: '',
    })
    type IData = GetPromiseReturns<typeof searchPasswordCategory>
    type IDataItem = getArraySubitem<IData>
    const { t } = useI18n()
    const { category } = useCategory<IData>()
    const compCategory = computed(() => category.value.slice(1))
    const rulesRef = reactive({
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
    const { validate, resetFields } = Form.useForm(modalRef, rulesRef)
    const handleCancel = () => {
      resetFields()
      emit('cancel')
    }
    const handleSubmit = async () => {
      await validate()
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
          cid: Number(modalRef.cid) || 0,
        })
      }
      successMsg(t('add-password-modal.successMsg'))
      emit('refresh')
      handleCancel()
    }
    const setFieldsValue = (data: Partial<typeof modalRef>) => {
      Object.assign(modalRef, data)
    }
    expose({
      setFieldsValue,
    })
    return () => (
      <Modal
        visible={props.visible}
        title={t('pages-password.addPasswordModalTitle')}
        onCancel={handleCancel}
        onOk={handleSubmit}
        okText={t('pages-password.submitButtonText')}
        cancelText={t('pages-password.cancelButtonText')}
      >
        <Form model={modalRef} rules={rulesRef} labelCol={{ span: 5 }}>
          <FormItem label="category" required name="cid">
            <Select
              value={modalRef.cid}
              onChange={(val: SelectValue) => {
                modalRef.cid = val as string
              }}
            >
              {compCategory.value.map((item: IDataItem) => {
                return (
                  <Select.Option value={item.id}>{item.category}</Select.Option>
                )
              })}
            </Select>
          </FormItem>
          <FormItem label="title" required name="title">
            <Input
              value={modalRef.title}
              onChange={(e: ChangeEvent) => (modalRef.title = e.target.value!)}
            />
          </FormItem>
          <FormItem label="url" name="url">
            <Input
              value={modalRef.url}
              onChange={(e: ChangeEvent) => (modalRef.url = e.target.value!)}
            />
          </FormItem>
          <FormItem label="description" name="description">
            <Input
              value={modalRef.description}
              onChange={(e: ChangeEvent) =>
                (modalRef.description = e.target.value!)
              }
            />
          </FormItem>
          <FormItem label="username" required name="username">
            <Input
              value={modalRef.username}
              onChange={(e: ChangeEvent) =>
                (modalRef.username = e.target.value!)
              }
            />
          </FormItem>
          <FormItem label="password" required name="password">
            <InputPassword
              value={modalRef.password}
              onChange={(e: ChangeEvent) =>
                (modalRef.password = e.target.value!)
              }
            />
          </FormItem>
        </Form>
      </Modal>
    )
  },
})
