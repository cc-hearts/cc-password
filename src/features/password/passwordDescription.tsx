import {
  changePasswordDescription,
  findPasswordDetail,
  searchPassword,
} from '@/model/password'
import { useDescription } from '@/storage/description'
import { GetPromiseReturns } from '@/types/utils'
import { Descriptions, DescriptionsItem, Input, message } from 'ant-design-vue'
import { defineComponent, reactive, watch } from 'vue'
import ViewIcon from '@/icons/view.vue'
import ViewCloseIcon from '@/icons/viewClose.vue'
import { decodeAes } from '@/utils/crypto'
import * as electron from 'electron'
import { EditIcon } from '@/icons'
import { IEvent } from '@/types/common'

const { clipboard } = electron

export default defineComponent({
  name: 'passwordDescription',
  setup() {
    const columns = [
      { label: 'title', key: 'title' },
      { label: 'url', key: 'url' },
      { label: 'description', key: 'description' },
      { label: 'username', key: 'username' },
      { label: 'password', key: 'password' },
    ] as const

    const requiredField = ['username', 'password']
    const passwordLabel = '******************'
    const { activeDescription } = useDescription()
    const description = reactive({
      data: {} as GetPromiseReturns<typeof findPasswordDetail>,
      password: '',
      editKey: null as null | string,
      editValue: '',
    })
    async function getData() {
      handleRemovePassword()
      const id = Number(activeDescription.value)
      if (id) {
        const data = await findPasswordDetail(id)
        if (data) description.data = data
      }
    }

    async function handleSearchPassword(id: number) {
      const result = await searchPassword(id)
      if (result && result.password) {
        const code = await decodeAes(result.password)
        if (code) description.password = code
      }
    }
    function handleRemovePassword() {
      description.password = ''
    }
    watch(
      () => activeDescription.value,
      () => {
        getData()
      }
    )

    const handleCopyPassword = async (key: string) => {
      if (key === 'password') {
        const pwd = description.password
        if (!pwd) {
          await handleSearchPassword(description.data!.id)
        }
        clipboard.writeText(description.password)
        message.success('🎉 copy password to clipboard success')
        description.password = pwd
      }
    }

    const handleEditPasswordDescription = (field: string) => {
      description.editKey = field
      description.editValue = Reflect.get(description.data!, field)
    }
    const handleEditBlur = () => {
      description.editKey = null
      description.editValue = ''
    }

    const handleKeyDown = (e: KeyboardEvent) => {
      if (e.code === 'Escape') {
        handleEditBlur
      } else if (e.code === 'Enter') {
        handleEditDescription()
        handleEditBlur()
      }
    }

    const handleEditDescription = async () => {
      if (!description.editKey) return
      const isRequired = requiredField.includes(description.editKey)
      if (isRequired) {
        if (!description.editValue.trim()) {
          message.error('required field')
          return
        }
      }
      const changeField = { [description.editKey]: description.editValue }
      await changePasswordDescription(
        Number(activeDescription.value),
        changeField
      )
      message.success('🎉 change success')
      getData()
    }

    return () => (
      <div class="min-w-0 p-x-2 overflow-hidden">
        <Descriptions column={1}>
          {activeDescription.value &&
            description.data?.id &&
            columns.map((column) => {
              const key = column.key as (typeof columns)[number]['key']
              const isPasswordField = key === 'password'
              return (
                <DescriptionsItem
                  id={column.key}
                  label={column.label}
                  labelStyle={{
                    width: '100px',
                    'justify-content': 'end',
                    'user-select': 'none',
                  }}
                >
                  {description.editKey !== column.key ? (
                    <div
                      class={
                        'overflow-hidden relative whitespace-nowrap text-ellipsis flex-1 p-r-12 description-item' +
                        (isPasswordField ? ' cursor-pointer' : '')
                      }
                    >
                      <span onClick={() => handleCopyPassword(key)} class="relative">
                        {isPasswordField
                          ? description.password || passwordLabel
                          : description.data![key]}
                      </span>
                      <EditIcon
                        class={'description-item__field m-l-1 align-middle'}
                        onClick={() => {
                          handleEditPasswordDescription(column.key)
                        }}
                      />
                      {isPasswordField && (
                        <span class="absolute right-0 select-none">
                          {description.password ? (
                            <span onClick={handleRemovePassword}>
                              <ViewCloseIcon />
                            </span>
                          ) : (
                            <span
                              onClick={() =>
                                handleSearchPassword(description.data!.id)
                              }
                            >
                              <ViewIcon />
                            </span>
                          )}
                        </span>
                      )}
                    </div>
                  ) : (
                    <div onKeydown={handleKeyDown}>
                      <Input
                        onBlur={() => {
                          handleEditBlur()
                        }}
                        value={description.editValue}
                        onChange={(e: IEvent<HTMLInputElement>) => description.editValue = e.target.value}
                        style={{ width: '80%' }}
                        suffix={
                          <span class="keyboard-bg p-x-1 rounded">Esc</span>
                        }
                      />
                    </div>
                  )}
                </DescriptionsItem>
              )
            })}
        </Descriptions>
      </div>
    )
  },
})
