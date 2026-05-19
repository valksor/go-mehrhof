import { useRef, useId, useEffect, type ReactNode } from 'react'
import { FocusTrap } from 'focus-trap-react'

interface AccessibleModalProps {
  isOpen: boolean
  onClose: () => void
  title: string
  children: ReactNode
  actions?: ReactNode
  size?: 'sm' | 'md' | 'lg' | 'xl' | '2xl' | '3xl' | '4xl' | '5xl'
}

const sizeClasses: Record<string, string> = {
  sm: 'max-w-sm',
  md: 'max-w-md',
  lg: 'max-w-lg',
  xl: 'max-w-xl',
  '2xl': 'max-w-2xl',
  '3xl': 'max-w-3xl',
  '4xl': 'max-w-4xl',
  '5xl': 'max-w-5xl',
}

export function AccessibleModal({ isOpen, onClose, title, children, actions, size = '3xl' }: AccessibleModalProps) {
  const closeRef = useRef<HTMLButtonElement>(null)
  const titleId = useId()

  // Handle Escape key globally when modal is open
  useEffect(() => {
    if (!isOpen) return
    const handleKeyDown = (e: KeyboardEvent) => {
      if (e.key === 'Escape') onClose()
    }
    document.addEventListener('keydown', handleKeyDown)
    return () => document.removeEventListener('keydown', handleKeyDown)
  }, [isOpen, onClose])

  if (!isOpen) return null

  return (
    <FocusTrap
      focusTrapOptions={{
        initialFocus: () => closeRef.current ?? false,
        allowOutsideClick: true,
        escapeDeactivates: false, // We handle Escape via useEffect
      }}
    >
      <div className="modal modal-open" role="dialog" aria-modal="true" aria-labelledby={titleId}>
        <div className={`modal-box ${sizeClasses[size] ?? sizeClasses['3xl']}`}>
          <div className="flex items-start justify-between mb-4">
            <h2 id={titleId} className="text-xl font-bold">
              {title}
            </h2>
            <button
              ref={closeRef}
              onClick={onClose}
              className="btn btn-ghost btn-sm btn-circle"
              aria-label="Close dialog"
            >
              <svg
                aria-hidden="true"
                width="20"
                height="20"
                viewBox="0 0 24 24"
                fill="none"
                stroke="currentColor"
                strokeWidth="2"
              >
                <path d="M18 6 6 18M6 6l12 12" />
              </svg>
            </button>
          </div>
          <div>{children}</div>
          {actions && <div className="modal-action">{actions}</div>}
        </div>
        <div className="modal-backdrop" onClick={onClose} aria-hidden="true" data-testid="modal-backdrop" />
      </div>
    </FocusTrap>
  )
}
