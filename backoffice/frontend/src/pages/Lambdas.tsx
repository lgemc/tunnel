import { useEffect, useState } from 'react'
import { Zap, RefreshCw, ChevronDown, ChevronUp } from 'lucide-react'
import { api, type LambdaInfo, type LogEvent } from '../api/client'
import StatusBadge from '../components/StatusBadge'
import LogViewer from '../components/LogViewer'

export default function Lambdas() {
  const [lambdas, setLambdas] = useState<LambdaInfo[]>([])
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState<string | null>(null)
  const [expanded, setExpanded] = useState<string | null>(null)
  const [logs, setLogs] = useState<Record<string, LogEvent[]>>({})
  const [logsLoading, setLogsLoading] = useState<Record<string, boolean>>({})

  const load = async () => {
    try {
      setLoading(true)
      setError(null)
      const data = await api.listLambdas()
      setLambdas(data.lambdas ?? [])
    } catch (e) {
      setError((e as Error).message)
    } finally {
      setLoading(false)
    }
  }

  const loadLogs = async (name: string) => {
    if (logs[name]) return
    setLogsLoading((prev) => ({ ...prev, [name]: true }))
    try {
      const data = await api.getLambdaLogs(name, 100)
      setLogs((prev) => ({ ...prev, [name]: data.events ?? [] }))
    } catch {
      setLogs((prev) => ({ ...prev, [name]: [] }))
    } finally {
      setLogsLoading((prev) => ({ ...prev, [name]: false }))
    }
  }

  const toggleExpand = (name: string) => {
    if (expanded === name) {
      setExpanded(null)
    } else {
      setExpanded(name)
      loadLogs(name)
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
          <h1 className="text-xl font-bold text-white">Lambda Functions</h1>
          <p className="text-sm text-gray-500 mt-0.5">{lambdas.length} functions</p>
        </div>
        <button
          onClick={() => { setLogs({}); load() }}
          className="flex items-center gap-2 px-3 py-1.5 rounded-lg bg-gray-800 hover:bg-gray-700 text-sm text-gray-300 transition-colors"
        >
          <RefreshCw size={14} />
          Refresh
        </button>
      </div>

      <div className="space-y-2">
        {lambdas.map((fn) => (
          <div key={fn.name} className="bg-gray-900 border border-gray-800 rounded-xl overflow-hidden">
            {/* Header row */}
            <button
              onClick={() => toggleExpand(fn.name)}
              className="w-full flex items-center gap-3 px-4 py-3 hover:bg-gray-800/50 transition-colors text-left"
            >
              <Zap size={15} className="text-brand-400 shrink-0" />
              <div className="flex-1 min-w-0">
                <p className="text-sm font-medium text-white truncate">{fn.name}</p>
                <p className="text-xs text-gray-500 mt-0.5">
                  {fn.runtime} · {fn.memory_size_mb}MB · {fn.timeout_seconds}s timeout
                </p>
              </div>
              <div className="flex items-center gap-3">
                <StatusBadge status={fn.state} />
                <span className="text-xs text-gray-600">
                  {fn.code_size_bytes > 0 ? `${(fn.code_size_bytes / 1024).toFixed(0)}KB` : '—'}
                </span>
                {expanded === fn.name ? (
                  <ChevronUp size={14} className="text-gray-500" />
                ) : (
                  <ChevronDown size={14} className="text-gray-500" />
                )}
              </div>
            </button>

            {/* Expanded: details + logs */}
            {expanded === fn.name && (
              <div className="border-t border-gray-800 p-4 space-y-4">
                {/* Meta */}
                <div className="grid grid-cols-2 md:grid-cols-4 gap-3">
                  <MetaItem label="Handler" value={fn.handler} />
                  <MetaItem label="Last Modified" value={fn.last_modified ? new Date(fn.last_modified).toLocaleString() : '—'} />
                  <MetaItem label="Log Group" value={fn.log_group} mono />
                  <MetaItem
                    label="ARN"
                    value={fn.function_arn.split(':').slice(-2).join(':')}
                    mono
                  />
                </div>

                {/* Logs */}
                <div>
                  <div className="flex items-center justify-between mb-2">
                    <p className="text-xs font-medium text-gray-400">Recent Logs (last 30 min)</p>
                    <button
                      onClick={() => {
                        setLogs((prev) => { const n = { ...prev }; delete n[fn.name]; return n })
                        loadLogs(fn.name)
                      }}
                      className="text-xs text-gray-500 hover:text-gray-300 flex items-center gap-1"
                    >
                      <RefreshCw size={11} />
                      Reload
                    </button>
                  </div>
                  <LogViewer events={logs[fn.name] ?? []} loading={logsLoading[fn.name]} />
                </div>
              </div>
            )}
          </div>
        ))}
      </div>
    </div>
  )
}

function MetaItem({ label, value, mono }: { label: string; value: string; mono?: boolean }) {
  return (
    <div>
      <p className="text-xs text-gray-500 mb-0.5">{label}</p>
      <p className={`text-xs text-gray-300 truncate ${mono ? 'font-mono' : ''}`}>{value || '—'}</p>
    </div>
  )
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
