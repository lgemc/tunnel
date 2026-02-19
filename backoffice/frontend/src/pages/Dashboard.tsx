import { useEffect, useState } from 'react'
import { Zap, Network, Users, Globe, Database, Clock } from 'lucide-react'
import { api, type Stats } from '../api/client'
import StatCard from '../components/StatCard'

export default function Dashboard() {
  const [stats, setStats] = useState<Stats | null>(null)
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState<string | null>(null)

  const load = async () => {
    try {
      setLoading(true)
      setError(null)
      const data = await api.getStats()
      setStats(data)
    } catch (e) {
      setError((e as Error).message)
    } finally {
      setLoading(false)
    }
  }

  useEffect(() => { load() }, [])

  if (loading) return <LoadingSkeleton />

  if (error) {
    return (
      <div className="rounded-xl bg-red-500/10 border border-red-500/20 p-4 text-sm text-red-400">
        Failed to load stats: {error}
      </div>
    )
  }

  return (
    <div className="space-y-6">
      <div className="flex items-center justify-between">
        <div>
          <h1 className="text-xl font-bold text-white">Dashboard</h1>
          <p className="text-sm text-gray-500 mt-0.5">
            System overview{' '}
            {stats?.fetched_at && (
              <span>— updated {new Date(stats.fetched_at).toLocaleTimeString()}</span>
            )}
          </p>
        </div>
        <button
          onClick={load}
          className="flex items-center gap-2 px-3 py-1.5 rounded-lg bg-gray-800 hover:bg-gray-700 text-sm text-gray-300 transition-colors"
        >
          <Clock size={14} />
          Refresh
        </button>
      </div>

      <div className="grid grid-cols-2 lg:grid-cols-3 xl:grid-cols-4 gap-4">
        <StatCard
          label="Lambda Functions"
          value={stats?.total_lambdas ?? 0}
          icon={Zap}
          sub={`${stats?.active_lambdas ?? 0} active`}
          accent="blue"
        />
        <StatCard
          label="Active Tunnels"
          value={stats?.active_tunnels ?? 0}
          icon={Network}
          sub={`${stats?.total_tunnels ?? 0} total`}
          accent="emerald"
        />
        <StatCard
          label="Clients"
          value={stats?.total_clients ?? 0}
          icon={Users}
          accent="purple"
        />
        <StatCard
          label="Domains"
          value={stats?.total_domains ?? 0}
          icon={Globe}
          accent="sky"
        />
        <StatCard
          label="Pending Requests"
          value={stats?.pending_requests ?? 0}
          icon={Database}
          sub="in-flight (TTL 5m)"
          accent="amber"
        />
      </div>

      {/* Quick status */}
      <div className="bg-gray-900 border border-gray-800 rounded-xl p-4">
        <h2 className="text-sm font-semibold text-white mb-3">System Health</h2>
        <div className="grid grid-cols-1 sm:grid-cols-2 gap-3">
          <HealthRow
            label="Lambda Functions"
            total={stats?.total_lambdas ?? 0}
            healthy={stats?.active_lambdas ?? 0}
          />
          <HealthRow
            label="Tunnels"
            total={stats?.total_tunnels ?? 0}
            healthy={stats?.active_tunnels ?? 0}
          />
        </div>
      </div>
    </div>
  )
}

function HealthRow({ label, total, healthy }: { label: string; total: number; healthy: number }) {
  const pct = total > 0 ? Math.round((healthy / total) * 100) : 0
  const color = pct >= 80 ? 'bg-emerald-500' : pct >= 50 ? 'bg-amber-500' : 'bg-red-500'
  return (
    <div>
      <div className="flex justify-between text-xs mb-1">
        <span className="text-gray-400">{label}</span>
        <span className="text-gray-500">{healthy}/{total}</span>
      </div>
      <div className="h-1.5 bg-gray-800 rounded-full overflow-hidden">
        <div className={`h-full ${color} rounded-full transition-all`} style={{ width: `${pct}%` }} />
      </div>
    </div>
  )
}

function LoadingSkeleton() {
  return (
    <div className="space-y-6">
      <div className="h-8 w-40 bg-gray-800 rounded-lg animate-pulse" />
      <div className="grid grid-cols-2 lg:grid-cols-3 xl:grid-cols-4 gap-4">
        {Array.from({ length: 5 }).map((_, i) => (
          <div key={i} className="h-24 bg-gray-900 border border-gray-800 rounded-xl animate-pulse" />
        ))}
      </div>
    </div>
  )
}
