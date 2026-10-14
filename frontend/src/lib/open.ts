/** Opens a URL in the user's real browser instead of navigating the webview. */
export function openExternal(url: string): void {
  const normalized = url.includes('://') ? url : `https://${url}`
  const runtime = window.runtime as { BrowserOpenURL?: (url: string) => void } | undefined
  if (runtime?.BrowserOpenURL) {
    runtime.BrowserOpenURL(normalized)
    return
  }
  window.open(normalized, '_blank', 'noopener,noreferrer')
}
