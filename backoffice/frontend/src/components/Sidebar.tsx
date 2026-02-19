import { NavLink } from 'react-router-dom'
import {
  LayoutDashboard,
  Zap,
  Database,
  Globe,
  Network,
  Users,
  LogOut,
} from 'lucide-react'
import { useAuthStore, useUIStore } from '../store/useStore'

const navItems = [
  { to: '/', label: 'Dashboard', icon: LayoutDashboard },
  { to: '/lambdas', label: 'Lambdas', icon: Zap },
  { to: '/databases', label: 'Databases', icon: Database },
  { to: '/cloudfront', label: 'CloudFront', icon: Globe },
  { to: '/tunnels', label: 'Tunnels', icon: Network },
  { to: '/clients', label: 'Clients', icon: Users },
]

export default function Sidebar() {
  const { sidebarOpen } = useUIStore()
  const { logout } = useAuthStore()

  if (!sidebarOpen) return null

  return (
    <aside className="fixed left-0 top-0 h-full w-56 bg-gray-900 border-r border-gray-800 flex flex-col z-20">
      {/* Logo */}
      <div className="flex items-center gap-2.5 px-4 py-4 border-b border-gray-800">
        <div className="w-7 h-7 rounded-lg bg-brand-600 flex items-center justify-center">
          <Network size={14} className="text-white" />
        </div>
        <div>
          <p className="text-sm font-semibold text-white leading-none">Tunnel</p>
          <p className="text-xs text-gray-500 mt-0.5">Backoffice</p>
        </div>
      </div>

      {/* Navigation */}
      <nav className="flex-1 px-2 py-3 space-y-0.5 overflow-y-auto">
        {navItems.map(({ to, label, icon: Icon }) => (
          <NavLink
            key={to}
            to={to}
            end={to === '/'}
            className={({ isActive }) =>
              `flex items-center gap-2.5 px-3 py-2 rounded-lg text-sm transition-colors ${
                isActive
                  ? 'bg-brand-600/20 text-brand-400 font-medium'
                  : 'text-gray-400 hover:text-white hover:bg-gray-800'
              }`
            }
          >
            <Icon size={15} />
            {label}
          </NavLink>
        ))}
      </nav>

      {/* Footer */}
      <div className="px-2 py-3 border-t border-gray-800">
        <button
          onClick={logout}
          className="flex items-center gap-2.5 w-full px-3 py-2 rounded-lg text-sm text-gray-400 hover:text-red-400 hover:bg-red-400/10 transition-colors"
        >
          <LogOut size={15} />
          Sign out
        </button>
      </div>
    </aside>
  )
}
