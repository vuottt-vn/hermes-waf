import { BrowserRouter as Router, Routes, Route, Link, useLocation } from 'react-router-dom'
import { Shield, LayoutDashboard, Users, Activity, Settings } from 'lucide-react'
import Dashboard from './pages/Dashboard'
import TenantsList from './pages/TenantsList'
import TenantDetail from './pages/TenantDetail'
import Metrics from './pages/Metrics'
import SettingsPage from './pages/Settings'

function Sidebar() {
  const location = useLocation()
  
  const menuItems = [
    { path: '/', icon: LayoutDashboard, label: 'Dashboard' },
    { path: '/tenants', icon: Users, label: 'Tenants' },
    { path: '/metrics', icon: Activity, label: 'Metrics' },
    { path: '/settings', icon: Settings, label: 'Settings' },
  ]

  return (
    <div className="w-64 bg-gray-900 text-white min-h-screen">
      <div className="p-6 border-b border-gray-800">
        <div className="flex items-center space-x-3">
          <Shield className="w-8 h-8 text-primary-500" />
          <div>
            <h1 className="text-xl font-bold">WAF Dashboard</h1>
            <p className="text-xs text-gray-400">Vinahost Security</p>
          </div>
        </div>
      </div>
      
      <nav className="mt-6">
        {menuItems.map((item) => {
          const Icon = item.icon
          const isActive = location.pathname === item.path
          return (
            <Link
              key={item.path}
              to={item.path}
              className={`flex items-center px-6 py-3 text-sm transition-colors ${
                isActive
                  ? 'bg-primary-600 text-white border-r-4 border-primary-500'
                  : 'text-gray-300 hover:bg-gray-800'
              }`}
            >
              <Icon className="w-5 h-5 mr-3" />
              {item.label}
            </Link>
          )
        })}
      </nav>
    </div>
  )
}

function App() {
  return (
    <Router>
      <div className="flex bg-gray-50">
        <Sidebar />
        <div className="flex-1">
          <Routes>
            <Route path="/" element={<Dashboard />} />
            <Route path="/tenants" element={<TenantsList />} />
            <Route path="/tenants/:id" element={<TenantDetail />} />
            <Route path="/metrics" element={<Metrics />} />
            <Route path="/settings" element={<SettingsPage />} />
          </Routes>
        </div>
      </div>
    </Router>
  )
}

export default App
