import { Message } from '@arco-design/web-vue'

export function useToast() {
  return {
    success(msg: string) { Message.success(msg) },
    error(msg: string) { Message.error(msg) },
    warning(msg: string) { Message.warning(msg) },
    info(msg: string) { Message.info(msg) },
  }
}
