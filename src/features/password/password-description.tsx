import { useCssNamespace } from '@/hooks/use-css-namespace'
import { useOpenLink } from '@/hooks/use-open-link'
import ViewOff from '@/icons/view-off.vue'
import ViewIcon from '@/icons/view.vue'
import { findPasswordDetail, searchPassword } from '@/model/password'
import { useDescription } from '@/storage/description'
import { GetPromiseReturns } from '@/types/utils'
import { classnames } from '@/utils/classnames'
import { decodeAes } from '@/utils/crypto'
import { EditOutlined } from '@ant-design/icons-vue'
import { fn } from '@cc-heart/utils/helper'
import { Button, Input, Popover, Spin, message } from 'ant-design-vue'
import type { MenuInfo } from 'ant-design-vue/es/menu/src/interface'
import * as electron from 'electron'
import { defineComponent, reactive, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import PasswordModal from './password-modal'
const { clipboard } = electron

export default defineComponent({
  name: 'passwordDescription',
  setup() {
    const columns = [
      { label: 'username', key: 'username' },
      { label: 'password', key: 'password' },
    ] as const

    const otherColumns = [
      { label: 'website', key: 'url' },
      { label: 'description', key: 'description' },
    ] as const

    const ns = useCssNamespace('password')
    const passwordModelProps = reactive({
      visible: false,
    })
    const passwordModalRef = ref<{ setFieldsValue: fn } | null>(null)
    const reviewPasswordStatus = ref(false)

    const { activeDescription } = useDescription()

    const description = reactive({
      data: {} as GetPromiseReturns<typeof findPasswordDetail>,
      password: '',
      loading: false,
    })

    const toggleReviewPasswordStatus = () => {
      reviewPasswordStatus.value = !reviewPasswordStatus.value
    }
    async function getData() {
      handleRemovePassword()
      const id = Number(activeDescription.value)
      if (id) {
        description.loading = true
        try {
          const data = await findPasswordDetail(id)
          if (data) {
            description.data = data
            handleSearchPassword(description.data.id!)
            Reflect.set(
              data,
              'createdAt',
              new Date(data.createdAt).toLocaleString()
            )
            Reflect.set(
              data,
              'updatedAt',
              new Date(data.updatedAt).toLocaleString()
            )
          }
        } catch (e) {
          console.log('[passwordDescription] getData error', e)
        } finally {
          description.loading = false
        }
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
      reviewPasswordStatus.value = false
    }

    watch(
      () => activeDescription.value,
      () => {
        getData()
      }
    )
    const { t } = useI18n()

    const handleCopyPassword = async (key: string) => {
      if (key === 'password') {
        const pwd = description.password
        if (!pwd) {
          await handleSearchPassword(description.data!.id)
        }
        clipboard.writeText(description.password)
        message.success(t('passwordDescriptionPage.copySuccessText'))
        description.password = pwd
      }
    }

    const handleEditPasswordDescription = () => {
      passwordModelProps.visible = true
      passwordModalRef &&
        passwordModalRef.value?.setFieldsValue({
          ...description.data,
          password: description.password,
        })
    }
    const handleCancelPasswordModal = () => {
      passwordModelProps.visible = false
    }

    const handleMenuClick = ({ key }: MenuInfo) => {
      switch (key) {
        case 'toggle':
          toggleReviewPasswordStatus()
          if (reviewPasswordStatus.value)
            handleSearchPassword(description.data!.id)
          break
        case 'copy':
          handleCopyPassword('password')
          break
      }
    }

    const formatDescription = (field: string, key: string) => {
      if (key === 'password')
        return (
          <div
            class={classnames(['w-full', 'h-full', ns.e('hover')])}
            onClick={() => handleMenuClick({ key: 'toggle' } as MenuInfo)}
          >
            <Input
              style={{ padding: '4px 0' }}
              type={reviewPasswordStatus.value ? 'text' : 'password'}
              bordered={false}
              onFocus={(e) =>
                (e.target as HTMLInputElement | undefined)?.blur?.()
              }
              value={description.password}
            />
            {reviewPasswordStatus.value ? <ViewOff /> : <ViewIcon />}
          </div>
        )
      return field
    }

    const formatOtherDescription = (description: string, key: string) => {
      if (key === 'url')
        return <a onClick={() => useOpenLink(description)}>{description}</a>
      return description
    }
    return () => {
      if (!activeDescription.value) return null
      return (
        <div>
          <Spin spinning={description.loading}>
            <div class="flex m-t-4 justify-between">
              <div>
                <Button
                  icon={<EditOutlined />}
                  type="link"
                  onClick={handleEditPasswordDescription}
                >
                  {t('passwordDescriptionPage.edit')}
                </Button>
              </div>
            </div>
            <div class="m-6 leading-5">
              <h3>{Reflect.get(description.data!, 'title')}</h3>
              <div class={'border-ins border-solid border-1px rounded m-y-6'}>
                {columns.map((column, index) => {
                  const key = column.key as (typeof columns)[number]['key']
                  const isPasswordField = key === 'password'
                  const isLastIndex = index === columns.length - 1
                  return (
                    <div
                      key={key}
                      class={classnames(
                        isLastIndex
                          ? ['border-b-1px', 'border-b-solid', 'border-b-ins']
                          : '',
                        'p-3'
                      )}
                    >
                      <div
                        class={classnames(
                          isPasswordField
                            ? ['flex', 'items-center', 'justify-between']
                            : ''
                        )}
                      >
                        <span>{column.label}</span>
                      </div>
                      <div>
                        {isPasswordField ? (
                          <Popover trigger="hover">
                            {{
                              content: () => (
                                <Button
                                  type="link"
                                  onClick={() => handleCopyPassword(key)}
                                >
                                  {t('passwordDescriptionPage.copy')}
                                </Button>
                              ),
                              default: () =>
                                formatDescription(
                                  Reflect.get(description.data!, key),
                                  key
                                ),
                            }}
                          </Popover>
                        ) : (
                          formatDescription(description.data![key], key)
                        )}
                      </div>
                    </div>
                  )
                })}
              </div>
            </div>
            <div class="m-6 leading-5">
              {otherColumns.map((column) => {
                const key = column.key as (typeof columns)[number]['key']
                return (
                  <div key={key} class="m-y-3">
                    <div>{column.label}</div>
                    <div>
                      {formatOtherDescription(
                        Reflect.get(description.data!, key),
                        key
                      )}
                    </div>
                  </div>
                )
              })}
            </div>
            <div class="m-t-20 flex text-sm	text-slate-400 flex-col items-center justify-center p-b-4">
              {description.data?.createdAt && (
                <div>
                  {t('passwordDescriptionPage.created')}
                  {description.data.createdAt}
                </div>
              )}
              {description.data?.updatedAt && (
                <div>
                  {t('passwordDescriptionPage.modified')}
                  {description.data.updatedAt}
                </div>
              )}
            </div>
          </Spin>
          <PasswordModal
            ref={passwordModalRef}
            id={description.data!.id}
            status="edit"
            onRefresh={getData}
            onCancel={handleCancelPasswordModal}
            visible={passwordModelProps.visible}
          />
        </div>
      )
    }
  },
})
