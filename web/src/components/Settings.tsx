import { useState, useEffect, useCallback } from 'react'
import { useGlobalStore } from '../stores/globalStore'
import { useDebugStore } from '../stores/debugStore'
import { DynamicForm } from './settings/DynamicForm'
import { setPath, isMaskedToken } from '../lib/schemaUtils'
import { AccessibleModal } from './ui/AccessibleModal'
import { ProviderTestButtons } from './settings/ProviderTestButtons'
import { ConfigValidation } from './settings/ConfigValidation'
import { ConfigCheckButton } from './settings/ConfigCheckButton'
import { NotifyTestButton } from './settings/NotifyTestButton'
import type { Schema, SettingsResponse, Scope } from '../types/settings'

interface SettingsProps {
  isOpen: boolean
  onClose: () => void
  defaultScope?: 'global' | 'project'
}

export function Settings({ isOpen, onClose, defaultScope }: SettingsProps) {
  const { client, selectedProject, loading: storeLoading } = useGlobalStore()

  // Default to project scope if a project is selected, otherwise global
  const initialScope = defaultScope ?? (selectedProject ? 'project' : 'global')
  const [scope, setScope] = useState<Scope>(initialScope)

  // Reset scope when modal opens
  useEffect(() => {
    if (isOpen) {
      // eslint-disable-next-line react-hooks/set-state-in-effect -- load/poll idiom: setState lives in the async callback, not synchronous in the effect body
      setScope(defaultScope ?? (selectedProject ? 'project' : 'global'))
    }
  }, [isOpen, defaultScope, selectedProject])
  const [schema, setSchema] = useState<Schema | null>(null)
  const [effectiveSettings, setEffectiveSettings] = useState<Record<string, unknown> | null>(null)
  const [pendingChanges, setPendingChanges] = useState<Record<string, unknown>>({})
  const [loading, setLoading] = useState(false)
  const [saving, setSaving] = useState(false)
  const [error, setError] = useState<string | null>(null)

  // Load settings when modal opens
  const loadSettings = useCallback(async () => {
    if (!client || !isOpen) return

    setLoading(true)
    setError(null)

    try {
      const result = await client.call<SettingsResponse>('settings.get', {
        project_path: selectedProject?.path
      })

      setSchema(result.schema)
      setEffectiveSettings(result.effective)
      setPendingChanges({})
    } catch (err) {
      console.error('[Settings] Error loading:', err)
      setError(err instanceof Error ? err.message : 'Failed to load settings')
    } finally {
      setLoading(false)
    }
  }, [client, isOpen, selectedProject])

  useEffect(() => {
    // eslint-disable-next-line react-hooks/set-state-in-effect -- load/poll idiom: setState lives in the async callback, not synchronous in the effect body
    void loadSettings()
  }, [loadSettings])

  // Get values for display - use effective settings (includes defaults)
  // with pending changes merged on top
  const getCurrentValues = (): Record<string, unknown> => {
    let values = effectiveSettings ? { ...effectiveSettings } : {}
    for (const [path, value] of Object.entries(pendingChanges)) {
      values = setPath(values, path, value)
    }
    return values
  }

  // Handle field change
  const handleChange = (path: string, value: unknown) => {
    // Skip masked tokens (user didn't actually change them)
    if (isMaskedToken(value)) return

    setPendingChanges(prev => ({
      ...prev,
      [path]: value
    }))
  }

  // Save changes
  const handleSave = async () => {
    if (!client || Object.keys(pendingChanges).length === 0) return

    setSaving(true)
    setError(null)

    try {
      await client.call('settings.set', {
        scope,
        values: pendingChanges,
        project_path: selectedProject?.path
      })

      // Reload settings to get updated values
      await loadSettings()
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to save settings')
    } finally {
      setSaving(false)
    }
  }

  // Check if there are unsaved changes
  const hasChanges = Object.keys(pendingChanges).length > 0

  const actions = (
    <>
      <button onClick={onClose} className="btn btn-ghost">
        Cancel
      </button>
      <button
        onClick={handleSave}
        disabled={saving || storeLoading || !hasChanges}
        className="btn btn-primary"
      >
        {saving ? (
          <span className="loading loading-spinner loading-sm"></span>
        ) : (
          <svg aria-hidden="true" className="w-4 h-4" fill="none" viewBox="0 0 24 24" stroke="currentColor">
            <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M5 13l4 4L19 7" />
          </svg>
        )}
        Save {scope === 'global' ? 'Global' : 'Project'}
      </button>
    </>
  )

  return (
    <AccessibleModal isOpen={isOpen} onClose={onClose} title="Settings" size="2xl" actions={actions}>
      {/* Scope Tabs */}
      <div role="tablist" className="tabs tabs-lifted mb-4">
        <button
          role="tab"
          aria-selected={scope === 'global'}
          className={`tab gap-2 ${scope === 'global' ? 'tab-active [--tab-bg:var(--b1)] [--tab-border-color:var(--b3)]' : ''}`}
          onClick={() => {
            setScope('global')
            setPendingChanges({})
          }}
        >
          <svg aria-hidden="true" className="w-4 h-4" fill="none" viewBox="0 0 24 24" stroke="currentColor">
            <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M3.055 11H5a2 2 0 012 2v1a2 2 0 002 2 2 2 0 012 2v2.945M8 3.935V5.5A2.5 2.5 0 0010.5 8h.5a2 2 0 012 2 2 2 0 104 0 2 2 0 012-2h1.064M15 20.488V18a2 2 0 012-2h3.064M21 12a9 9 0 11-18 0 9 9 0 0118 0z" />
          </svg>
          Global
        </button>
        <button
          role="tab"
          aria-selected={scope === 'project'}
          className={`tab gap-2 ${scope === 'project' ? 'tab-active [--tab-bg:var(--b1)] [--tab-border-color:var(--b3)]' : ''} ${!selectedProject ? 'tab-disabled' : ''}`}
          onClick={() => {
            if (selectedProject) {
              setScope('project')
              setPendingChanges({})
            }
          }}
          disabled={!selectedProject}
        >
          <svg aria-hidden="true" className="w-4 h-4" fill="none" viewBox="0 0 24 24" stroke="currentColor">
            <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M3 7v10a2 2 0 002 2h14a2 2 0 002-2V9a2 2 0 00-2-2h-6l-2-2H5a2 2 0 00-2 2z" />
          </svg>
          Project
          {!selectedProject && <span className="text-xs opacity-50">(none)</span>}
        </button>
      </div>

      {/* Scope Info */}
      <div className="alert alert-info mb-4 py-2">
        <svg aria-hidden="true" className="w-4 h-4" fill="none" viewBox="0 0 24 24" stroke="currentColor">
          <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M13 16h-1v-4h-1m1-4h.01M21 12a9 9 0 11-18 0 9 9 0 0118 0z" />
        </svg>
        <span className="text-sm">
          {scope === 'global'
            ? 'Global settings apply to all projects unless overridden.'
            : 'Project settings override global settings for this project only.'}
        </span>
      </div>

      {/* Error */}
      {error && (
        <div className="alert alert-error mb-4 py-2" role="alert">
          <svg aria-hidden="true" className="w-4 h-4" fill="none" viewBox="0 0 24 24" stroke="currentColor">
            <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M12 8v4m0 4h.01M21 12a9 9 0 11-18 0 9 9 0 0118 0z" />
          </svg>
          <span className="text-sm">{error}</span>
        </div>
      )}

      {/* Content */}
      <div className="flex-1 overflow-y-auto">
        {loading ? (
          <div className="flex items-center justify-center py-12" role="status" aria-label="Loading settings">
            <span className="loading loading-spinner loading-lg" aria-hidden="true"></span>
            <span className="sr-only">Loading settings...</span>
          </div>
        ) : schema ? (
          <>
            <DynamicForm
              schema={schema}
              values={getCurrentValues()}
              onChange={handleChange}
              disabled={saving}
              defaultOpen="first"
            />
            {scope === 'global' && (
              <>
                <ProviderTestButtons />
                <NotifyTestButton />
                <ConfigValidation />
                <ConfigCheckButton />
              </>
            )}
          </>
        ) : (
          <div className="text-center py-8 text-base-content/50">
            No settings schema available
          </div>
        )}
      </div>

      {/* Footer: Config path + debug toggle */}
      <div className="border-t border-base-300 pt-3 mt-4 flex flex-col sm:flex-row items-start sm:items-center justify-between gap-2">
        <span className="text-xs text-base-content/40 font-mono select-all">
          Config: ~/.valksor/kvelmo/kvelmo.yaml
        </span>
        <DebugToggle />
      </div>
    </AccessibleModal>
  )
}

function DebugToggle() {
  const { enabled, setEnabled } = useDebugStore()

  return (
    <label className="flex items-center gap-2 cursor-pointer">
      <span className="text-xs text-base-content/50">Debug mode</span>
      <input
        type="checkbox"
        className="toggle toggle-xs toggle-primary"
        checked={enabled}
        onChange={(e) => setEnabled(e.target.checked)}
      />
    </label>
  )
}
