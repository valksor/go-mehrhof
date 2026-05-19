import { useState, useCallback, useEffect } from 'react'
import { useGlobalStore } from '../stores/globalStore'
import { AccessibleModal } from './ui/AccessibleModal'

interface CatalogPanelProps {
  isOpen: boolean
  onClose: () => void
}

interface Template {
  name: string
  description: string
  source: string
  agent: string
  tags: string[]
}

type CatalogView = 'list' | 'detail'

export function CatalogPanel({ isOpen, onClose }: CatalogPanelProps) {
  const { client, connected } = useGlobalStore()

  const [templates, setTemplates] = useState<Template[]>([])
  const [loading, setLoading] = useState(false)
  const [error, setError] = useState<string | null>(null)

  const [view, setView] = useState<CatalogView>('list')
  const [selectedTemplate, setSelectedTemplate] = useState<Template | null>(null)
  const [detailLoading, setDetailLoading] = useState(false)

  const [importPath, setImportPath] = useState('')
  const [importing, setImporting] = useState(false)
  const [importError, setImportError] = useState<string | null>(null)
  const [importSuccess, setImportSuccess] = useState(false)

  const loadTemplates = useCallback(async () => {
    if (!client || !connected) return

    setLoading(true)
    setError(null)

    try {
      const result = await client.call<{ templates: Template[] }>('catalog.list', {})
      setTemplates(result.templates || [])
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to load templates')
      setTemplates([])
    } finally {
      setLoading(false)
    }
  }, [client, connected])

  // Auto-load when panel opens
  useEffect(() => {
    if (isOpen && connected) {
      // eslint-disable-next-line react-hooks/set-state-in-effect -- load/poll idiom: setState lives in the async callback, not synchronous in the effect body
      void loadTemplates()
      setView('list')
      setSelectedTemplate(null)
      setImportPath('')
      setImportError(null)
      setImportSuccess(false)
    }
  }, [isOpen, connected, loadTemplates])

  const handleSelectTemplate = useCallback(
    async (template: Template) => {
      if (!client || !connected) return

      setDetailLoading(true)
      setView('detail')

      try {
        const result = await client.call<Template>('catalog.get', { name: template.name })
        setSelectedTemplate(result)
      } catch {
        setSelectedTemplate(template)
      } finally {
        setDetailLoading(false)
      }
    },
    [client, connected],
  )

  const handleBack = () => {
    setView('list')
    setSelectedTemplate(null)
  }

  const handleImport = useCallback(async () => {
    if (!client || !connected || !importPath.trim()) return

    setImporting(true)
    setImportError(null)
    setImportSuccess(false)

    try {
      await client.call<{ success: boolean }>('catalog.import', { path: importPath.trim() })
      setImportSuccess(true)
      setImportPath('')
      await loadTemplates()
    } catch (err) {
      setImportError(err instanceof Error ? err.message : 'Import failed')
    } finally {
      setImporting(false)
    }
  }, [client, connected, importPath, loadTemplates])

  const handleImportKeyDown = (e: React.KeyboardEvent) => {
    if (e.key === 'Enter') {
      void handleImport()
    }
  }

  return (
    <AccessibleModal isOpen={isOpen} onClose={onClose} title="Task Templates" size="3xl">
      <div className="max-h-[70vh] flex flex-col">
        {/* Error */}
        {error && (
          <div className="alert alert-error py-2 mb-4">
            <span className="text-sm">{error}</span>
          </div>
        )}

        {view === 'list' ? (
          <>
            {/* Import section */}
            <div className="flex gap-2 mb-4">
              <input
                type="text"
                value={importPath}
                onChange={(e) => {
                  setImportPath(e.target.value)
                  setImportSuccess(false)
                  setImportError(null)
                }}
                onKeyDown={handleImportKeyDown}
                placeholder="File path to import template..."
                aria-label="Template file path"
                className="input input-bordered input-sm flex-1"
              />
              <button
                onClick={handleImport}
                disabled={importing || !importPath.trim() || !connected}
                className="btn btn-primary btn-sm"
              >
                {importing ? (
                  <span className="loading loading-spinner loading-xs"></span>
                ) : (
                  <svg aria-hidden="true" className="w-4 h-4" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                    <path
                      strokeLinecap="round"
                      strokeLinejoin="round"
                      strokeWidth={2}
                      d="M4 16v1a3 3 0 003 3h10a3 3 0 003-3v-1m-4-8l-4-4m0 0L8 8m4-4v12"
                    />
                  </svg>
                )}
                Import
              </button>
            </div>

            {importError && (
              <div className="alert alert-error py-2 mb-4">
                <span className="text-sm">{importError}</span>
              </div>
            )}

            {importSuccess && (
              <div className="alert alert-success py-2 mb-4">
                <span className="text-sm">Template imported successfully</span>
              </div>
            )}

            {/* Template grid */}
            <div className="flex-1 overflow-y-auto">
              {loading ? (
                <div className="flex items-center justify-center py-12">
                  <span className="loading loading-spinner loading-lg text-primary"></span>
                </div>
              ) : templates.length === 0 ? (
                <div className="text-center py-12 text-base-content/50">
                  <svg
                    aria-hidden="true"
                    className="w-10 h-10 mx-auto mb-3 opacity-30"
                    fill="none"
                    viewBox="0 0 24 24"
                    stroke="currentColor"
                  >
                    <path
                      strokeLinecap="round"
                      strokeLinejoin="round"
                      strokeWidth={1.5}
                      d="M19 11H5m14 0a2 2 0 012 2v6a2 2 0 01-2 2H5a2 2 0 01-2-2v-6a2 2 0 012-2m14 0V9a2 2 0 00-2-2M5 11V9a2 2 0 012-2m0 0V5a2 2 0 012-2h6a2 2 0 012 2v2M7 7h10"
                    />
                  </svg>
                  <p>No templates available</p>
                  <p className="text-xs mt-2 text-base-content/40">Import a template file to get started</p>
                </div>
              ) : (
                <div className="grid grid-cols-1 sm:grid-cols-2 gap-3">
                  {templates.map((template) => (
                    <button
                      key={template.name}
                      onClick={() => handleSelectTemplate(template)}
                      className="card bg-base-200 border border-base-300 hover:border-primary/40 transition-colors text-left cursor-pointer"
                    >
                      <div className="card-body p-4">
                        <h3 className="font-semibold text-sm text-base-content">{template.name}</h3>
                        <p className="text-xs text-base-content/60 line-clamp-2">{template.description}</p>
                        {template.tags.length > 0 && (
                          <div className="flex flex-wrap gap-1 mt-2">
                            {template.tags.map((tag) => (
                              <span key={tag} className="badge badge-sm badge-ghost">
                                {tag}
                              </span>
                            ))}
                          </div>
                        )}
                      </div>
                    </button>
                  ))}
                </div>
              )}
            </div>
          </>
        ) : (
          /* Detail view */
          <div className="flex-1 overflow-y-auto">
            <button onClick={handleBack} className="btn btn-ghost btn-sm mb-4">
              <svg aria-hidden="true" className="w-4 h-4" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M15 19l-7-7 7-7" />
              </svg>
              Back to templates
            </button>

            {detailLoading ? (
              <div className="flex items-center justify-center py-12">
                <span className="loading loading-spinner loading-lg text-primary"></span>
              </div>
            ) : selectedTemplate ? (
              <div className="space-y-4">
                <div>
                  <h3 className="text-lg font-bold text-base-content">{selectedTemplate.name}</h3>
                  <p className="text-sm text-base-content/70 mt-1">{selectedTemplate.description}</p>
                </div>

                <div className="grid grid-cols-2 gap-3">
                  <div className="bg-base-200 rounded-lg p-3">
                    <div className="text-xs text-base-content/50 mb-1">Source</div>
                    <div className="text-sm font-mono">{selectedTemplate.source || 'N/A'}</div>
                  </div>
                  <div className="bg-base-200 rounded-lg p-3">
                    <div className="text-xs text-base-content/50 mb-1">Agent</div>
                    <div className="text-sm font-mono">{selectedTemplate.agent || 'N/A'}</div>
                  </div>
                </div>

                {selectedTemplate.tags.length > 0 && (
                  <div>
                    <div className="text-xs text-base-content/50 mb-2">Tags</div>
                    <div className="flex flex-wrap gap-1.5">
                      {selectedTemplate.tags.map((tag) => (
                        <span key={tag} className="badge badge-sm badge-outline">
                          {tag}
                        </span>
                      ))}
                    </div>
                  </div>
                )}
              </div>
            ) : null}
          </div>
        )}
      </div>
    </AccessibleModal>
  )
}
