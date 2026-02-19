import { BrowserRouter, Routes, Route, Navigate } from 'react-router-dom'
import { useAuthStore } from './store/useStore'
import Layout from './components/Layout'
import Login from './pages/Login'
import Dashboard from './pages/Dashboard'
import Lambdas from './pages/Lambdas'
import Databases from './pages/Databases'
import CloudFront from './pages/CloudFront'
import Tunnels from './pages/Tunnels'
import Clients from './pages/Clients'

function ProtectedRoute({ children }: { children: React.ReactNode }) {
  const isAuthenticated = useAuthStore((s) => s.isAuthenticated)
  if (!isAuthenticated) return <Navigate to="/login" replace />
  return <>{children}</>
}

export default function App() {
  return (
    <BrowserRouter>
      <Routes>
        <Route path="/login" element={<Login />} />
        <Route
          path="/"
          element={
            <ProtectedRoute>
              <Layout />
            </ProtectedRoute>
          }
        >
          <Route index element={<Dashboard />} />
          <Route path="lambdas" element={<Lambdas />} />
          <Route path="databases" element={<Databases />} />
          <Route path="cloudfront" element={<CloudFront />} />
          <Route path="tunnels" element={<Tunnels />} />
          <Route path="clients" element={<Clients />} />
        </Route>
        <Route path="*" element={<Navigate to="/" replace />} />
      </Routes>
    </BrowserRouter>
  )
}
