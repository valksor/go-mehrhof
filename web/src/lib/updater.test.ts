import { describe, it, expect, afterEach, beforeEach, vi } from 'vitest'

describe('updater', () => {
  beforeEach(() => {
    vi.resetModules()
  })

  afterEach(() => {
    vi.restoreAllMocks()
    vi.unstubAllGlobals()
  })

  it('returns early when not in Tauri context', async () => {
    const mockCheck = vi.fn()
    vi.doMock('@tauri-apps/plugin-updater', () => ({
      check: mockCheck,
    }))

    const { checkForUpdates } = await import('./updater')
    await checkForUpdates()

    expect(mockCheck).not.toHaveBeenCalled()
  })

  it('calls check() in Tauri context', async () => {
    vi.stubGlobal('__TAURI__', {})
    const mockCheck = vi.fn().mockResolvedValue(null)
    vi.doMock('@tauri-apps/plugin-updater', () => ({
      check: mockCheck,
    }))

    const { checkForUpdates } = await import('./updater')
    await checkForUpdates()

    expect(mockCheck).toHaveBeenCalled()
  })

  it('sends notification and downloads when update is available', async () => {
    vi.stubGlobal('__TAURI__', {})

    const mockDownloadAndInstall = vi.fn().mockResolvedValue(undefined)
    const mockCheck = vi.fn().mockResolvedValue({
      version: '1.2.3',
      downloadAndInstall: mockDownloadAndInstall,
    })
    const mockSendNotification = vi.fn().mockResolvedValue(undefined)

    vi.doMock('@tauri-apps/plugin-updater', () => ({
      check: mockCheck,
    }))
    vi.doMock('./notify', () => ({
      sendNotification: mockSendNotification,
    }))

    const { checkForUpdates } = await import('./updater')
    await checkForUpdates()

    expect(mockCheck).toHaveBeenCalled()
    expect(mockSendNotification).toHaveBeenCalledWith(
      'Update Available',
      'kvelmo 1.2.3 is available. Restart to update.'
    )
    expect(mockDownloadAndInstall).toHaveBeenCalled()
  })

  it('does nothing when no update is available', async () => {
    vi.stubGlobal('__TAURI__', {})

    const mockCheck = vi.fn().mockResolvedValue(null)
    const mockSendNotification = vi.fn()

    vi.doMock('@tauri-apps/plugin-updater', () => ({
      check: mockCheck,
    }))
    vi.doMock('./notify', () => ({
      sendNotification: mockSendNotification,
    }))

    const { checkForUpdates } = await import('./updater')
    await checkForUpdates()

    expect(mockCheck).toHaveBeenCalled()
    expect(mockSendNotification).not.toHaveBeenCalled()
  })

  it('silently catches check() errors', async () => {
    vi.stubGlobal('__TAURI__', {})

    const mockCheck = vi.fn().mockRejectedValue(new Error('Network failure'))
    vi.doMock('@tauri-apps/plugin-updater', () => ({
      check: mockCheck,
    }))

    const { checkForUpdates } = await import('./updater')
    await expect(checkForUpdates()).resolves.toBeUndefined()
  })

  it('silently catches downloadAndInstall() errors', async () => {
    vi.stubGlobal('__TAURI__', {})

    const mockDownloadAndInstall = vi.fn().mockRejectedValue(new Error('Download failed'))
    const mockCheck = vi.fn().mockResolvedValue({
      version: '1.2.3',
      downloadAndInstall: mockDownloadAndInstall,
    })
    const mockSendNotification = vi.fn().mockResolvedValue(undefined)

    vi.doMock('@tauri-apps/plugin-updater', () => ({
      check: mockCheck,
    }))
    vi.doMock('./notify', () => ({
      sendNotification: mockSendNotification,
    }))

    const { checkForUpdates } = await import('./updater')
    await expect(checkForUpdates()).resolves.toBeUndefined()
  })
})
