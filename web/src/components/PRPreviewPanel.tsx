import { useState } from 'react'
import { useProjectStore } from '../stores/projectStore'

interface PRPreviewPanelProps {
  data?: Record<string, unknown>
}

interface PreviewData {
  title: string
  body: string
  branch: string
  base_branch: string
  diff_stat?: string
  checkpoints: number
  specifications: number
}

export function PRPreviewPanel({ data }: PRPreviewPanelProps) {
  const preview = data?.preview as PreviewData | undefined
  const [editing, setEditing] = useState(false)
  // Initialize from preview body. The tab is recreated with fresh data on each dry-run,
  // so we don't need to sync — the component remounts with new props.
  const [editBody, setEditBody] = useState(preview?.body ?? '')
  const setPrBodyOverride = useProjectStore((s) => s.setPrBodyOverride)

  if (!preview) {
    return (
      <div className="flex items-center justify-center h-full text-base-content/50">
        <p className="text-sm">No preview data</p>
      </div>
    )
  }

  const handleToggleEdit = () => {
    if (editing) {
      // Save: update the store override
      setPrBodyOverride(editBody !== preview.body ? editBody : null)
    }
    setEditing(!editing)
  }

  const handleReset = () => {
    setEditBody(preview.body)
    setPrBodyOverride(null)
    setEditing(false)
  }

  return (
    <div className="h-full overflow-auto p-4">
      <div className="space-y-4 max-w-3xl">
        {/* Header */}
        <div className="space-y-2">
          <h1 className="text-xl font-bold">{preview.title}</h1>
          <div className="flex items-center gap-3 text-sm text-base-content/60">
            <span className="font-mono bg-base-200 px-2 py-0.5 rounded">{preview.branch}</span>
            <span>{'→'}</span>
            <span className="font-mono bg-base-200 px-2 py-0.5 rounded">{preview.base_branch}</span>
          </div>
        </div>

        {/* Metadata badges */}
        <div className="flex gap-2 flex-wrap">
          {preview.specifications > 0 && (
            <span className="badge badge-outline badge-sm">{preview.specifications} spec(s)</span>
          )}
          {preview.checkpoints > 0 && (
            <span className="badge badge-outline badge-sm">{preview.checkpoints} checkpoint(s)</span>
          )}
        </div>

        {/* Edit controls */}
        <div className="flex items-center gap-2">
          <button className={`btn btn-sm ${editing ? 'btn-primary' : 'btn-ghost'}`} onClick={handleToggleEdit}>
            {editing ? 'Save' : 'Edit'}
          </button>
          {editing && (
            <button className="btn btn-sm btn-ghost" onClick={handleReset}>
              Reset
            </button>
          )}
          {!editing && editBody !== preview.body && <span className="text-xs text-warning">Modified</span>}
        </div>

        {/* PR Body — editable or rendered */}
        <div className="divider" />
        {editing ? (
          <textarea
            className="textarea textarea-bordered w-full font-mono text-sm"
            rows={20}
            value={editBody}
            onChange={(e) => setEditBody(e.target.value)}
          />
        ) : (
          <PRBodyRenderer body={editBody} />
        )}

        {/* Diff stat */}
        {preview.diff_stat && (
          <details className="collapse collapse-arrow bg-base-200">
            <summary className="collapse-title text-sm font-medium">Diff stats</summary>
            <div className="collapse-content">
              <pre className="text-xs font-mono whitespace-pre-wrap">{preview.diff_stat}</pre>
            </div>
          </details>
        )}
      </div>
    </div>
  )
}

function PRBodyRenderer({ body }: { body: string }) {
  // Split PR body into sections by ## headers and render each
  const sections = parseSections(body)

  return (
    <div className="space-y-4">
      {sections.map((section, i) => (
        <div key={i}>
          {section.title && <h2 className="text-base font-semibold mb-2">{section.title}</h2>}
          {section.isCollapsible ? (
            <details className="collapse collapse-arrow bg-base-200">
              <summary className="collapse-title text-sm font-medium">{section.summaryText || 'Details'}</summary>
              <div className="collapse-content">
                <pre className="text-xs whitespace-pre-wrap">{section.content}</pre>
              </div>
            </details>
          ) : (
            <div className="text-sm text-base-content/80 whitespace-pre-wrap">{section.content}</div>
          )}
        </div>
      ))}
    </div>
  )
}

interface Section {
  title: string
  content: string
  isCollapsible: boolean
  summaryText?: string
}

function parseSections(body: string): Section[] {
  const lines = body.split('\n')
  const sections: Section[] = []
  let current: Section | null = null

  for (const line of lines) {
    if (line.startsWith('## ')) {
      if (current) sections.push(current)
      current = {
        title: line.slice(3).trim(),
        content: '',
        isCollapsible: false,
      }
    } else if (line.startsWith('---') && line.trim() === '---') {
      // Skip horizontal rules (footer separator)
      continue
    } else if (line.startsWith('*Generated by')) {
      // Skip footer
      continue
    } else if (line.startsWith('<details>')) {
      // Mark as collapsible section
      if (current) current.isCollapsible = true
    } else if (line.startsWith('<summary>')) {
      if (current) current.summaryText = line.replace(/<\/?summary>/g, '').trim()
    } else if (line.startsWith('</details>') || line.startsWith('</summary>')) {
      continue
    } else {
      if (!current) {
        current = { title: '', content: '', isCollapsible: false }
      }
      current.content += (current.content ? '\n' : '') + line
    }
  }

  if (current) sections.push(current)

  // Trim content
  return sections.map((s) => ({ ...s, content: s.content.trim() })).filter((s) => s.content || s.title)
}
