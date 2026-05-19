import { useEffect, useRef, memo } from 'react'
import { useVirtualizer } from '@tanstack/react-virtual'
import { useProjectStore } from '../stores/projectStore'

interface OutputWidgetProps {
  embedded?: boolean
}

// Memoized line component to prevent unnecessary re-renders
const OutputLine = memo(function OutputLine({ line }: { line: string }) {
  const className =
    line.startsWith('ERROR') || line.startsWith('error')
      ? 'text-error'
      : line.startsWith('WARN') || line.startsWith('warn')
        ? 'text-warning'
        : line.startsWith('✓') || line.startsWith('success')
          ? 'text-success'
          : ''

  return <div className={`leading-relaxed px-1 ${className}`}>{line}</div>
})

function VirtualizedOutput({ output }: { output: string[] }) {
  const parentRef = useRef<HTMLDivElement>(null)
  const lastOutputLength = useRef(output.length)

  const virtualizer = useVirtualizer({
    count: output.length,
    getScrollElement: () => parentRef.current,
    estimateSize: () => 24, // Estimated line height in pixels
    overscan: 10, // Render extra items above/below viewport
  })

  // Auto-scroll to bottom when new output is added
  useEffect(() => {
    if (output.length > lastOutputLength.current && parentRef.current) {
      // Only auto-scroll if we were already near the bottom
      const el = parentRef.current
      const isNearBottom = el.scrollHeight - el.scrollTop - el.clientHeight < 100
      if (isNearBottom) {
        virtualizer.scrollToIndex(output.length - 1, { align: 'end' })
      }
    }
    lastOutputLength.current = output.length
  }, [output.length, virtualizer])

  const items = virtualizer.getVirtualItems()

  return (
    <div
      ref={parentRef}
      className="bg-neutral rounded-lg p-4 h-full overflow-auto font-mono text-sm text-neutral-content"
    >
      <div
        style={{
          height: `${virtualizer.getTotalSize()}px`,
          width: '100%',
          position: 'relative',
        }}
      >
        {items.map((virtualItem) => (
          <div
            key={virtualItem.key}
            style={{
              position: 'absolute',
              top: 0,
              left: 0,
              width: '100%',
              transform: `translateY(${virtualItem.start}px)`,
            }}
          >
            <OutputLine line={output[virtualItem.index]} />
          </div>
        ))}
      </div>
    </div>
  )
}

function EmptyOutput() {
  return (
    <div className="bg-neutral rounded-lg p-4 h-full overflow-auto font-mono text-sm text-neutral-content">
      <div className="text-neutral-content/50 flex items-center justify-center h-full min-h-[200px]">
        <div className="text-center">
          <svg
            className="w-8 h-8 mx-auto mb-2 text-neutral-content/30"
            fill="none"
            viewBox="0 0 24 24"
            stroke="currentColor"
          >
            <path
              strokeLinecap="round"
              strokeLinejoin="round"
              strokeWidth={1.5}
              d="M8 9l3 3-3 3m5 0h3M5 20h14a2 2 0 002-2V6a2 2 0 00-2-2H5a2 2 0 00-2 2v12a2 2 0 002 2z"
            />
          </svg>
          <span>No output yet</span>
        </div>
      </div>
    </div>
  )
}

export function OutputWidget({ embedded = false }: OutputWidgetProps) {
  const { output, clearOutput } = useProjectStore()

  const content = output.length === 0 ? <EmptyOutput /> : <VirtualizedOutput output={output} />

  // Embedded mode: just the content without card wrapper
  if (embedded) {
    return (
      <div className="h-full flex flex-col">
        <div className="flex items-center justify-end p-2 border-b border-base-300">
          <button onClick={clearOutput} className="btn btn-ghost btn-xs">
            Clear
          </button>
        </div>
        <div className="flex-1 p-2 overflow-hidden">{content}</div>
      </div>
    )
  }

  // Normal mode: with card wrapper
  return (
    <section className="card bg-base-200 flex-1 flex flex-col min-h-[300px]">
      <div className="card-body flex flex-col">
        <div className="flex items-center justify-between">
          <h2 className="card-title text-base-content flex items-center gap-2">
            <svg className="w-5 h-5 text-primary" fill="none" viewBox="0 0 24 24" stroke="currentColor">
              <path
                strokeLinecap="round"
                strokeLinejoin="round"
                strokeWidth={2}
                d="M8 9l3 3-3 3m5 0h3M5 20h14a2 2 0 002-2V6a2 2 0 00-2-2H5a2 2 0 00-2 2v12a2 2 0 002 2z"
              />
            </svg>
            Output
          </h2>
          <button onClick={clearOutput} className="btn btn-ghost btn-xs">
            Clear
          </button>
        </div>

        <div className="flex-1 mt-4 overflow-auto">{content}</div>
      </div>
    </section>
  )
}
