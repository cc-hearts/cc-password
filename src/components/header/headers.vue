<script setup lang="ts">
import { useNamespace } from '@/hooks'
import { GithubIcon } from '@/icons'
import { githubUrl } from '@/configs'
import SwitchTheme from './switchTheme.vue'
import { useOpenLink } from '@/hooks/useOpenLink'
import TagsIcon from '@/icons/Tags.vue'
import IPopover from './IPopover'
import LogOut from '@/icons/LogOut.vue'
import { Modal, Popover } from 'ant-design-vue'
import { MoreOutlined } from '@ant-design/icons-vue'
import { clearRefreshToken, clearToken } from '@/storage'
import { useRouter } from 'vue-router'

const router = useRouter()
const ns = useNamespace('header')
const toGithub = () => {
  if (githubUrl) useOpenLink(githubUrl)
}
const handleLogout = () => {
  Modal.confirm({
    title: 'Are you sure to log out?',
    content: 'You will be logged out and redirected to the login page.',
    okText: 'Yes',
    cancelText: 'No',
    onOk() {
      clearToken()
      clearRefreshToken()
      router.push('/login')
    },
  })
}
</script>

<template>
  <header
    class="flex justify-between items-center px-3 shrink-0"
    :class="[ns.cls]"
  >
    <slot name="left">
      <div></div>
    </slot>
    <div class="flex text-xl items-center" :class="[ns.e('icon')]">
      <slot name="right-icon"></slot>
      <IPopover content="password generator">
        <TagsIcon />
      </IPopover>
      <IPopover content="github">
        <GithubIcon @click="toGithub" />
      </IPopover>
      <div :class="[ns.e('split')]">
        <SwitchTheme />
      </div>
      <Popover :overlayClassName="ns.e('custom-popover')" placement="bottomRight">
        <template #content>
          <div
            class="flex items-center"
            :class="[ns.e('popover')]"
            @click="handleLogout"
          >
            <LogOut />
            <div class="popover__split"></div>
            <span>{{ $t('logout') }}</span>
          </div>
        </template>
        <MoreOutlined />
      </Popover>
    </div>
  </header>
</template>

<style lang="scss">
@use '@/assets/scss/var/variable.scss' as *;
@use '@/assets/scss/common/mixins.scss' as *;

@include b('header') {
  --header-shadow: rgb(229, 230, 235);
  --icon-background-color: rgba(0, 0, 0, 0.1);
  --cc-divider-light: rgba(60, 60, 60, 0.12);
  height: 48px;
  box-shadow: 0 1px 0 var(--header-shadow);

  &__icon {
    color: var(--color-text-2);

    & > div,
    & > button,
    & > span > svg {
      margin: 0 0.2rem;
    }

    & > span {
      box-sizing: content-box;
      padding: 0.3rem;
      transition: all 0.3s;
      border-radius: 4px;
      &:hover {
        background-color: var(--icon-background-color);
      }
    }

    svg {
      cursor: pointer;
      transition: color 0.3s;

      &:hover {
        color: var(--color-text-1);
      }
    }
  }

  &__split {
    $marginX: 6px;
    display: flex;
    &::before,
    &::after {
      content: '';
      height: 24px;
    }

    &::before {
      margin-right: $marginX;
      border-right: 1px solid var(--cc-divider-light);
    }

    &::after {
      margin-left: $marginX;
      border-left: 1px solid var(--cc-divider-light);
    }
  }

  @include e('popover') {
    --cc-popover-hover-bg-color: rgba(0,0,0,0.06);
    padding: 2px 4px;
    transition: all 0.3s;
    border-radius: 4px;
    cursor: pointer;
    &:hover {
      background-color: var(--cc-popover-hover-bg-color);
    }
  }

  @include e('custom-popover') {
    .ant-popover-inner {
      padding: 12px 6px;
    }
  }
}

.popover__split {
  --cc-divider-light: rgba(60, 60, 60, 0.12);
  width: 1px;
  height: 16px;
  margin: 0 4px;
  border-right: 1px solid var(--cc-divider-light);
}

.dark {
  @include b('header') {
    --header-shadow: rgb(72, 72, 73);
    --icon-background-color: rgba(255, 255, 255, 0.12);
    --cc-divider-light: rgba(84, 84, 84, 0.48);
  }

  .popover__split {
    --cc-divider-light: rgba(84, 84, 84, 0.48);
  }
}
</style>
