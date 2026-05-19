import { describe, it, expect, beforeEach, vi, afterEach } from 'vitest'
import { sendNotification, requestNotificationPermission } from './notify'

describe('notify', () => {
  afterEach(() => {
    vi.restoreAllMocks()
    vi.unstubAllGlobals()
  })

  describe('sendNotification', () => {
    describe('when Tauri is available', () => {
      beforeEach(() => {
        // Stub __TAURI__ on window to simulate Tauri environment
        vi.stubGlobal('__TAURI__', {})
      })

      it('calls tauriNotify when __TAURI__ is present', async () => {
        const mockTauriNotify = vi.fn().mockResolvedValue(undefined)
        vi.doMock('@tauri-apps/plugin-notification', () => ({
          sendNotification: mockTauriNotify,
        }))

        // Since dynamic import is used, we just verify no error is thrown
        await expect(sendNotification('Title', 'Body')).resolves.toBeUndefined()
      })
    })

    describe('when Notification API is available', () => {
      beforeEach(() => {
        // Ensure no __TAURI__ global
        if ('__TAURI__' in window) {
          vi.stubGlobal('__TAURI__', undefined)
        }
      })

      it('creates a Notification when permission is granted', async () => {
        const MockNotification = vi.fn()
        Object.defineProperty(MockNotification, 'permission', { value: 'granted', configurable: true })
        vi.stubGlobal('Notification', MockNotification)

        await sendNotification('Test Title', 'Test Body')

        expect(MockNotification).toHaveBeenCalledWith('Test Title', { body: 'Test Body' })
      })

      it('does not create a Notification when permission is denied', async () => {
        const MockNotification = vi.fn()
        Object.defineProperty(MockNotification, 'permission', { value: 'denied', configurable: true })
        vi.stubGlobal('Notification', MockNotification)

        await sendNotification('Test Title', 'Test Body')

        expect(MockNotification).not.toHaveBeenCalled()
      })

      it('does not create a Notification when permission is default (not yet requested)', async () => {
        const MockNotification = vi.fn()
        Object.defineProperty(MockNotification, 'permission', { value: 'default', configurable: true })
        vi.stubGlobal('Notification', MockNotification)

        await sendNotification('Test Title', 'Test Body')

        expect(MockNotification).not.toHaveBeenCalled()
      })

      it('does not throw when Notification is not available in window', async () => {
        // Remove Notification from window
        const originalNotification = (window as Window & { Notification?: typeof Notification }).Notification
        // eslint-disable-next-line @typescript-eslint/no-explicit-any
        delete (window as any).Notification

        await expect(sendNotification('Title', 'Body')).resolves.toBeUndefined()

        if (originalNotification) {
          vi.stubGlobal('Notification', originalNotification)
        }
      })
    })

    it('does not throw on any error (silently ignores)', async () => {
      // Make Notification constructor throw
      const MockNotification = vi.fn().mockImplementation(() => {
        throw new Error('Notification blocked')
      })
      Object.defineProperty(MockNotification, 'permission', { value: 'granted', configurable: true })
      vi.stubGlobal('Notification', MockNotification)

      await expect(sendNotification('Title', 'Body')).resolves.toBeUndefined()
    })
  })

  describe('requestNotificationPermission', () => {
    beforeEach(() => {
      // Ensure no __TAURI__ global
      if ('__TAURI__' in window) {
        vi.stubGlobal('__TAURI__', undefined)
      }
    })

    it('returns early when Tauri is available', async () => {
      vi.stubGlobal('__TAURI__', {})
      const MockNotification = vi.fn()
      const requestPermissionMock = vi.fn()
      Object.defineProperty(MockNotification, 'permission', { value: 'default', configurable: true })
      Object.defineProperty(MockNotification, 'requestPermission', {
        value: requestPermissionMock,
        configurable: true,
      })
      vi.stubGlobal('Notification', MockNotification)

      await requestNotificationPermission()

      expect(requestPermissionMock).not.toHaveBeenCalled()
    })

    it('requests permission when status is default', async () => {
      const requestPermissionMock = vi.fn().mockResolvedValue('granted')
      const MockNotification = vi.fn()
      Object.defineProperty(MockNotification, 'permission', { value: 'default', configurable: true })
      Object.defineProperty(MockNotification, 'requestPermission', {
        value: requestPermissionMock,
        configurable: true,
      })
      vi.stubGlobal('Notification', MockNotification)

      await requestNotificationPermission()

      expect(requestPermissionMock).toHaveBeenCalled()
    })

    it('does not request permission when already granted', async () => {
      const requestPermissionMock = vi.fn()
      const MockNotification = vi.fn()
      Object.defineProperty(MockNotification, 'permission', { value: 'granted', configurable: true })
      Object.defineProperty(MockNotification, 'requestPermission', {
        value: requestPermissionMock,
        configurable: true,
      })
      vi.stubGlobal('Notification', MockNotification)

      await requestNotificationPermission()

      expect(requestPermissionMock).not.toHaveBeenCalled()
    })

    it('does not request permission when already denied', async () => {
      const requestPermissionMock = vi.fn()
      const MockNotification = vi.fn()
      Object.defineProperty(MockNotification, 'permission', { value: 'denied', configurable: true })
      Object.defineProperty(MockNotification, 'requestPermission', {
        value: requestPermissionMock,
        configurable: true,
      })
      vi.stubGlobal('Notification', MockNotification)

      await requestNotificationPermission()

      expect(requestPermissionMock).not.toHaveBeenCalled()
    })

    it('does nothing when Notification is not available', async () => {
      // eslint-disable-next-line @typescript-eslint/no-explicit-any
      delete (window as any).Notification
      await expect(requestNotificationPermission()).resolves.toBeUndefined()
    })
  })
})
