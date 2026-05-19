/**
 * Check for updates on app launch (non-blocking).
 * Only runs in Tauri context.
 */
export async function checkForUpdates() {
  try {
    if (!('__TAURI__' in window)) return

    const { check } = await import('@tauri-apps/plugin-updater')
    const update = await check()

    if (update) {
      const { sendNotification } = await import('./notify')
      await sendNotification('Update Available', `kvelmo ${update.version} is available. Restart to update.`)

      // Download and install (will apply on next restart)
      await update.downloadAndInstall()
    }
  } catch {
    // Silently ignore update check failures
  }
}
