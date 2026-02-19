import { useEffect, useState } from 'react'
import { Database, RefreshCw, ChevronDown, ChevronUp, Table } from 'lucide-react'
import { api, type TableInfo } from '../api/client'
import StatusBadge from '../components/StatusBadge'

export default function Databases() {
  const [tables, setTables] = useState<TableInfo[]>([])
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState<string | null>(null)
  const [expanded, setExpanded] = useState<string | null>(null)
  const [items, setItems] = useState<Record<string, Record<string, unknown>[]>>({})
  const [itemsLoading, setItemsLoading] = useState<Record<string, boolean>>({})

  const load = async () => {
    try {
      setLoading(true)
      setError(null)
      const data = await api.listDatabases()
      setTables(data.tables ?? [])
    } catch (e) {
      setError((e as Error).message)
    } finally {
      setLoading(false)
    }
  }

  const loadItems = async (name: string) => {
    if (items[name]) return
    setItemsLoading((prev) => ({ ...prev, [name]: true }))
    try {
      const data = await api.getTableItems(name)
      setItems((prev) => ({ ...prev, [name]: data.items ?? [] }))
    } catch {
      setItems((prev) => ({ ...prev, [name]: [] }))
    } finally {
      setItemsLoading((prev) => ({ ...prev, [name]: false }))
    }
  }

  const toggleExpand = (name: string) => {
    if (expanded === name) {
      setExpanded(null)
    } else {
      setExpanded(name)
      loadItems(name)
    }
  }

  useEffect(() => { load() }, [])

  if (loading) return <Skeleton />

  if (error) {
    return (
      <div className="rounded-xl bg-red-500/10 border border-red-500/20 p-4 text-sm text-red-400">
        {error}
      </div>
    )
  }

  return (
    <div className="space-y-4">
      <div className="flex items-center justify-between">
        <div>
          <h1 className="text-xl font-bold text-white">DynamoDB Tables</h1>
          <p className="text-sm text-gray-500 mt-0.5">{tables.length} tables</p>
        </div>
        <button
          onClick={() => { setItems({}); load() }}
          className="flex items-center gap-2 px-3 py-1.5 rounded-lg bg-gray-800 hover:bg-gray-700 text-sm text-gray-300 transition-colors"
        >
          <RefreshCw size={14} />
          Refresh
        </button>
      </div>

      <div className="space-y-2">
        {tables.map((t) => (
          <div key={t.name} className="bg-gray-900 border border-gray-800 rounded-xl overflow-hidden">
            <button
              onClick={() => toggleExpand(t.name)}
              className="w-full flex items-center gap-3 px-4 py-3 hover:bg-gray-800/50 transition-colors text-left"
            >
              <Database size={15} className="text-purple-400 shrink-0" />
              <div className="flex-1 min-w-0">
                <p className="text-sm font-medium text-white truncate font-mono">{t.name}</p>
                <p className="text-xs text-gray-500 mt-0.5">
                  {t.item_count.toLocaleString()} items · {formatBytes(t.size_bytes)}
                  {t.gsi_count > 0 && ` · ${t.gsi_count} GSI`}
                </p>
              </div>
              <div className="flex items-center gap-3">
                <StatusBadge status={t.status} />
                {expanded === t.name ? (
                  <ChevronUp size={14} className="text-gray-500" />
                ) : (
                  <ChevronDown size={14} className="text-gray-500" />
                )}
              </div>
            </button>

            {expanded === t.name && (
              <div className="border-t border-gray-800 p-4 space-y-4">
                {/* Keys */}
                <div className="flex gap-4">
                  {t.key_schema?.map((k) => (
                    <div key={k.name} className="bg-gray-800 rounded-lg px-3 py-1.5">
                      <p className="text-xs text-gray-500">{k.type}</p>
                      <p className="text-xs font-mono text-gray-200">{k.name}</p>
                    </div>
                  ))}
                  <div className="bg-gray-800 rounded-lg px-3 py-1.5">
                    <p className="text-xs text-gray-500">Billing</p>
                    <p className="text-xs font-mono text-gray-200">{t.billing_mode || 'PAY_PER_REQUEST'}</p>
                  </div>
                </div>

                {/* Items preview */}
                <div>
                  <div className="flex items-center justify-between mb-2">
                    <p className="text-xs font-medium text-gray-400 flex items-center gap-1.5">
                      <Table size={11} />
                      Items preview (up to 50)
                    </p>
                    <button
                      onClick={() => {
                        setItems((prev) => { const n = { ...prev }; delete n[t.name]; return n })
                        loadItems(t.name)
                      }}
                      className="text-xs text-gray-500 hover:text-gray-300 flex items-center gap-1"
                    >
                      <RefreshCw size={11} />
                      Reload
                    </button>
                  </div>
                  {itemsLoading[t.name] ? (
                    <div className="h-32 bg-gray-950 rounded-lg border border-gray-800 flex items-center justify-center">
                      <span className="text-xs text-gray-600 animate-pulse">Loading…</span>
                    </div>
                  ) : (
                    <ItemsTable items={items[t.name] ?? []} />
                  )}
                </div>
              </div>
            )}
          </div>
        ))}
      </div>
    </div>
  )
}

function ItemsTable({ items }: { items: Record<string, unknown>[] }) {
  if (items.length === 0) {
    return (
      <div className="h-20 bg-gray-950 rounded-lg border border-gray-800 flex items-center justify-center">
        <span className="text-xs text-gray-600">No items</span>
      </div>
    )
  }

  const keys = Array.from(new Set(items.flatMap(Object.keys))).slice(0, 8)

  return (
    <div className="bg-gray-950 rounded-lg border border-gray-800 overflow-auto max-h-64">
      <table className="w-full text-xs">
        <thead className="sticky top-0 bg-gray-900">
          <tr>
            {keys.map((k) => (
              <th key={k} className="px-3 py-2 text-left text-gray-500 font-medium border-b border-gray-800 font-mono">
                {k}
              </th>
            ))}
          </tr>
        </thead>
        <tbody>
          {items.map((item, i) => (
            <tr key={i} className="border-b border-gray-800/50 hover:bg-gray-900/50">
              {keys.map((k) => (
                <td key={k} className="px-3 py-1.5 text-gray-300 font-mono max-w-xs truncate">
                  {formatCell(item[k])}
                </td>
              ))}
            </tr>
          ))}
        </tbody>
      </table>
    </div>
  )
}

function formatCell(v: unknown): string {
  if (v === null || v === undefined) return '—'
  if (typeof v === 'object') return JSON.stringify(v)
  return String(v)
}

function formatBytes(bytes: number): string {
  if (bytes < 1024) return `${bytes}B`
  if (bytes < 1024 * 1024) return `${(bytes / 1024).toFixed(1)}KB`
  return `${(bytes / 1024 / 1024).toFixed(1)}MB`
}

function Skeleton() {
  return (
    <div className="space-y-4">
      <div className="h-8 w-48 bg-gray-800 rounded-lg animate-pulse" />
      {Array.from({ length: 4 }).map((_, i) => (
        <div key={i} className="h-14 bg-gray-900 border border-gray-800 rounded-xl animate-pulse" />
      ))}
    </div>
  )
}
