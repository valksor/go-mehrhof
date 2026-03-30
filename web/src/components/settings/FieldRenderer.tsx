import type { Field, SelectOption } from '../../types/settings'
import { KeyValueField } from './KeyValueField'
import { ListField } from './ListField'

interface FieldRendererProps {
  field: Field
  value: unknown
  error?: string
  onChange: (value: unknown) => void
  disabled?: boolean
}

export function FieldRenderer({ field, value, error, onChange, disabled }: FieldRendererProps) {
  const inputClass = `input input-bordered w-full ${error ? 'input-error' : ''}`
  const selectClass = `select select-bordered w-full ${error ? 'select-error' : ''}`
  const fieldId = `field-${field.path.replace(/\./g, '-')}`
  const descId = `${fieldId}-desc`
  const errorId = `${fieldId}-error`

  // Build aria-describedby from available elements
  const describedBy = [
    field.description ? descId : null,
    error ? errorId : null,
  ].filter(Boolean).join(' ') || undefined

  switch (field.type) {
    case 'string':
      return (
        <div className="form-control">
          <label className="label" htmlFor={fieldId}>
            <span className="label-text">{field.label}</span>
          </label>
          <input
            id={fieldId}
            type="text"
            value={(value as string) ?? ''}
            onChange={e => onChange(e.target.value)}
            placeholder={field.placeholder}
            disabled={disabled}
            aria-invalid={error ? true : undefined}
            aria-describedby={describedBy}
            className={`${inputClass} font-mono text-sm`}
          />
          {field.description && (
            <label className="label">
              <span id={descId} className="label-text-alt text-base-content/50">{field.description}</span>
            </label>
          )}
          {error && (
            <label className="label">
              <span id={errorId} className="label-text-alt text-error" role="alert">{error}</span>
            </label>
          )}
        </div>
      )

    case 'password':
      return (
        <div className="form-control">
          <label className="label" htmlFor={fieldId}>
            <span className="label-text">{field.label}</span>
            <span className="label-text-alt flex gap-2 items-center">
              {field.helpUrl && (
                <a
                  href={field.helpUrl}
                  target="_blank"
                  rel="noopener noreferrer"
                  className="link link-primary text-xs"
                >
                  Get token
                </a>
              )}
              {field.sensitive && (
                <span className="badge badge-ghost badge-sm">Stored in .env</span>
              )}
            </span>
          </label>
          <input
            id={fieldId}
            type="password"
            value={(value as string) ?? ''}
            onChange={e => onChange(e.target.value)}
            placeholder={field.placeholder}
            disabled={disabled}
            aria-invalid={error ? true : undefined}
            aria-describedby={describedBy}
            className={`${inputClass} font-mono text-sm`}
          />
          {field.description && (
            <label className="label">
              <span id={descId} className="label-text-alt text-base-content/50">{field.description}</span>
            </label>
          )}
          {error && (
            <label className="label">
              <span id={errorId} className="label-text-alt text-error" role="alert">{error}</span>
            </label>
          )}
        </div>
      )

    case 'textarea':
      return (
        <div className="form-control">
          <label className="label" htmlFor={fieldId}>
            <span className="label-text">{field.label}</span>
          </label>
          <textarea
            id={fieldId}
            value={(value as string) ?? ''}
            onChange={e => onChange(e.target.value)}
            placeholder={field.placeholder}
            disabled={disabled}
            aria-invalid={error ? true : undefined}
            aria-describedby={describedBy}
            className={`textarea textarea-bordered w-full font-mono text-sm ${error ? 'textarea-error' : ''}`}
            rows={4}
          />
          {field.description && (
            <label className="label">
              <span id={descId} className="label-text-alt text-base-content/50">{field.description}</span>
            </label>
          )}
          {error && (
            <label className="label">
              <span id={errorId} className="label-text-alt text-error" role="alert">{error}</span>
            </label>
          )}
        </div>
      )

    case 'number':
      return (
        <div className="form-control">
          <label className="label" htmlFor={fieldId}>
            <span className="label-text">{field.label}</span>
          </label>
          <input
            id={fieldId}
            type="number"
            value={(value as number) ?? ''}
            onChange={e => onChange(e.target.value === '' ? undefined : Number(e.target.value))}
            placeholder={field.placeholder}
            disabled={disabled}
            min={field.validation?.min}
            max={field.validation?.max}
            aria-invalid={error ? true : undefined}
            aria-describedby={describedBy}
            className={inputClass}
          />
          {field.description && (
            <label className="label">
              <span id={descId} className="label-text-alt text-base-content/50">{field.description}</span>
            </label>
          )}
          {error && (
            <label className="label">
              <span id={errorId} className="label-text-alt text-error" role="alert">{error}</span>
            </label>
          )}
        </div>
      )

    case 'boolean':
      return (
        <div className="form-control">
          <label className="label cursor-pointer justify-start gap-3" htmlFor={fieldId}>
            <input
              id={fieldId}
              type="checkbox"
              checked={(value as boolean) ?? false}
              onChange={e => onChange(e.target.checked)}
              disabled={disabled}
              aria-invalid={error ? true : undefined}
              aria-describedby={describedBy}
              className="checkbox checkbox-primary"
            />
            <span className="label-text">{field.label}</span>
          </label>
          {field.description && (
            <label className="label pt-0">
              <span id={descId} className="label-text-alt text-base-content/50">{field.description}</span>
            </label>
          )}
          {error && (
            <label className="label">
              <span id={errorId} className="label-text-alt text-error" role="alert">{error}</span>
            </label>
          )}
        </div>
      )

    case 'select':
      // Multiselect: render as checkboxes
      if (field.multiple) {
        const selectedValues = Array.isArray(value) ? (value as string[]) : []
        const handleCheckboxChange = (optValue: string, checked: boolean) => {
          if (checked) {
            onChange([...selectedValues, optValue])
          } else {
            onChange(selectedValues.filter(v => v !== optValue))
          }
        }
        return (
          <div className="form-control">
            <label className="label">
              <span className="label-text">{field.label}</span>
            </label>
            <div className="flex flex-wrap gap-4">
              {field.options?.map((opt: SelectOption) => (
                <label key={opt.value} className="label cursor-pointer gap-2 justify-start">
                  <input
                    type="checkbox"
                    checked={selectedValues.includes(opt.value)}
                    onChange={e => handleCheckboxChange(opt.value, e.target.checked)}
                    disabled={disabled}
                    className="checkbox checkbox-primary checkbox-sm"
                  />
                  <span className="label-text">{opt.label}</span>
                </label>
              ))}
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
      // Single select: render as dropdown
      return (
        <div className="form-control">
          <label className="label" htmlFor={fieldId}>
            <span className="label-text">{field.label}</span>
          </label>
          <select
            id={fieldId}
            value={(value as string) ?? ''}
            onChange={e => onChange(e.target.value)}
            disabled={disabled}
            aria-invalid={error ? true : undefined}
            aria-describedby={describedBy}
            className={selectClass}
          >
            <option value="">Select...</option>
            {field.options?.map((opt: SelectOption) => (
              <option key={opt.value} value={opt.value}>
                {opt.label}
              </option>
            ))}
          </select>
          {field.description && (
            <label className="label">
              <span id={descId} className="label-text-alt text-base-content/50">{field.description}</span>
            </label>
          )}
          {error && (
            <label className="label">
              <span id={errorId} className="label-text-alt text-error" role="alert">{error}</span>
            </label>
          )}
        </div>
      )

    case 'tags':
      // Render as comma-separated input for simplicity
      return (
        <div className="form-control">
          <label className="label" htmlFor={fieldId}>
            <span className="label-text">{field.label}</span>
          </label>
          <input
            id={fieldId}
            type="text"
            value={Array.isArray(value) ? (value as string[]).join(', ') : ''}
            onChange={e => {
              const tags = e.target.value.split(',').map(t => t.trim()).filter(Boolean)
              onChange(tags)
            }}
            placeholder={field.placeholder || 'Enter comma-separated values'}
            disabled={disabled}
            aria-invalid={error ? true : undefined}
            aria-describedby={describedBy}
            className={`${inputClass} font-mono text-sm`}
          />
          {field.description && (
            <label className="label">
              <span id={descId} className="label-text-alt text-base-content/50">{field.description}</span>
            </label>
          )}
          {error && (
            <label className="label">
              <span id={errorId} className="label-text-alt text-error" role="alert">{error}</span>
            </label>
          )}
        </div>
      )

    case 'keyvalue':
      return (
        <KeyValueField
          field={field}
          value={value as Record<string, string>}
          onChange={onChange}
          disabled={disabled}
          error={error}
        />
      )

    case 'list':
      return (
        <ListField
          field={field}
          value={value as Record<string, Record<string, unknown>>}
          onChange={onChange}
          disabled={disabled}
          error={error}
        />
      )

    default:
      return (
        <div className="form-control">
          <label className="label" htmlFor={fieldId}>
            <span className="label-text">{field.label}</span>
          </label>
          <input
            id={fieldId}
            type="text"
            value={value == null ? '' : String(value as string | number)}
            onChange={e => onChange(e.target.value)}
            placeholder={field.placeholder}
            disabled={disabled}
            aria-invalid={error ? true : undefined}
            aria-describedby={describedBy}
            className={inputClass}
          />
          {field.description && (
            <label className="label">
              <span id={descId} className="label-text-alt text-base-content/50">{field.description}</span>
            </label>
          )}
          {error && (
            <label className="label">
              <span id={errorId} className="label-text-alt text-error" role="alert">{error}</span>
            </label>
          )}
        </div>
      )
  }
}
