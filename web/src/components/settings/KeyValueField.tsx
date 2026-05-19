import { useState } from 'react'
import type { Field } from '../../types/settings'

interface KeyValueFieldProps {
  field: Field
  value: Record<string, string> | null | undefined
  onChange: (value: Record<string, string>) => void
  disabled?: boolean
  error?: string
}

export function KeyValueField({ field, value, onChange, disabled, error }: KeyValueFieldProps) {
  const entries = Object.entries(value ?? {})
  const [newKey, setNewKey] = useState('')
  const [newValue, setNewValue] = useState('')

  const handleAdd = () => {
    if (!newKey.trim()) return
    onChange({
      ...value,
      [newKey.trim()]: newValue,
    })
    setNewKey('')
    setNewValue('')
  }

  const handleRemove = (key: string) => {
    const next = { ...value }
    delete next[key]
    onChange(next)
  }

  const handleValueChange = (key: string, newVal: string) => {
    onChange({
      ...value,
      [key]: newVal,
    })
  }

  return (
    <div className="form-control">
      <label className="label">
        <span className="label-text">{field.label}</span>
      </label>

      {/* Existing entries */}
      <div className="space-y-2">
        {entries.map(([key, val]) => (
          <div key={key} className="flex gap-2 items-center">
            <input
              type="text"
              value={key}
              disabled
              className="input input-bordered input-sm flex-1 font-mono bg-base-200"
            />
            <span className="text-base-content/50">=</span>
            <input
              type="text"
              value={val}
              onChange={(e) => handleValueChange(key, e.target.value)}
              disabled={disabled}
              className="input input-bordered input-sm flex-1 font-mono"
              placeholder="Value"
            />
            <button
              type="button"
              onClick={() => handleRemove(key)}
              disabled={disabled}
              className="btn btn-ghost btn-sm btn-square text-error"
              aria-label={`Remove ${key}`}
            >
              <svg aria-hidden="true" className="w-4 h-4" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M6 18L18 6M6 6l12 12" />
              </svg>
            </button>
          </div>
        ))}
      </div>

      {/* Add new entry */}
      <div className="flex gap-2 items-center mt-2">
        <input
          type="text"
          value={newKey}
          onChange={(e) => setNewKey(e.target.value)}
          disabled={disabled}
          className="input input-bordered input-sm flex-1 font-mono"
          placeholder="KEY"
          onKeyDown={(e) => e.key === 'Enter' && handleAdd()}
        />
        <span className="text-base-content/50">=</span>
        <input
          type="text"
          value={newValue}
          onChange={(e) => setNewValue(e.target.value)}
          disabled={disabled}
          className="input input-bordered input-sm flex-1 font-mono"
          placeholder="value"
          onKeyDown={(e) => e.key === 'Enter' && handleAdd()}
        />
        <button
          type="button"
          onClick={handleAdd}
          disabled={disabled || !newKey.trim()}
          className="btn btn-ghost btn-sm btn-square text-success"
          aria-label="Add entry"
        >
          <svg aria-hidden="true" className="w-4 h-4" fill="none" viewBox="0 0 24 24" stroke="currentColor">
            <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M12 4v16m8-8H4" />
          </svg>
        </button>
      </div>

      {field.description && (
        <label className="label">
          <span className="label-text-alt text-base-content/50">{field.description}</span>
        </label>
      )}
      {error && (
        <label className="label">
          <span className="label-text-alt text-error">{error}</span>
        </label>
      )}
    </div>
  )
}
