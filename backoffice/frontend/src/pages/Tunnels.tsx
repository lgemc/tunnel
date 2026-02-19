import { useEffect, useState } from 'react'
import { Network, RefreshCw, Search } from 'lucide-react'
import { api, type TunnelItem } from '../api/client'
import StatusBadge from '../components/StatusBadge'

export default function Tunnels() {
  const [tunnels, setTunnels] = useState<TunnelItem[]>([])
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState<string | null>(null)
  const [search, setSearch] = useState('')
  const [statusFilter, setStatusFilter] = useState<string>('')
  const [summary, setSummary] = useState({ active: 0, inactive: 0, total: 0 })

  const load = async () => {
    try {
      setLoading(true)
      setError(null)
      const data = await api.listTunnels()
      setTunnels(data.tunnels ?? [])
      setSummary({ active: data.active, inactive: data.inactive, total: data.count })
    } catch (e) {
      setError((e as Error).message)
    } finally {
      setLoading(false)
    }
  }

  useEffect(() => { load() }, [])

  const filtered = tunnels.filter((t) => {
    const matchStatus = statusFilter ? t.status === statusFilter : true
    const matchSearch = search
      ? t.domain.includes(search) || t.client_id.includes(search) || t.tunnel_id.includes(search)
      : true
    return matchStatus && matchSearch
  })

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
          <h1 className="text-xl font-bold text-white">Tunnels</h1>
          <p className="text-sm text-gray-500 mt-0.5">
            {summary.active} active · {summary.inactive} inactive · {summary.total} total
          </p>
        </div>
        <button
          onClick={load}
          className="flex items-center gap-2 px-3 py-1.5 rounded-lg bg-gray-800 hover:bg-gray-700 text-sm text-gray-300 transition-colors"
        >
          <RefreshCw size={14} />
          Refresh
        </button>
      </div>

      {/* Filters */}
      <div className="flex gap-2">
        <div className="relative flex-1 max-w-sm">
          <Search size={13} className="absolute left-3 top-1/2 -translate-y-1/2 text-gray-500" />
          <input
            type="text"
            value={search}
            onChange={(e) => setSearch(e.target.value)}
            placeholder="Search by domain, client, or ID…"
            className="w-full bg-gray-900 border border-gray-700 rounded-lg pl-8 pr-3 py-2 text-sm text-white placeholder-gray-600 focus:outline-none focus:border-brand-500 focus:ring-1 focus:ring-brand-500"
          />
        </div>
        <select
          value={statusFilter}
          onChange={(e) => setStatusFilter(e.target.value)}
          className="bg-gray-900 border border-gray-700 rounded-lg px-3 py-2 text-sm text-gray-300 focus:outline-none focus:border-brand-500"
        >
          <option value="">All statuses</option>
          <option value="active">Active</option>
          <option value="inactive">Inactive</option>
        </select>
      </div>

      {/* Table */}
      <div className="bg-gray-900 border border-gray-800 rounded-xl overflow-hidden">
        {filtered.length === 0 ? (
          <div className="p-8 text-center">
            <Network size={24} className="text-gray-700 mx-auto mb-2" />
            <p className="text-sm text-gray-500">No tunnels found</p>
          </div>
        ) : (
          <div className="overflow-x-auto">
            <table className="w-full text-sm">
              <thead className="bg-gray-800/50">
                <tr>
                  <Th>Domain</Th>
                  <Th>Status</Th>
                  <Th>Client ID</Th>
                  <Th>Connection ID</Th>
                  <Th>Created</Th>
                  <Th>Updated</Th>
                </tr>
              </thead>
              <tbody className="divide-y divide-gray-800">
                {filtered.map((t) => (
                  <tr key={t.tunnel_id} className="hover:bg-gray-800/30 transition-colors">
                    <td className="px-4 py-3">
                      <p className="font-mono text-xs text-white">{t.domain}</p>
                      <p className="font-mono text-xs text-gray-600 mt-0.5">{t.tunnel_id}</p>
                    </td>
                    <td className="px-4 py-3">
                      <StatusBadge status={t.status} />
                    </td>
                    <td className="px-4 py-3 font-mono text-xs text-gray-400">{t.client_id}</td>
                    <td className="px-4 py-3 font-mono text-xs text-gray-500">
                      {t.connection_id || '—'}
                    </td>
                    <td className="px-4 py-3 text-xs text-gray-500">
                      {t.created_at ? new Date(t.created_at).toLocaleString() : '—'}
                    </td>
                    <td className="px-4 py-3 text-xs text-gray-500">
                      {t.updated_at ? new Date(t.updated_at).toLocaleString() : '—'}
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        )}
      </div>
    </div>
  )
}

function Th({ children }: { children: React.ReactNode }) {
  return (
    <th className="px-4 py-2.5 text-left text-xs font-medium text-gray-500 uppercase tracking-wide">
      {children}
    </th>
  )
}

function Skeleton() {
  return (
    <div className="space-y-4">
      <div className="h-8 w-48 bg-gray-800 rounded-lg animate-pulse" />
      <div className="h-64 bg-gray-900 border border-gray-800 rounded-xl animate-pulse" />
    </div>
  )
}
