interface StatusBadgeProps {
  status: string
  size?: 'sm' | 'md'
}

const colorMap: Record<string, string> = {
  active: 'bg-emerald-400/15 text-emerald-400 ring-emerald-400/20',
  Active: 'bg-emerald-400/15 text-emerald-400 ring-emerald-400/20',
  Deployed: 'bg-emerald-400/15 text-emerald-400 ring-emerald-400/20',
  ACTIVE: 'bg-emerald-400/15 text-emerald-400 ring-emerald-400/20',
  inactive: 'bg-gray-400/15 text-gray-400 ring-gray-400/20',
  Inactive: 'bg-gray-400/15 text-gray-400 ring-gray-400/20',
  INACTIVE: 'bg-gray-400/15 text-gray-400 ring-gray-400/20',
  InProgress: 'bg-amber-400/15 text-amber-400 ring-amber-400/20',
  error: 'bg-red-400/15 text-red-400 ring-red-400/20',
  Failed: 'bg-red-400/15 text-red-400 ring-red-400/20',
  CREATING: 'bg-blue-400/15 text-blue-400 ring-blue-400/20',
  UPDATING: 'bg-blue-400/15 text-blue-400 ring-blue-400/20',
}

export default function StatusBadge({ status, size = 'sm' }: StatusBadgeProps) {
  const color = colorMap[status] ?? 'bg-gray-400/15 text-gray-400 ring-gray-400/20'
  const sz = size === 'sm' ? 'px-2 py-0.5 text-xs' : 'px-2.5 py-1 text-sm'
  return (
    <span className={`inline-flex items-center rounded-full ring-1 font-medium ${color} ${sz}`}>
      {status}
    </span>
  )
}
