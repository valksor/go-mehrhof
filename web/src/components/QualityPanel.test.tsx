import { render, waitFor, act } from '@testing-library/react'
import { describe, it, expect, vi, beforeEach } from 'vitest'
import { QualityPanel } from './QualityPanel'

const mockCall = vi.fn()
const mockClient = { call: mockCall }

let mockConnected = true
let mockQualityPrompt: { question: string } | null = null

vi.mock('../stores/projectStore', () => ({
  useProjectStore: Object.assign(
    (selector?: (s: Record<string, unknown>) => unknown) => {
      const state = { connected: mockConnected, client: mockClient, qualityPrompt: mockQualityPrompt }
      if (selector) return selector(state)
      return state
    },
    {
      getState: () => ({ client: mockClient }),
    },
  ),
}))

const sampleFindings = [
  {
    severity: 'high',
    category: 'lint',
    rule: 'no-unused-vars',
    file: 'src/main.ts',
    line: 42,
    message: 'Variable x is declared but never used',
    origin: 'eslint',
    classification: 'genuine',
  },
  {
    severity: 'medium',
    category: 'lint',
    rule: 'no-console',
    file: 'src/utils.ts',
    line: 10,
    message: 'Unexpected console statement',
    origin: 'eslint',
    classification: 'flaky',
  },
]

function mockRPCByMethod(
  autofix: unknown = null,
  failclass: unknown = null,
  quality: unknown = { findings: [], status: 'idle' },
) {
  mockCall.mockImplementation((method: string) => {
    switch (method) {
      case 'autofix.status':
        return Promise.resolve(autofix)
      case 'failclass.stats':
        return Promise.resolve(failclass)
      case 'quality.respond':
        return Promise.resolve(quality)
      default:
        return Promise.reject(new Error(`Unknown method: ${method}`))
    }
  })
}

describe('QualityPanel', () => {
  beforeEach(() => {
    mockCall.mockReset()
    mockConnected = true
    mockQualityPrompt = null
  })

  it('does not render when closed', () => {
    const { queryByRole } = render(
      <QualityPanel isOpen={false} onClose={vi.fn()} />,
    )
    expect(queryByRole('dialog')).not.toBeInTheDocument()
  })

  it('renders modal with title when open', async () => {
    mockRPCByMethod()
    const { getByText, findByText } = render(
      <QualityPanel isOpen={true} onClose={vi.fn()} />,
    )
    expect(getByText('Quality Gates')).toBeInTheDocument()
    await findByText('No quality gate data')
  })

  it('shows loading spinner while fetching', () => {
    mockCall.mockReturnValue(new Promise(() => {}))
    const { container } = render(
      <QualityPanel isOpen={true} onClose={vi.fn()} />,
    )
    expect(container.querySelector('.loading-spinner')).toBeInTheDocument()
  })

  it('shows empty state when no data', async () => {
    mockRPCByMethod(null, null, { findings: [], status: 'idle' })
    const { findByText } = render(
      <QualityPanel isOpen={true} onClose={vi.fn()} />,
    )
    expect(await findByText('No quality gate data')).toBeInTheDocument()
  })

  it('shows autofix idle status', async () => {
    mockRPCByMethod(
      { active: false, attempt: 0, max_attempts: 3 },
    )
    const { findByText } = render(
      <QualityPanel isOpen={true} onClose={vi.fn()} />,
    )
    expect(await findByText('Idle')).toBeInTheDocument()
  })

  it('shows autofix running status', async () => {
    mockRPCByMethod(
      { active: true, attempt: 2, max_attempts: 5 },
    )
    const { findByText } = render(
      <QualityPanel isOpen={true} onClose={vi.fn()} />,
    )
    expect(await findByText('Running')).toBeInTheDocument()
    expect(await findByText('2/5')).toBeInTheDocument()
  })

  it('shows autofix last error', async () => {
    mockRPCByMethod(
      { active: true, attempt: 1, max_attempts: 3, last_error: 'lint failed on line 5' },
    )
    const { findByText } = render(
      <QualityPanel isOpen={true} onClose={vi.fn()} />,
    )
    expect(await findByText('lint failed on line 5')).toBeInTheDocument()
  })

  it('shows failclass stats', async () => {
    mockRPCByMethod(
      null,
      { total: 10, genuine: 3, flaky: 5, intermittent: 2, unclassified: 0 },
    )
    const { findByText } = render(
      <QualityPanel isOpen={true} onClose={vi.fn()} />,
    )
    expect(await findByText('10')).toBeInTheDocument()
    expect(await findByText('3')).toBeInTheDocument()
    expect(await findByText('5')).toBeInTheDocument()
    expect(await findByText('2')).toBeInTheDocument()
  })

  it('shows findings table', async () => {
    mockRPCByMethod(
      null,
      null,
      { findings: sampleFindings, status: 'done' },
    )
    const { findByText } = render(
      <QualityPanel isOpen={true} onClose={vi.fn()} />,
    )
    expect(await findByText('high')).toBeInTheDocument()
    expect(await findByText('medium')).toBeInTheDocument()
    expect(await findByText('no-unused-vars')).toBeInTheDocument()
    expect(await findByText('no-console')).toBeInTheDocument()
    expect(await findByText('src/main.ts:42')).toBeInTheDocument()
    expect(await findByText('src/utils.ts:10')).toBeInTheDocument()
  })

  it('shows quality prompt warning', async () => {
    mockQualityPrompt = { question: 'Approve suppressing 3 lint errors?' }
    mockRPCByMethod()
    const { findByText } = render(
      <QualityPanel isOpen={true} onClose={vi.fn()} />,
    )
    expect(await findByText('Quality gate requires input')).toBeInTheDocument()
    expect(await findByText('Approve suppressing 3 lint errors?')).toBeInTheDocument()
  })

  it('shows error on RPC failure', async () => {
    mockCall.mockImplementation(() => { throw new Error('Connection refused') })
    const { findByText } = render(
      <QualityPanel isOpen={true} onClose={vi.fn()} />,
    )
    expect(await findByText('Connection refused')).toBeInTheDocument()
  })

  it('shows generic error for non-Error', async () => {
    mockCall.mockImplementation(() => { throw 'string error' }) // eslint-disable-line no-throw-literal
    const { findByText } = render(
      <QualityPanel isOpen={true} onClose={vi.fn()} />,
    )
    expect(await findByText('Failed to load quality data')).toBeInTheDocument()
  })

  it('refresh button triggers reload', async () => {
    mockRPCByMethod()
    const { getByLabelText, findByText } = render(
      <QualityPanel isOpen={true} onClose={vi.fn()} />,
    )
    await findByText('No quality gate data')

    const callCountBefore = mockCall.mock.calls.length

    mockRPCByMethod()
    await act(async () => { getByLabelText('Refresh quality data').click() })

    await waitFor(() => {
      expect(mockCall.mock.calls.length).toBeGreaterThan(callCountBefore)
    })
  })

  it('calls onClose when close button clicked', async () => {
    mockRPCByMethod()
    const onClose = vi.fn()
    const { getByLabelText, findByText } = render(
      <QualityPanel isOpen={true} onClose={onClose} />,
    )
    await findByText('No quality gate data')
    getByLabelText('Close dialog').click()
    expect(onClose).toHaveBeenCalledOnce()
  })

  it('does not show quality prompt when null', async () => {
    mockQualityPrompt = null
    mockRPCByMethod()
    const { findByText, queryByText } = render(
      <QualityPanel isOpen={true} onClose={vi.fn()} />,
    )
    await findByText('No quality gate data')
    expect(queryByText('Quality gate requires input')).not.toBeInTheDocument()
  })
})
