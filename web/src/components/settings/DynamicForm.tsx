import type { Schema, Section } from '../../types/settings'
import { DynamicSection } from './DynamicSection'

interface DynamicFormProps {
  schema: Schema
  values: Record<string, unknown>
  errors?: Record<string, string>
  onChange: (path: string, value: unknown) => void
  category?: 'core' | 'providers' | 'features'
  disabled?: boolean
  defaultOpen?: boolean | 'first'
}

export function DynamicForm({
  schema,
  values,
  errors = {},
  onChange,
  category,
  disabled = false,
  defaultOpen = 'first',
}: DynamicFormProps) {
  // Filter sections by category if specified
  const sections = category ? schema.sections.filter((s: Section) => s.category === category) : schema.sections

  if (sections.length === 0) {
    return <div className="text-center py-8 text-base-content/50">No settings available</div>
  }

  return (
    <div className="space-y-4">
      {sections.map((section: Section, index: number) => (
        <DynamicSection
          key={section.id}
          section={section}
          values={values}
          errors={errors}
          onChange={onChange}
          defaultOpen={defaultOpen === 'first' ? index === 0 : Boolean(defaultOpen)}
          disabled={disabled}
        />
      ))}
    </div>
  )
}
