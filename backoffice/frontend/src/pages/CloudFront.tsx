import { useEffect, useState } from 'react'
import { Globe, RefreshCw, CheckCircle, XCircle } from 'lucide-react'
import { api, type DistributionInfo } from '../api/client'
import StatusBadge from '../components/StatusBadge'

export default function CloudFront() {
  const [distributions, setDistributions] = useState<DistributionInfo[]>([])
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState<string | null>(null)

  const load = async () => {
    try {
      setLoading(true)
      setError(null)
      const data = await api.getCloudFront()
      setDistributions(data.distributions ?? [])
    } catch (e) {
      setError((e as Error).message)
    } finally {
      setLoading(false)
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
          <h1 className="text-xl font-bold text-white">CloudFront</h1>
          <p className="text-sm text-gray-500 mt-0.5">{distributions.length} distributions</p>
        </div>
        <button
          onClick={load}
          className="flex items-center gap-2 px-3 py-1.5 rounded-lg bg-gray-800 hover:bg-gray-700 text-sm text-gray-300 transition-colors"
        >
          <RefreshCw size={14} />
          Refresh
        </button>
      </div>

      {distributions.length === 0 && (
        <div className="bg-gray-900 border border-gray-800 rounded-xl p-8 text-center">
          <Globe size={24} className="text-gray-700 mx-auto mb-2" />
          <p className="text-sm text-gray-500">No CloudFront distributions found</p>
        </div>
      )}

      <div className="space-y-3">
        {distributions.map((d) => (
          <div key={d.id} className="bg-gray-900 border border-gray-800 rounded-xl p-4">
            <div className="flex items-start justify-between gap-4 mb-3">
              <div className="flex items-center gap-3">
                <Globe size={16} className="text-sky-400 shrink-0 mt-0.5" />
                <div>
                  <p className="text-sm font-medium text-white font-mono">{d.domain_name}</p>
                  <p className="text-xs text-gray-500 mt-0.5">{d.id}</p>
                </div>
              </div>
              <div className="flex items-center gap-2 shrink-0">
                {d.enabled ? (
                  <CheckCircle size={14} className="text-emerald-400" />
                ) : (
                  <XCircle size={14} className="text-red-400" />
                )}
                <StatusBadge status={d.status} />
              </div>
            </div>

            {/* Comment */}
            {d.comment && (
              <p className="text-xs text-gray-400 mb-3 ml-7">{d.comment}</p>
            )}

            {/* Aliases */}
            {d.aliases?.length > 0 && (
              <div className="ml-7 mb-3 flex flex-wrap gap-1.5">
                {d.aliases.map((a) => (
                  <span key={a} className="inline-flex items-center px-2 py-0.5 rounded-md bg-sky-500/10 text-sky-400 text-xs font-mono ring-1 ring-sky-500/20">
                    {a}
                  </span>
                ))}
              </div>
            )}

            {/* Meta grid */}
            <div className="ml-7 grid grid-cols-2 md:grid-cols-4 gap-3">
              <MetaItem label="Price Class" value={d.price_class} />
              <MetaItem label="HTTP Version" value={d.http_version} />
              <MetaItem label="Origins" value={String(d.origin_count)} />
              <MetaItem label="Cache Behaviors" value={String(d.cache_behaviors_count)} />
              <MetaItem
                label="Last Modified"
                value={d.last_modified_time ? new Date(d.last_modified_time).toLocaleDateString() : '—'}
              />
            </div>
          </div>
        ))}
      </div>
    </div>
  )
}

function MetaItem({ label, value }: { label: string; value: string }) {
  return (
    <div>
      <p className="text-xs text-gray-500 mb-0.5">{label}</p>
      <p className="text-xs text-gray-300">{value || '—'}</p>
    </div>
  )
}

function Skeleton() {
  return (
    <div className="space-y-4">
      <div className="h-8 w-48 bg-gray-800 rounded-lg animate-pulse" />
      {Array.from({ length: 2 }).map((_, i) => (
        <div key={i} className="h-40 bg-gray-900 border border-gray-800 rounded-xl animate-pulse" />
      ))}
    </div>
  )
}
