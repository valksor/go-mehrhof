import { useState, useId } from 'react'
import type { Field, SelectOption } from '../../types/settings'
import { KeyValueField } from './KeyValueField'

interface ListItem {
  [key: string]: unknown
}

interface ListFieldProps {
  field: Field
  value: Record<string, ListItem> | null | undefined
  onChange: (value: Record<string, ListItem>) => void
  disabled?: boolean
  error?: string
}

export function ListField({ field, value, onChange, disabled, error }: ListFieldProps) {
  const items = value ?? {}
  const itemNames = Object.keys(items)
  const [expandedItems, setExpandedItems] = useState<Set<string>>(new Set())
  const [newItemName, setNewItemName] = useState('')
  const [showAddForm, setShowAddForm] = useState(false)
  const newItemInputId = useId()

  const toggleExpanded = (name: string) => {
    setExpandedItems((prev) => {
      const next = new Set(prev)
      if (next.has(name)) {
        next.delete(name)
      } else {
        next.add(name)
      }
      return next
    })
  }

  const handleAdd = () => {
    const name = newItemName.trim()
    if (!name || items[name]) return

    // Initialize with defaults from itemSchema
    const newItem: ListItem = {}
    field.itemSchema?.forEach((f) => {
      if (f.default !== undefined) {
        newItem[f.path] = f.default
      } else if (f.type === 'tags') {
        newItem[f.path] = []
      } else if (f.type === 'keyvalue') {
        newItem[f.path] = {}
      } else {
        newItem[f.path] = ''
      }
    })

    onChange({
      ...items,
      [name]: newItem,
    })
    setNewItemName('')
    setShowAddForm(false)
    setExpandedItems((prev) => new Set(prev).add(name))
  }

  const handleRemove = (name: string) => {
    const next = { ...items }
    delete next[name]
    onChange(next)
    setExpandedItems((prev) => {
      const next = new Set(prev)
      next.delete(name)
      return next
    })
  }

  const handleItemFieldChange = (itemName: string, fieldPath: string, fieldValue: unknown) => {
    onChange({
      ...items,
      [itemName]: {
        ...items[itemName],
        [fieldPath]: fieldValue,
      },
    })
  }

  const renderItemField = (itemName: string, itemField: Field) => {
    const fieldValue = (items[itemName] as Record<string, unknown>)?.[itemField.path]

    switch (itemField.type) {
      case 'select':
        return (
          <div className="form-control" key={itemField.path}>
            <label className="label py-1">
              <span className="label-text text-sm">{itemField.label}</span>
            </label>
            <select
              value={(fieldValue as string) ?? ''}
              onChange={(e) => handleItemFieldChange(itemName, itemField.path, e.target.value)}
              disabled={disabled}
              className="select select-bordered select-sm w-full"
            >
              <option value="">Select...</option>
              {itemField.options?.map((opt: SelectOption) => (
                <option key={opt.value} value={opt.value}>
                  {opt.label}
                </option>
              ))}
            </select>
            {itemField.description && (
              <label className="label py-0">
                <span className="label-text-alt text-base-content/50 text-xs">{itemField.description}</span>
              </label>
            )}
          </div>
        )

      case 'string':
        return (
          <div className="form-control" key={itemField.path}>
            <label className="label py-1">
              <span className="label-text text-sm">{itemField.label}</span>
            </label>
            <input
              type="text"
              value={(fieldValue as string) ?? ''}
              onChange={(e) => handleItemFieldChange(itemName, itemField.path, e.target.value)}
              disabled={disabled}
              className="input input-bordered input-sm w-full"
              placeholder={itemField.placeholder}
            />
            {itemField.description && (
              <label className="label py-0">
                <span className="label-text-alt text-base-content/50 text-xs">{itemField.description}</span>
              </label>
            )}
          </div>
        )

      case 'tags':
        const tagsValue = Array.isArray(fieldValue) ? (fieldValue as string[]) : []
        return (
          <div className="form-control" key={itemField.path}>
            <label className="label py-1">
              <span className="label-text text-sm">{itemField.label}</span>
            </label>
            <input
              type="text"
              value={tagsValue.join(', ')}
              onChange={(e) => {
                const tags = e.target.value
                  .split(',')
                  .map((t) => t.trim())
                  .filter(Boolean)
                handleItemFieldChange(itemName, itemField.path, tags)
              }}
              disabled={disabled}
              className="input input-bordered input-sm w-full font-mono"
              placeholder="comma-separated values"
            />
            {itemField.description && (
              <label className="label py-0">
                <span className="label-text-alt text-base-content/50 text-xs">{itemField.description}</span>
              </label>
            )}
          </div>
        )

      case 'keyvalue':
        return (
          <KeyValueField
            key={itemField.path}
            field={itemField}
            value={fieldValue as Record<string, string>}
            onChange={(v) => handleItemFieldChange(itemName, itemField.path, v)}
            disabled={disabled}
          />
        )

      default:
        return null
    }
  }

  return (
    <div className="form-control">
      <div className="flex items-center justify-between mb-2">
        <label className="label py-0">
          <span className="label-text font-medium">{field.label}</span>
        </label>
        {!showAddForm && (
          <button
            type="button"
            onClick={() => setShowAddForm(true)}
            disabled={disabled}
            className="btn btn-ghost btn-sm gap-1"
          >
            <svg className="w-4 h-4" fill="none" viewBox="0 0 24 24" stroke="currentColor">
              <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M12 4v16m8-8H4" />
            </svg>
            Add
          </button>
        )}
      </div>

      {field.description && <p className="text-sm text-base-content/50 mb-3">{field.description}</p>}

      {/* Add new item form */}
      {showAddForm && (
        <div className="card bg-base-200 p-3 mb-3">
          <div className="flex gap-2 items-end">
            <div className="form-control flex-1">
              <label htmlFor={newItemInputId} className="label py-1">
                <span className="label-text text-sm">Agent Name</span>
              </label>
              <input
                id={newItemInputId}
                type="text"
                value={newItemName}
                onChange={(e) => setNewItemName(e.target.value)}
                disabled={disabled}
                className="input input-bordered input-sm w-full font-mono"
                placeholder="my-agent"
                // eslint-disable-next-line jsx-a11y/no-autofocus -- Intentional: focus new item input when add form appears
                autoFocus
                onKeyDown={(e) => {
                  if (e.key === 'Enter') handleAdd()
                  if (e.key === 'Escape') setShowAddForm(false)
                }}
              />
            </div>
            <button
              type="button"
              onClick={handleAdd}
              disabled={disabled || !newItemName.trim() || !!items[newItemName.trim()]}
              className="btn btn-primary btn-sm"
            >
              Create
            </button>
            <button
              type="button"
              onClick={() => {
                setShowAddForm(false)
                setNewItemName('')
              }}
              className="btn btn-ghost btn-sm"
            >
              Cancel
            </button>
          </div>
          {items[newItemName.trim()] && (
            <p className="text-error text-sm mt-1">An agent with this name already exists</p>
          )}
        </div>
      )}

      {/* Existing items */}
      <div className="space-y-2">
        {itemNames.length === 0 && !showAddForm && (
          <div className="text-center py-6 text-base-content/50 bg-base-200 rounded-lg">
            No custom agents configured
          </div>
        )}

        {itemNames.map((name) => {
          const isExpanded = expandedItems.has(name)
          const item = items[name]
          const extends_ = (item as Record<string, unknown>)?.extends as string

          return (
            <div key={name} className="card bg-base-200">
              {/* Header */}
              <button
                type="button"
                className="flex items-center gap-2 p-3 w-full text-left hover:bg-base-300 rounded-t-lg transition-colors"
                onClick={() => toggleExpanded(name)}
                aria-expanded={isExpanded}
                aria-label={`${name} agent settings`}
              >
                <svg
                  aria-hidden="true"
                  className={`w-4 h-4 transition-transform ${isExpanded ? 'rotate-90' : ''}`}
                  fill="none"
                  viewBox="0 0 24 24"
                  stroke="currentColor"
                >
                  <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M9 5l7 7-7 7" />
                </svg>
                <span className="font-mono font-medium flex-1">{name}</span>
                {extends_ && <span className="badge badge-ghost badge-sm">extends {extends_}</span>}
                <button
                  type="button"
                  onClick={(e) => {
                    e.stopPropagation()
                    handleRemove(name)
                  }}
                  disabled={disabled}
                  className="btn btn-ghost btn-xs btn-square text-error"
                  aria-label={`Delete ${name} agent`}
                >
                  <svg aria-hidden="true" className="w-4 h-4" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                    <path
                      strokeLinecap="round"
                      strokeLinejoin="round"
                      strokeWidth={2}
                      d="M19 7l-.867 12.142A2 2 0 0116.138 21H7.862a2 2 0 01-1.995-1.858L5 7m5 4v6m4-6v6m1-10V4a1 1 0 00-1-1h-4a1 1 0 00-1 1v3M4 7h16"
                    />
                  </svg>
                </button>
              </button>

              {/* Expanded content */}
              {isExpanded && (
                <div className="p-3 pt-0 space-y-3 border-t border-base-300">
                  {field.itemSchema?.map((itemField) => renderItemField(name, itemField))}
                </div>
              )}
            </div>
          )
        })}
      </div>

      {error && (
        <label className="label">
          <span className="label-text-alt text-error">{error}</span>
        </label>
      )}
    </div>
  )
}
