import { render } from '@testing-library/react'
import { describe, it, expect, vi, beforeEach } from 'vitest'
import { StateAnnouncer } from './StateAnnouncer'

const mockAnnounce = vi.fn()

vi.mock('./ui/useAnnouncer', () => ({
  useAnnouncer: () => ({ announce: mockAnnounce }),
}))

let mockGlobalState: Record<string, unknown> = {
  connected: false,
  connecting: false,
  selectedProject: null,
}

let mockProjectState: Record<string, unknown> = {
  state: 'none',
}

vi.mock('../stores/globalStore', () => ({
  useGlobalStore: (selector: (s: Record<string, unknown>) => unknown) => selector(mockGlobalState),
}))

vi.mock('../stores/projectStore', () => ({
  useProjectStore: (selector: (s: Record<string, unknown>) => unknown) => selector(mockProjectState),
}))

describe('StateAnnouncer', () => {
  beforeEach(() => {
    mockAnnounce.mockClear()
    mockGlobalState = {
      connected: false,
      connecting: false,
      selectedProject: null,
    }
    mockProjectState = { state: 'none' }
  })

  it('renders nothing (returns null)', () => {
    const { container } = render(<StateAnnouncer />)
    expect(container.innerHTML).toBe('')
  })

  it('does not announce on initial render', () => {
    render(<StateAnnouncer />)
    expect(mockAnnounce).not.toHaveBeenCalled()
  })
})
