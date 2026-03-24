import { useState, type ReactNode } from 'react'
import type { Section, Field } from '../../types/settings'
import { evaluateShowWhen, getPath } from '../../lib/schemaUtils'
import { getSectionFieldGroups } from '../../lib/settingsLayout'
import { FieldRenderer } from './FieldRenderer'

// Icon mapping for section icons
const iconMap: Record<string, ReactNode> = {
  bot: (
    <svg className="w-4 h-4" fill="none" viewBox="0 0 24 24" stroke="currentColor">
      <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M9.75 17L9 20l-1 1h8l-1-1-.75-3M3 13h18M5 17h14a2 2 0 002-2V5a2 2 0 00-2-2H5a2 2 0 00-2 2v10a2 2 0 002 2z" />
    </svg>
  ),
  plug: (
    <svg className="w-4 h-4" fill="none" viewBox="0 0 24 24" stroke="currentColor">
      <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M13 10V3L4 14h7v7l9-11h-7z" />
    </svg>
  ),
  github: (
    <svg className="w-4 h-4" fill="currentColor" viewBox="0 0 24 24">
      <path fillRule="evenodd" d="M12 2C6.477 2 2 6.484 2 12.017c0 4.425 2.865 8.18 6.839 9.504.5.092.682-.217.682-.483 0-.237-.008-.868-.013-1.703-2.782.605-3.369-1.343-3.369-1.343-.454-1.158-1.11-1.466-1.11-1.466-.908-.62.069-.608.069-.608 1.003.07 1.531 1.032 1.531 1.032.892 1.53 2.341 1.088 2.91.832.092-.647.35-1.088.636-1.338-2.22-.253-4.555-1.113-4.555-4.951 0-1.093.39-1.988 1.029-2.688-.103-.253-.446-1.272.098-2.65 0 0 .84-.27 2.75 1.026A9.564 9.564 0 0112 6.844c.85.004 1.705.115 2.504.337 1.909-1.296 2.747-1.027 2.747-1.027.546 1.379.202 2.398.1 2.651.64.7 1.028 1.595 1.028 2.688 0 3.848-2.339 4.695-4.566 4.943.359.309.678.92.678 1.855 0 1.338-.012 2.419-.012 2.747 0 .268.18.58.688.482A10.019 10.019 0 0022 12.017C22 6.484 17.522 2 12 2z" clipRule="evenodd" />
    </svg>
  ),
  gitlab: (
    <svg className="w-4 h-4" fill="currentColor" viewBox="0 0 24 24">
      <path d="M22.65 14.39L12 22.13 1.35 14.39a.84.84 0 01-.3-.94l1.22-3.78 2.44-7.51A.42.42 0 014.82 2a.43.43 0 01.58 0 .42.42 0 01.11.18l2.44 7.49h8.1l2.44-7.51A.42.42 0 0118.6 2a.43.43 0 01.58 0 .42.42 0 01.11.18l2.44 7.51L23 13.45a.84.84 0 01-.35.94z" />
    </svg>
  ),
  briefcase: (
    <svg className="w-4 h-4" fill="none" viewBox="0 0 24 24" stroke="currentColor">
      <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M21 13.255A23.931 23.931 0 0112 15c-3.183 0-6.22-.62-9-1.745M16 6V4a2 2 0 00-2-2h-4a2 2 0 00-2 2v2m4 6h.01M5 20h14a2 2 0 002-2V8a2 2 0 00-2-2H5a2 2 0 00-2 2v10a2 2 0 002 2z" />
    </svg>
  ),
  'git-branch': (
    <svg className="w-4 h-4" fill="none" viewBox="0 0 24 24" stroke="currentColor">
      <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M13 10V3L4 14h7v7l9-11h-7z" />
    </svg>
  ),
  users: (
    <svg className="w-4 h-4" fill="none" viewBox="0 0 24 24" stroke="currentColor">
      <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M12 4.354a4 4 0 110 5.292M15 21H3v-1a6 6 0 0112 0v1zm0 0h6v-1a6 6 0 00-9-5.197M13 7a4 4 0 11-8 0 4 4 0 018 0z" />
    </svg>
  ),
  wand: (
    <svg className="w-4 h-4" fill="none" viewBox="0 0 24 24" stroke="currentColor">
      <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M15.232 5.232l3.536 3.536m-2.036-5.036a2.5 2.5 0 113.536 3.536L6.5 21.036H3v-3.572L16.732 3.732z" />
    </svg>
  ),
}

interface DynamicSectionProps {
  section: Section
  values: Record<string, unknown>
  errors?: Record<string, string>
  onChange: (path: string, value: unknown) => void
  defaultOpen?: boolean
  disabled?: boolean
}

export function DynamicSection({
  section,
  values,
  errors = {},
  onChange,
  defaultOpen = false,
  disabled = false
}: DynamicSectionProps) {
  const [isOpen, setIsOpen] = useState(defaultOpen)

  // Filter visible fields based on showWhen conditions
  const visibleFields = section.fields.filter((field: Field) => {
    return evaluateShowWhen(field.showWhen, values)
  })

  if (visibleFields.length === 0) {
    return null
  }

  const icon = section.icon ? iconMap[section.icon] : null
  const fieldGroups = getSectionFieldGroups(section, visibleFields)

  return (
    <div className="collapse collapse-arrow bg-base-200 rounded-lg">
      <input
        type="checkbox"
        checked={isOpen}
        onChange={e => setIsOpen(e.target.checked)}
        className="min-h-0"
        aria-label={section.title}
      />
      <div className="collapse-title min-h-0 py-3 px-4 flex items-center gap-2">
        {icon && <span aria-hidden="true" className="text-primary">{icon}</span>}
        <span className="font-medium">{section.title}</span>
        {section.description && (
          <span className="text-base-content/50 text-sm hidden sm:inline">
            — {section.description}
          </span>
        )}
      </div>
      <div className="collapse-content px-4 pb-4">
        <div className="space-y-4 pt-2">
          {fieldGroups.map(group => (
            <section key={group.id} className="rounded-xl border border-base-300 bg-base-100/70 p-4 space-y-4">
              {(fieldGroups.length > 1 || group.badge || group.description) && (
                <div className="space-y-2">
                  <div className="flex flex-wrap items-center gap-2">
                    <h4 className="font-medium text-sm md:text-base">{group.title}</h4>
                    {group.badge && <span className="badge badge-outline badge-sm">{group.badge}</span>}
                  </div>
                  {group.description && (
                    <p className="text-sm text-base-content/60">{group.description}</p>
                  )}
                </div>
              )}

              <div className="space-y-4">
                {group.fields.map((field: Field) => (
                  <FieldRenderer
                    key={field.path}
                    field={field}
                    value={getPath(values, field.path)}
                    error={errors[field.path]}
                    onChange={value => onChange(field.path, value)}
                    disabled={disabled}
                  />
                ))}
              </div>
            </section>
          ))}
        </div>
      </div>
    </div>
  )
}
