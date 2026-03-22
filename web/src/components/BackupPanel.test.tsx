import { render, waitFor, act } from '@testing-library/react'
import { describe, it, expect, vi, beforeEach } from 'vitest'
import { BackupPanel } from './BackupPanel'

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

const sampleBackups = [
  {
    name: 'backup-2026-03-01.tar.gz',
    path: '/home/user/.valksor/kvelmo/backups/backup-2026-03-01.tar.gz',
    size: 1048576,
    created_at: '2026-03-01T10:00:00Z',
  },
  {
    name: 'backup-2026-03-10.tar.gz',
    path: '/home/user/.valksor/kvelmo/backups/backup-2026-03-10.tar.gz',
    size: 2097152,
    created_at: '2026-03-10T15:30:00Z',
  },
]

const createResult = {
  path: '/home/user/.valksor/kvelmo/backups/backup-2026-03-14.tar.gz',
  size: 524288,
  files: 15,
}

describe('BackupPanel', () => {
  beforeEach(() => {
    mockCall.mockReset()
    mockConnected = true
  })

  it('does not render when closed', () => {
    const { queryByRole } = render(
      <BackupPanel isOpen={false} onClose={vi.fn()} />,
    )
    expect(queryByRole('dialog')).not.toBeInTheDocument()
  })

  it('renders modal with title when open', async () => {
    mockCall.mockResolvedValueOnce({ backups: [] })
    const { getByText, findByText } = render(
      <BackupPanel isOpen={true} onClose={vi.fn()} />,
    )
    expect(getByText('Backup')).toBeInTheDocument()
    await findByText('No backups found')
  })

  it('shows description text', async () => {
    mockCall.mockResolvedValueOnce({ backups: [] })
    const { getByText, findByText } = render(
      <BackupPanel isOpen={true} onClose={vi.fn()} />,
    )
    expect(
      getByText('Create and manage backups of kvelmo state'),
    ).toBeInTheDocument()
    await findByText('No backups found')
  })

  it('has a Create Backup button', async () => {
    mockCall.mockResolvedValueOnce({ backups: [] })
    const { getByText, findByText } = render(
      <BackupPanel isOpen={true} onClose={vi.fn()} />,
    )
    expect(getByText('Create Backup')).toBeInTheDocument()
    await findByText('No backups found')
  })

  it('shows loading spinner while fetching backups', () => {
    mockCall.mockReturnValue(new Promise(() => {}))
    const { container } = render(
      <BackupPanel isOpen={true} onClose={vi.fn()} />,
    )
    expect(container.querySelector('.loading-spinner')).toBeInTheDocument()
  })

  it('shows empty state when no backups', async () => {
    mockCall.mockResolvedValueOnce({ backups: [] })
    const { findByText } = render(
      <BackupPanel isOpen={true} onClose={vi.fn()} />,
    )
    expect(await findByText('No backups found')).toBeInTheDocument()
  })

  it('shows helper text in empty state', async () => {
    mockCall.mockResolvedValueOnce({ backups: [] })
    const { findByText } = render(
      <BackupPanel isOpen={true} onClose={vi.fn()} />,
    )
    expect(
      await findByText('Click "Create Backup" to create your first backup'),
    ).toBeInTheDocument()
  })

  it('shows "Existing Backups" heading', async () => {
    mockCall.mockResolvedValueOnce({ backups: [] })
    const { findByText } = render(
      <BackupPanel isOpen={true} onClose={vi.fn()} />,
    )
    expect(await findByText('Existing Backups')).toBeInTheDocument()
  })

  it('lists existing backups with names', async () => {
    mockCall.mockResolvedValueOnce({ backups: sampleBackups })
    const { findByText } = render(
      <BackupPanel isOpen={true} onClose={vi.fn()} />,
    )
    expect(await findByText('backup-2026-03-01.tar.gz')).toBeInTheDocument()
    expect(await findByText('backup-2026-03-10.tar.gz')).toBeInTheDocument()
  })

  it('shows backup file paths', async () => {
    mockCall.mockResolvedValueOnce({ backups: sampleBackups })
    const { findByText } = render(
      <BackupPanel isOpen={true} onClose={vi.fn()} />,
    )
    expect(
      await findByText(
        '/home/user/.valksor/kvelmo/backups/backup-2026-03-01.tar.gz',
      ),
    ).toBeInTheDocument()
  })

  it('formats backup sizes', async () => {
    mockCall.mockResolvedValueOnce({ backups: sampleBackups })
    const { findByText } = render(
      <BackupPanel isOpen={true} onClose={vi.fn()} />,
    )
    expect(await findByText('1 MB')).toBeInTheDocument()
    expect(await findByText('2 MB')).toBeInTheDocument()
  })

  it('calls backup.list on open', async () => {
    mockCall.mockResolvedValueOnce({ backups: [] })
    render(<BackupPanel isOpen={true} onClose={vi.fn()} />)
    await waitFor(() => {
      expect(mockCall).toHaveBeenCalledWith('backup.list')
    })
  })

  it('creates backup when Create Backup is clicked', async () => {
    mockCall.mockResolvedValueOnce({ backups: [] }) // initial list
    const { findByText } = render(
      <BackupPanel isOpen={true} onClose={vi.fn()} />,
    )

    const btn = await findByText('Create Backup')

    mockCall.mockResolvedValueOnce(createResult) // backup.create
    mockCall.mockResolvedValueOnce({ backups: sampleBackups }) // refresh list
    await act(async () => { btn.click() })

    await waitFor(() => {
      expect(mockCall).toHaveBeenCalledWith('backup.create')
    })
  })

  it('shows success message after creating backup', async () => {
    mockCall.mockResolvedValueOnce({ backups: [] }) // initial list
    const { findByText } = render(
      <BackupPanel isOpen={true} onClose={vi.fn()} />,
    )

    const btn = await findByText('Create Backup')

    mockCall.mockResolvedValueOnce(createResult) // backup.create
    mockCall.mockResolvedValueOnce({ backups: [] }) // refresh list
    await act(async () => { btn.click() })

    expect(
      await findByText('Backup created successfully'),
    ).toBeInTheDocument()
  })

  it('shows backup path in success message', async () => {
    mockCall.mockResolvedValueOnce({ backups: [] })
    const { findByText } = render(
      <BackupPanel isOpen={true} onClose={vi.fn()} />,
    )

    const btn = await findByText('Create Backup')
    mockCall.mockResolvedValueOnce(createResult)
    mockCall.mockResolvedValueOnce({ backups: [] })
    await act(async () => { btn.click() })

    expect(
      await findByText(createResult.path),
    ).toBeInTheDocument()
  })

  it('shows backup size and file count in success message', async () => {
    mockCall.mockResolvedValueOnce({ backups: [] })
    const { findByText } = render(
      <BackupPanel isOpen={true} onClose={vi.fn()} />,
    )

    const btn = await findByText('Create Backup')
    mockCall.mockResolvedValueOnce(createResult)
    mockCall.mockResolvedValueOnce({ backups: [] })
    await act(async () => { btn.click() })

    expect(await findByText('512 KB (15 files)')).toBeInTheDocument()
  })

  it('refreshes backup list after creating', async () => {
    mockCall.mockResolvedValueOnce({ backups: [] }) // initial list
    const { findByText } = render(
      <BackupPanel isOpen={true} onClose={vi.fn()} />,
    )

    const btn = await findByText('Create Backup')
    mockCall.mockResolvedValueOnce(createResult) // backup.create
    mockCall.mockResolvedValueOnce({ backups: sampleBackups }) // refresh
    await act(async () => { btn.click() })

    // After refresh, should show the backups
    expect(await findByText('backup-2026-03-01.tar.gz')).toBeInTheDocument()
  })

  it('shows error when create fails', async () => {
    mockCall.mockResolvedValueOnce({ backups: [] })
    const { findByText } = render(
      <BackupPanel isOpen={true} onClose={vi.fn()} />,
    )

    const btn = await findByText('Create Backup')
    mockCall.mockRejectedValueOnce(new Error('Disk full'))
    await act(async () => { btn.click() })

    expect(await findByText('Disk full')).toBeInTheDocument()
  })

  it('shows generic error for non-Error rejects on create', async () => {
    mockCall.mockResolvedValueOnce({ backups: [] })
    const { findByText } = render(
      <BackupPanel isOpen={true} onClose={vi.fn()} />,
    )

    const btn = await findByText('Create Backup')
    mockCall.mockRejectedValueOnce('unknown')
    await act(async () => { btn.click() })

    expect(await findByText('Failed to create backup')).toBeInTheDocument()
  })

  it('shows error when list fails', async () => {
    mockCall.mockRejectedValueOnce(new Error('Permission denied'))
    const { findByText } = render(
      <BackupPanel isOpen={true} onClose={vi.fn()} />,
    )
    expect(await findByText('Permission denied')).toBeInTheDocument()
  })

  it('shows generic error for non-Error list failure', async () => {
    mockCall.mockRejectedValueOnce('oops')
    const { findByText } = render(
      <BackupPanel isOpen={true} onClose={vi.fn()} />,
    )
    expect(await findByText('Failed to list backups')).toBeInTheDocument()
  })

  it('handles null backups in response', async () => {
    mockCall.mockResolvedValueOnce({ backups: null })
    const { findByText } = render(
      <BackupPanel isOpen={true} onClose={vi.fn()} />,
    )
    expect(await findByText('No backups found')).toBeInTheDocument()
  })

  it('calls onClose when close button is clicked', async () => {
    mockCall.mockResolvedValueOnce({ backups: [] })
    const onClose = vi.fn()
    const { getByLabelText, findByText } = render(
      <BackupPanel isOpen={true} onClose={onClose} />,
    )
    await findByText('No backups found')
    getByLabelText('Close dialog').click()
    expect(onClose).toHaveBeenCalledOnce()
  })

  it('disables Create Backup button when disconnected', async () => {
    mockConnected = false
    const { getByText } = render(
      <BackupPanel isOpen={true} onClose={vi.fn()} />,
    )
    expect(getByText('Create Backup').closest('button')).toBeDisabled()
  })
})
