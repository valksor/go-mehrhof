import { describe, it, expect, vi, beforeEach } from 'vitest'
import { downloadJSON } from './export'

describe('downloadJSON', () => {
  let clickSpy: ReturnType<typeof vi.fn>
  let createObjectURLSpy: ReturnType<typeof vi.spyOn>
  let revokeObjectURLSpy: ReturnType<typeof vi.spyOn>
  let createElementSpy: ReturnType<typeof vi.spyOn>

  beforeEach(() => {
    clickSpy = vi.fn()
    const fakeAnchor = {
      href: '',
      download: '',
      click: clickSpy,
    }
    createElementSpy = vi.spyOn(document, 'createElement').mockReturnValue(fakeAnchor as unknown as HTMLElement)
    createObjectURLSpy = vi.spyOn(URL, 'createObjectURL').mockReturnValue('blob:fake-url')
    revokeObjectURLSpy = vi.spyOn(URL, 'revokeObjectURL').mockImplementation(() => {})
  })

  it('creates an anchor element', () => {
    downloadJSON({ a: 1 }, 'test.json')
    expect(createElementSpy).toHaveBeenCalledWith('a')
  })

  it('triggers a click on the anchor', () => {
    downloadJSON({ a: 1 }, 'test.json')
    expect(clickSpy).toHaveBeenCalledOnce()
  })

  it('sets the download filename', () => {
    const fakeAnchor = { href: '', download: '', click: vi.fn() }
    createElementSpy.mockReturnValue(fakeAnchor)
    downloadJSON({ a: 1 }, 'my-data.json')
    expect(fakeAnchor.download).toBe('my-data.json')
  })

  it('creates a blob URL and revokes it after click', () => {
    downloadJSON({ a: 1 }, 'test.json')
    expect(createObjectURLSpy).toHaveBeenCalledOnce()
    expect(revokeObjectURLSpy).toHaveBeenCalledWith('blob:fake-url')
  })

  it('serializes data as pretty-printed JSON', () => {
    let capturedBlob: Blob | undefined
    createObjectURLSpy.mockImplementation((blob: Blob) => {
      capturedBlob = blob
      return 'blob:fake'
    })
    const data = { key: 'value', num: 42 }
    downloadJSON(data, 'test.json')
    expect(capturedBlob).toBeInstanceOf(Blob)
    expect(capturedBlob!.type).toBe('application/json')
  })

  it('handles null data', () => {
    expect(() => downloadJSON(null, 'null.json')).not.toThrow()
    expect(clickSpy).toHaveBeenCalledOnce()
  })

  it('handles array data', () => {
    expect(() => downloadJSON([1, 2, 3], 'arr.json')).not.toThrow()
    expect(clickSpy).toHaveBeenCalledOnce()
  })
})
