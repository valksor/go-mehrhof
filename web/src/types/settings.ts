// Schema types for dynamic form rendering
// These match the schema structure from Go, NOT the settings data structure

export type FieldType =
  | 'string'
  | 'boolean'
  | 'number'
  | 'select'
  | 'textarea'
  | 'password'
  | 'tags'
  | 'keyvalue'
  | 'list'

export interface SelectOption {
  value: string
  label: string
}

export interface ValidationRules {
  required?: boolean
  min?: number
  max?: number
  maxLength?: number
  pattern?: string
  patternMessage?: string
}

export interface Condition {
  field: string
  equals?: unknown
  notEquals?: unknown
}

export interface Field {
  path: string
  type: FieldType
  label: string
  description?: string
  placeholder?: string
  default?: unknown
  options?: SelectOption[]
  multiple?: boolean // True for multiselect fields (renders checkboxes)
  itemSchema?: Field[] // For list type - schema of each list item
  validation?: ValidationRules
  sensitive?: boolean
  envVar?: string
  helpUrl?: string // Link to help page (e.g., token setup)
  showWhen?: Condition
  advanced?: boolean
}

export interface Section {
  id: string
  title: string
  description?: string
  icon?: string
  category: 'core' | 'providers' | 'features'
  fields: Field[]
}

export interface Schema {
  version: string
  sections: Section[]
}

export type Scope = 'global' | 'project'

// Response from settings.get - values are generic objects, schema drives UI
export interface SettingsResponse {
  schema: Schema
  effective: Record<string, unknown> | null
  global: Record<string, unknown> | null
  project: Record<string, unknown> | null
}
