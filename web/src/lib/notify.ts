/**
 * Send a notification via Tauri or the browser Notification API.
 */
export async function sendNotification(title: string, body: string) {
  try {
    if ('__TAURI__' in window) {
      const { sendNotification: tauriNotify } = await import('@tauri-apps/plugin-notification')
      tauriNotify({ title, body })
      return
    }

    if ('Notification' in window && Notification.permission === 'granted') {
      new Notification(title, { body })
    }
  } catch {
    // Silently ignore
  }
}

export async function requestNotificationPermission() {
  if ('__TAURI__' in window) return
  if ('Notification' in window && Notification.permission === 'default') {
    await Notification.requestPermission()
  }
}
