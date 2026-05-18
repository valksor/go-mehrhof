import { useEffect, useState, useCallback } from 'react'
import { useProjectStore } from '../stores/projectStore'

interface ChecklistItem {
  text: string
  checked: boolean
}

interface ChecklistWidgetProps {
  embedded?: boolean
}

export function ChecklistWidget({ embedded = false }: ChecklistWidgetProps) {
  const { connected, state } = useProjectStore()
  const client = useProjectStore(s => s.client)
  const [items, setItems] = useState<ChecklistItem[]>([])
  const [loading, setLoading] = useState(false)
  const [toggleIndex, setToggleIndex] = useState<number | null>(null)

  const loadChecklist = useCallback(async () => {
    if (!connected || !client) return
    try {
      const result = await client.call<{ items: ChecklistItem[] }>('review.checklist.get', {})
      setItems(result.items || [])
    } catch {
      setItems([])
    }
  }, [connected, client])

  useEffect(() => {
    if (connected && client && (state === 'reviewing' || state === 'implemented')) {
      // eslint-disable-next-line react-hooks/set-state-in-effect -- load/poll idiom: setState lives in the async callback, not synchronous in the effect body
      void loadChecklist()
    }
  }, [connected, client, state, loadChecklist])

  const toggleItem = async (index: number, currentlyChecked: boolean) => {
    if (!client || !connected) return
    setToggleIndex(index)
    setLoading(true)
    try {
      const method = currentlyChecked ? 'review.checklist.uncheck' : 'review.checklist.check'
      await client.call(method, { index })
      await loadChecklist()
    } catch {
      // silently fail
    } finally {
      setLoading(false)
      setToggleIndex(null)
    }
  }

  if (state !== 'reviewing' && state !== 'implemented') return null

  const checkedCount = items.filter(i => i.checked).length
  const totalCount = items.length

  const content = (
    <div className="space-y-2">
      {/* Progress indicator */}
      {totalCount > 0 && (
        <div className="flex items-center gap-2">
          <progress
            className="progress progress-primary flex-1"
            value={checkedCount}
            max={totalCount}
          />
          <span className="text-xs text-base-content/60 font-mono whitespace-nowrap">
            {checkedCount}/{totalCount}
          </span>
        </div>
      )}

      {/* Checklist items */}
      {totalCount > 0 ? (
        <ul className="space-y-1">
          {items.map((item, index) => (
            <li key={index} className="flex items-start gap-2">
              <label className="flex items-start gap-2 cursor-pointer w-full py-0.5">
                <input
                  type="checkbox"
                  className="checkbox checkbox-sm checkbox-primary mt-0.5"
                  checked={item.checked}
                  onChange={() => toggleItem(index, item.checked)}
                  disabled={loading && toggleIndex === index}
                />
                <span className={`text-sm leading-relaxed ${item.checked ? 'line-through text-base-content/50' : 'text-base-content'}`}>
                  {item.text}
                </span>
              </label>
              {loading && toggleIndex === index && (
                <span className="loading loading-spinner loading-xs mt-1 flex-shrink-0" />
              )}
            </li>
          ))}
        </ul>
      ) : (
        <p className="text-xs text-base-content/50">No checklist items</p>
      )}
    </div>
  )

  if (embedded) {
    return content
  }

  return (
    <section className="card bg-base-200">
      <div className="card-body">
        <h2 className="card-title text-base-content flex items-center gap-2">
          Review Checklist
          {totalCount > 0 && (
            <span className="badge badge-sm badge-ghost">{checkedCount}/{totalCount}</span>
          )}
        </h2>
        {content}
      </div>
    </section>
  )
}
