import { addSecurity } from '@/model/security'
import { useProfile } from '@/storage/user'
import { generatorAesSign } from '@/utils/crypto'
import {
  Button,
  Form,
  FormItem,
  InputPassword,
  Tooltip,
  TypographyTitle,
} from 'ant-design-vue'
import { defineComponent, reactive } from 'vue'
import { useRouter } from 'vue-router'

export default defineComponent({
  name: 'GeneratorSecurityPage',
  setup() {
    const modalRef = reactive({
      security: '',
      iv: '',
    })
    const { profile } = useProfile()
    const router = useRouter()
    const rulesRef = reactive({
      security: [
        {
          required: true,
          message: '请生成密钥',
        },
      ],
      iv: [
        {
          required: true,
          message: '请生成iv',
        },
      ],
    })
    const { validate } = Form.useForm(modalRef, rulesRef)
    const generatorSecurity = () => {
      const { key, iv } = generatorAesSign()
      modalRef.security = key.toString()
      modalRef.iv = iv.toString()
    }

    const handleSubmit = async () => {
      await validate()
      await addSecurity({ ...modalRef, uid: profile.value!.uid })
      router.push('/password')
    }
    return () => (
      <div class="h-full w-full flex flex-col justify-center items-center">
        <div class="translate-x-20%">
          <TypographyTitle level={4}>初次使用，需要生成密钥。</TypographyTitle>
        </div>
        <Form class="w-sm m-t-3" modal={modalRef} labelCol={{ span: 6 }}>
          <FormItem label="security" name="security" required>
            <Tooltip>
              {{
                title: () => modalRef.security,
                default: () => <InputPassword value={modalRef.security} />,
              }}
            </Tooltip>
          </FormItem>
          <FormItem label="iv" name="iv" required>
            <Tooltip>
              {{
                title: () => modalRef.iv,
                default: () => <InputPassword value={modalRef.iv} />,
              }}
            </Tooltip>
          </FormItem>
          <div class="text-center translate-x-10%">
            <Button class="m-r-2 " onClick={generatorSecurity}>
              生成密钥
            </Button>
            <Button onClick={handleSubmit}>开始使用</Button>
          </div>
        </Form>
      </div>
    )
  },
})
