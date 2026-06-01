import { Message } from '@arco-design/web-vue'

export function useToast() {
  return {
    success(msg) { Message.success(msg) },
    error(msg) { Message.error(msg) },
    warning(msg) { Message.warning(msg) },
    info(msg) { Message.info(msg) },
  }
}