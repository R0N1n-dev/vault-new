import { ref } from 'vue'
import { friendlyError } from '../lib/backend'

export type ToastKind = 'success' | 'error' | 'info'

export interface Toast {
  id: number
  kind: ToastKind
  title: string
  detail?: string
  timeoutMs: number
}

export const toasts = ref<Toast[]>([])

let sequence = 0

export function pushToast(input: {
  kind: ToastKind
  title: string
  detail?: string
  timeoutMs?: number
}): number {
  sequence += 1
  const id = sequence
  const toast: Toast = {
    id,
    kind: input.kind,
    title: input.title,
    detail: input.detail,
    timeoutMs: input.timeoutMs ?? (input.kind === 'error' ? 7000 : 3800),
  }
  // Keep the stack short: the newest message is the useful one.
  toasts.value = [...toasts.value.slice(-3), toast]
  window.setTimeout(() => dismissToast(id), toast.timeoutMs)
  return id
}

export function dismissToast(id: number): void {
  toasts.value = toasts.value.filter((toast) => toast.id !== id)
}

export function toastSuccess(title: string, detail?: string): void {
  pushToast({ kind: 'success', title, detail })
}

export function toastInfo(title: string, detail?: string): void {
  pushToast({ kind: 'info', title, detail })
}

/** toastError turns any thrown value into a readable, friendly message. */
export function toastError(error: unknown, title = 'Something went wrong'): void {
  pushToast({ kind: 'error', title, detail: friendlyError(error) })
}
