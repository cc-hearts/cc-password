import { notification } from 'ant-design-vue'

type IconType = 'success' | 'info' | 'warning' | 'error'

function message(type: IconType) {
  return (msg: string) => {
    notification[type]({
      message: msg,
    })
  }
}
export function errorMsg(msg = '') {
  message('error')(msg)
}

export function successMsg(msg = '') {
  message('success')(msg)
}
