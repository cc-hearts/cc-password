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
import { useProfile } from '@/storage/user'
import { IEvent } from '@/types/common'
import { GetPromiseReturns } from '@/types/utils'
import { successMsg } from '@/utils/message'
import { computed, defineComponent, reactive } from 'vue'
import { useI18n } from 'vue-i18n'
import { encodeAes } from '@/utils/crypto'

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
    const { t } = useI18n()
    const { profile } = useProfile()
    const { category } =
      useCategory<GetPromiseReturns<typeof searchPasswordCategory>>()
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
      const password = encodeAes(modalRef.password)
      if (!password) return
      await addPassWord({
        ...modalRef,
        password,
        cid: Number(modalRef.cid) || 0,
        uid: profile.value!.uid,
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
              {compCategory.value.map((item) => {
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
