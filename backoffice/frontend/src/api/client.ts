import { useAuthStore } from '../store/useStore'

const BASE = import.meta.env.VITE_API_URL ?? ''

async function apiFetch<T>(path: string, options?: RequestInit): Promise<T> {
  const { apiKey } = useAuthStore.getState()
  const res = await fetch(`${BASE}${path}`, {
    ...options,
    headers: {
      'Content-Type': 'application/json',
      ...(apiKey ? { Authorization: `Bearer ${apiKey}` } : {}),
      ...(options?.headers ?? {}),
    },
  })
  if (res.status === 401) {
    useAuthStore.getState().logout()
    throw new Error('Unauthorized')
  }
  if (!res.ok) {
    const body = await res.json().catch(() => ({}))
    throw new Error((body as { error?: string }).error ?? `HTTP ${res.status}`)
  }
  return res.json() as Promise<T>
}

// ---- Types ----

export interface LambdaInfo {
  name: string
  function_arn: string
  runtime: string
  handler: string
  memory_size_mb: number
  timeout_seconds: number
  code_size_bytes: number
  last_modified: string
  state: string
  description: string
  log_group: string
  fetched_at: string
}

export interface LogEvent {
  timestamp: string
  message: string
  log_stream: string
}

export interface TableInfo {
  name: string
  status: string
  item_count: number
  size_bytes: number
  billing_mode: string
  key_schema: { name: string; type: string }[]
  created_at: string
  last_updated_at: string
  gsi_count: number
}

export interface DistributionInfo {
  id: string
  domain_name: string
  status: string
  enabled: boolean
  comment: string
  price_class: string
  http_version: string
  aliases: string[]
  origin_count: number
  cache_behaviors_count: number
  last_modified_time: string
}

export interface TunnelItem {
  tunnel_id: string
  client_id: string
  domain: string
  subdomain: string
  status: string
  connection_id?: string
  created_at: string
  updated_at: string
}

export interface ClientItem {
  client_id: string
  status: string
  created_at: string
}

export interface Stats {
  total_lambdas: number
  active_lambdas: number
  total_tunnels: number
  active_tunnels: number
  total_clients: number
  total_domains: number
  pending_requests: number
  fetched_at: string
}

// ---- API functions ----

export const api = {
  getStats: () => apiFetch<Stats>('/api/stats'),

  listLambdas: () =>
    apiFetch<{ lambdas: LambdaInfo[]; count: number }>('/api/lambdas'),

  getLambdaLogs: (name: string, limit = 100, sinceMs?: number) => {
    const params = new URLSearchParams({ limit: String(limit) })
    if (sinceMs) params.set('since_ms', String(sinceMs))
    return apiFetch<{ function: string; log_group: string; events: LogEvent[]; count: number }>(
      `/api/lambdas/${encodeURIComponent(name)}/logs?${params}`,
    )
  },

  listDatabases: () =>
    apiFetch<{ tables: TableInfo[]; count: number }>('/api/databases'),

  getTableItems: (table: string) =>
    apiFetch<{ table: string; items: Record<string, unknown>[]; count: number; has_more: boolean }>(
      `/api/databases/${encodeURIComponent(table)}/items`,
    ),

  getCloudFront: () =>
    apiFetch<{ distributions: DistributionInfo[]; count: number }>('/api/cloudfront'),

  listTunnels: (status?: string) => {
    const params = status ? `?status=${status}` : ''
    return apiFetch<{ tunnels: TunnelItem[]; count: number; active: number; inactive: number }>(
      `/api/tunnels${params}`,
    )
  },

  listClients: () =>
    apiFetch<{ clients: ClientItem[]; count: number }>('/api/clients'),
}
