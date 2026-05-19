import { describe, it, expect, vi } from 'vitest'
import { asyncAction } from './store-helpers'

describe('asyncAction', () => {
  it('sets loading true before fn and false after (in order)', async () => {
    const set = vi.fn()
    await asyncAction(set, async () => {})
    expect(set).toHaveBeenNthCalledWith(1, { isLoading: true, error: null })
    expect(set).toHaveBeenLastCalledWith({ isLoading: false })
  })

  it('does not set error on success', async () => {
    const set = vi.fn()
    await asyncAction(set, async () => {})
    const errorCalls = set.mock.calls.filter((args: unknown[]) => {
      const arg = args[0] as Record<string, unknown>
      return 'error' in arg && arg.error !== null
    })
    expect(errorCalls).toHaveLength(0)
  })

  it('sets error message and re-throws on failure', async () => {
    const set = vi.fn()
    await expect(
      asyncAction(set, async () => {
        throw new Error('boom')
      }),
    ).rejects.toThrow('boom')
    expect(set).toHaveBeenCalledWith({ error: 'boom' })
  })

  it('sets loading false even on failure', async () => {
    const set = vi.fn()
    await asyncAction(set, async () => {
      throw new Error('fail')
    }).catch(() => {})
    const lastCall = set.mock.calls[set.mock.calls.length - 1][0]
    expect(lastCall).toEqual({ isLoading: false })
  })

  it('uses custom loadingKey and errorKey', async () => {
    const set = vi.fn()
    await asyncAction(set, async () => {}, { loadingKey: 'isFetching', errorKey: 'fetchError' })
    expect(set).toHaveBeenCalledWith({ isFetching: true, fetchError: null })
    expect(set).toHaveBeenCalledWith({ isFetching: false })
  })

  it('logs to console.error when showToast is true and fn fails', async () => {
    const set = vi.fn()
    const spy = vi.spyOn(console, 'error').mockImplementation(() => {})
    try {
      await asyncAction(
        set,
        async () => {
          throw new Error('toast me')
        },
        { showToast: true },
      ).catch(() => {})
      expect(spy).toHaveBeenCalledWith(expect.stringContaining('toast me'))
    } finally {
      spy.mockRestore()
    }
  })

  it('uses custom toastMessage in console.error', async () => {
    const set = vi.fn()
    const spy = vi.spyOn(console, 'error').mockImplementation(() => {})
    try {
      await asyncAction(
        set,
        async () => {
          throw new Error('err')
        },
        {
          showToast: true,
          toastMessage: 'Custom toast',
        },
      ).catch(() => {})
      expect(spy).toHaveBeenCalledWith(expect.stringContaining('Custom toast'))
    } finally {
      spy.mockRestore()
    }
  })

  it('converts non-Error throws to string', async () => {
    const set = vi.fn()
    await asyncAction(set, async () => {
      // eslint-disable-next-line @typescript-eslint/only-throw-error -- intentionally testing non-Error throw handling
      throw 'string error'
    }).catch(() => {})
    expect(set).toHaveBeenCalledWith({ error: 'string error' })
  })
})
