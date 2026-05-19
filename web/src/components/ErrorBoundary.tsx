import { Component, type ErrorInfo, type ReactNode } from 'react'

interface Props {
  children: ReactNode
  fallback?: ReactNode
  onError?: (error: Error, errorInfo: ErrorInfo) => void
}

interface State {
  hasError: boolean
  error: Error | null
}

export class ErrorBoundary extends Component<Props, State> {
  constructor(props: Props) {
    super(props)
    this.state = { hasError: false, error: null }
  }

  static getDerivedStateFromError(error: Error): State {
    return { hasError: true, error }
  }

  componentDidCatch(error: Error, errorInfo: ErrorInfo) {
    console.error('[ErrorBoundary]', error, errorInfo)
    this.props.onError?.(error, errorInfo)
  }

  handleReload = () => {
    window.location.reload()
  }

  handleReset = () => {
    this.setState({ hasError: false, error: null })
  }

  render() {
    if (this.state.hasError) {
      if (this.props.fallback) return this.props.fallback

      return (
        <div className="flex flex-col items-center justify-center p-6 text-center">
          <h3 className="text-lg font-semibold text-error mb-2">Something went wrong</h3>
          <p className="text-sm text-base-content/60 mb-4">
            {this.state.error?.message || 'An unexpected error occurred'}
          </p>
          {import.meta.env.DEV && this.state.error && (
            <details className="text-left w-full max-w-lg">
              <summary className="cursor-pointer text-sm text-base-content/40">Stack trace</summary>
              <pre className="text-xs bg-base-200 p-3 rounded mt-2 overflow-auto max-h-48">
                {this.state.error.stack}
              </pre>
            </details>
          )}
          <div className="flex gap-2 justify-center mt-4">
            <button className="btn btn-sm btn-outline" onClick={this.handleReset}>
              Try again
            </button>
            <button className="btn btn-sm btn-primary" onClick={this.handleReload}>
              Reload Page
            </button>
          </div>
        </div>
      )
    }

    return this.props.children
  }
}
