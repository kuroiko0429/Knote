import { writable } from 'svelte/store'

export const toast = writable('')
let toastTimer: ReturnType<typeof setTimeout>
export function showToast(msg: string): void {
  toast.set(msg)
  clearTimeout(toastTimer)
  toastTimer = setTimeout(() => toast.set(''), 4000)
}

export const fontFamily = writable('')
export const fontSize = writable(0)
export const previewFontFamily = writable('')
export const previewFontSize = writable(0)
