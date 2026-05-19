import { render, waitFor, act } from '@testing-library/react'
import { describe, it, expect, vi, beforeEach } from 'vitest'
import { DiagnosePanel } from './DiagnosePanel'

const mockCall = vi.fn()
const mockClient = { call: mockCall }

let mockConnected = true

vi.mock('../stores/globalStore', () => ({
  useGlobalStore: Object.assign(
    (selector?: (s: Record<string, unknown>) => unknown) => {
      const state = { connected: mockConnected, client: mockClient }
      if (selector) return selector(state)
      return state
    },
    {
      getState: () => ({ client: mockClient }),
    },
  ),
}))

const allPassedData = {
  checks: [
    { name: 'git', status: 'passed', detail: 'v2.43' },
    { name: 'claude', status: 'passed', detail: '/usr/local/bin/claude' },
  ],
  global_socket: 'running',
  providers: [
    { name: 'GitHub', configured: true },
    { name: 'Linear', configured: false },
  ],
  issues: [],
}

const failedData = {
  checks: [
    { name: 'git', status: 'passed', detail: 'v2.43' },
    { name: 'claude', status: 'failed', fix: 'Install Claude CLI' },
    { name: 'codex', status: 'warning', fix: 'Optional: install codex' },
  ],
  global_socket: 'running',
  providers: [{ name: 'GitHub', configured: true }],
  issues: ['Install Claude CLI', 'Consider installing codex'],
}

describe('DiagnosePanel', () => {
  beforeEach(() => {
    mockCall.mockReset()
    mockConnected = true
  })

  it('does not render when closed', () => {
    const { queryByRole } = render(<DiagnosePanel isOpen={false} onClose={vi.fn()} />)
    expect(queryByRole('dialog')).not.toBeInTheDocument()
  })

  it('renders modal with title when open', async () => {
    mockCall.mockResolvedValueOnce(allPassedData)
    const { getByText, findByText } = render(<DiagnosePanel isOpen={true} onClose={vi.fn()} />)
    expect(getByText('System Diagnostics')).toBeInTheDocument()
    await findByText('All checks passed')
  })

  it('shows loading spinner while fetching', () => {
    // Never-resolving promise to keep loading state
    mockCall.mockReturnValue(new Promise(() => {}))
    const { container } = render(<DiagnosePanel isOpen={true} onClose={vi.fn()} />)
    expect(container.querySelector('.loading-spinner')).toBeInTheDocument()
  })

  it('shows "All checks passed" when no issues', async () => {
    mockCall.mockResolvedValueOnce(allPassedData)
    const { findByText } = render(<DiagnosePanel isOpen={true} onClose={vi.fn()} />)
    expect(await findByText('All checks passed')).toBeInTheDocument()
  })

  it('displays check names using display names', async () => {
    mockCall.mockResolvedValueOnce(allPassedData)
    const { findByText } = render(<DiagnosePanel isOpen={true} onClose={vi.fn()} />)
    expect(await findByText('Git')).toBeInTheDocument()
    expect(await findByText('Claude CLI')).toBeInTheDocument()
  })

  it('shows status badges for checks', async () => {
    mockCall.mockResolvedValueOnce(allPassedData)
    const { findAllByText } = render(<DiagnosePanel isOpen={true} onClose={vi.fn()} />)
    const okBadges = await findAllByText('OK')
    expect(okBadges.length).toBe(2)
  })

  it('shows failed badge for failed checks', async () => {
    mockCall.mockResolvedValueOnce(failedData)
    const { findByText } = render(<DiagnosePanel isOpen={true} onClose={vi.fn()} />)
    expect(await findByText('Failed')).toBeInTheDocument()
  })

  it('shows warning badge for warning checks', async () => {
    mockCall.mockResolvedValueOnce(failedData)
    const { findByText } = render(<DiagnosePanel isOpen={true} onClose={vi.fn()} />)
    expect(await findByText('Warning')).toBeInTheDocument()
  })

  it('shows issue count when issues exist', async () => {
    mockCall.mockResolvedValueOnce(failedData)
    const { findByText } = render(<DiagnosePanel isOpen={true} onClose={vi.fn()} />)
    expect(await findByText('2 issues found')).toBeInTheDocument()
  })

  it('shows singular "issue" for single issue', async () => {
    const singleIssue = { ...failedData, issues: ['One problem'] }
    mockCall.mockResolvedValueOnce(singleIssue)
    const { findByText } = render(<DiagnosePanel isOpen={true} onClose={vi.fn()} />)
    expect(await findByText('1 issue found')).toBeInTheDocument()
  })

  it('shows fix text for checks with fixes', async () => {
    mockCall.mockResolvedValueOnce(failedData)
    const { findAllByText } = render(<DiagnosePanel isOpen={true} onClose={vi.fn()} />)
    // "Install Claude CLI" appears as both check fix text and in issues list
    const matches = await findAllByText('Install Claude CLI')
    expect(matches.length).toBeGreaterThanOrEqual(1)
  })

  it('shows detail text for passed checks', async () => {
    mockCall.mockResolvedValueOnce(allPassedData)
    const { findByText } = render(<DiagnosePanel isOpen={true} onClose={vi.fn()} />)
    expect(await findByText('v2.43')).toBeInTheDocument()
  })

  it('shows Global Socket status', async () => {
    mockCall.mockResolvedValueOnce(allPassedData)
    const { findByText } = render(<DiagnosePanel isOpen={true} onClose={vi.fn()} />)
    expect(await findByText('Global Socket')).toBeInTheDocument()
    expect(await findByText('Running')).toBeInTheDocument()
  })

  it('shows provider tokens section', async () => {
    mockCall.mockResolvedValueOnce(allPassedData)
    const { findByText } = render(<DiagnosePanel isOpen={true} onClose={vi.fn()} />)
    expect(await findByText('Provider Tokens')).toBeInTheDocument()
    expect(await findByText('GitHub')).toBeInTheDocument()
    expect(await findByText('Configured')).toBeInTheDocument()
    expect(await findByText('Not configured')).toBeInTheDocument()
  })

  it('shows Next Steps section when issues exist', async () => {
    mockCall.mockResolvedValueOnce(failedData)
    const { findByText } = render(<DiagnosePanel isOpen={true} onClose={vi.fn()} />)
    expect(await findByText('Next Steps')).toBeInTheDocument()
  })

  it('does not show Next Steps when all checks pass', async () => {
    mockCall.mockResolvedValueOnce(allPassedData)
    const { findByText, queryByText } = render(<DiagnosePanel isOpen={true} onClose={vi.fn()} />)
    await findByText('All checks passed')
    expect(queryByText('Next Steps')).not.toBeInTheDocument()
  })

  it('shows error message when RPC call fails', async () => {
    mockCall.mockRejectedValueOnce(new Error('Connection refused'))
    const { findByText } = render(<DiagnosePanel isOpen={true} onClose={vi.fn()} />)
    expect(await findByText('Connection refused')).toBeInTheDocument()
  })

  it('shows generic error for non-Error rejects', async () => {
    mockCall.mockRejectedValueOnce('string error')
    const { findByText } = render(<DiagnosePanel isOpen={true} onClose={vi.fn()} />)
    expect(await findByText('Diagnosis failed')).toBeInTheDocument()
  })

  it('has a Re-run button that triggers another diagnose', async () => {
    mockCall.mockResolvedValueOnce(allPassedData)
    const { findByText } = render(<DiagnosePanel isOpen={true} onClose={vi.fn()} />)
    const rerun = await findByText('Re-run')
    expect(rerun).toBeInTheDocument()

    mockCall.mockResolvedValueOnce(failedData)
    await act(async () => {
      rerun.click()
    })
    await waitFor(() => {
      expect(mockCall).toHaveBeenCalledTimes(2)
    })
  })

  it('calls system.diagnose RPC method', async () => {
    mockCall.mockResolvedValueOnce(allPassedData)
    render(<DiagnosePanel isOpen={true} onClose={vi.fn()} />)
    await waitFor(() => {
      expect(mockCall).toHaveBeenCalledWith('system.diagnose', {})
    })
  })

  it('calls onClose when close button is clicked', async () => {
    mockCall.mockResolvedValueOnce(allPassedData)
    const onClose = vi.fn()
    const { getByLabelText, findByText } = render(<DiagnosePanel isOpen={true} onClose={onClose} />)
    await findByText('All checks passed')
    getByLabelText('Close dialog').click()
    expect(onClose).toHaveBeenCalledOnce()
  })
})
