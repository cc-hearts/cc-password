import {
  Modal,
  Form,
  FormItem,
  Input,
  Select,
  InputPassword,
} from 'ant-design-vue'
import { addPassWord } from '@/model/password'
import { searchPasswordCategory } from '@/model/passwordCategory'
import { useCategory } from '@/storage/category'
import { IEvent } from '@/types/common'
import { GetPromiseReturns } from '@/types/utils'
import { successMsg } from '@/utils/message'
import { computed, defineComponent, reactive } from 'vue'
import { useI18n } from 'vue-i18n'
import { encodeAes } from '@/utils/crypto'
import type { getArraySubitem } from '@cc-heart/utils/helper'

export default defineComponent({
  name: 'AddPasswordModal',
  props: {
    visible: {
      type: Boolean,
      default: false,
    },
  },
  emits: ['cancel', 'refresh'],
  setup(props, { emit }) {
    const modalRef = reactive({
      username: '',
      cid: '' as string | number,
      url: '',
      password: '',
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
      await addPassWord({
        ...modalRef,
        password,
        cid: Number(modalRef.cid) || 0,
      })
      successMsg('添加成功')
      emit('refresh')
      handleCancel()
    }
    return () => (
      <Modal
        visible={props.visible}
        title={t('pages-password.addPasswordModalTitle')}
        onCancel={handleCancel}
        onOk={handleSubmit}
      >
        <Form model={modalRef} rules={rulesRef} labelCol={{ span: 5 }}>
          <FormItem label="category" required name="cid">
            <Select
              value={modalRef.cid}
              onChange={(val: number) => (modalRef.cid = val)}
            >
              {compCategory.value.map((item: IDataItem) => {
                return (
                  <Select.Option value={item.id}>{item.category}</Select.Option>
                )
              })}
            </Select>
          </FormItem>
          <FormItem label="url" name="url">
            <Input
              value={modalRef.url}
              onChange={(e: IEvent<HTMLInputElement>) =>
                (modalRef.url = e.target.value)
              }
            />
          </FormItem>
          <FormItem label="description" name="description">
            <Input
              value={modalRef.description}
              onChange={(e: IEvent<HTMLInputElement>) =>
                (modalRef.description = e.target.value)
              }
            />
          </FormItem>
          <FormItem label="username" required name="username">
            <Input
              value={modalRef.username}
              onChange={(e: IEvent<HTMLInputElement>) =>
                (modalRef.username = e.target.value)
              }
            />
          </FormItem>
          <FormItem label="password" required name="password">
            <InputPassword
              value={modalRef.password}
              onChange={(e: IEvent<HTMLInputElement>) =>
                (modalRef.password = e.target.value)
              }
            />
          </FormItem>
        </Form>
      </Modal>
    )
  },
})
