import { useState, useEffect } from 'react'
import { Link } from 'react-router-dom'
import { Users, Activity, Shield, CheckCircle, XCircle } from 'lucide-react'
import api from '../api'

export default function Dashboard() {
  const [health, setHealth] = useState(null)
  const [tenants, setTenants] = useState([])
  const [loading, setLoading] = useState(true)

  useEffect(() => {
    loadData()
    const interval = setInterval(loadData, 5000)
    return () => clearInterval(interval)
  }, [])

  const loadData = async () => {
    try {
      const [healthRes, tenantsRes] = await Promise.all([
        api.getHealth(),
        api.getTenants(),
      ])
      setHealth(healthRes.data)
      setTenants(tenantsRes.data.tenants || [])
    } catch (error) {
      console.error('Failed to load data:', error)
    } finally {
      setLoading(false)
    }
  }

  if (loading) {
    return (
      <div className="flex items-center justify-center h-screen">
        <div className="text-center">
          <div className="animate-spin rounded-full h-12 w-12 border-b-2 border-primary-600 mx-auto"></div>
          <p className="mt-4 text-gray-600">Loading...</p>
        </div>
      </div>
    )
  }

  const activeTenants = tenants.filter(t => t.enabled).length

  return (
    <div className="p-8">
      <div className="mb-8">
        <h1 className="text-3xl font-bold text-gray-900">Dashboard</h1>
        <p className="text-gray-600 mt-2">Tổng quan hệ thống WAF</p>
      </div>

      {/* Stats Grid */}
      <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-4 gap-6 mb-8">
        <StatCard
          icon={Shield}
          label="Version"
          value={health?.version || 'N/A'}
          color="blue"
        />
        <StatCard
          icon={Users}
          label="Total Tenants"
          value={tenants.length}
          color="green"
        />
        <StatCard
          icon={CheckCircle}
          label="Active Tenants"
          value={activeTenants}
          color="green"
        />
        <StatCard
          icon={Activity}
          label="Status"
          value={health?.status || 'Unknown'}
          color={health?.status === 'healthy' ? 'green' : 'red'}
        />
      </div>

      {/* System Info */}
      <div className="bg-white rounded-lg shadow p-6 mb-8">
        <h2 className="text-xl font-semibold mb-4">System Information</h2>
        <div className="grid grid-cols-1 md:grid-cols-2 gap-4">
          <InfoRow label="Mode" value={health?.mode} />
          <InfoRow label="Storage" value={health?.storage?.replace('*storage.', '')} />
          <InfoRow label="TLS" value={health?.tls ? 'Enabled' : 'Disabled'} />
          <InfoRow label="Metrics" value={health?.metrics ? 'Enabled' : 'Disabled'} />
          <InfoRow label="Uptime" value={health?.uptime} />
          <InfoRow label="Go Version" value={health?.go_version} />
        </div>
      </div>

      {/* Recent Tenants */}
      <div className="bg-white rounded-lg shadow p-6">
        <div className="flex justify-between items-center mb-4">
          <h2 className="text-xl font-semibold">Recent Tenants</h2>
          <Link to="/tenants" className="text-primary-600 hover:text-primary-700 text-sm">
            View All →
          </Link>
        </div>
        <div className="space-y-3">
          {tenants.slice(0, 5).map((tenant) => (
            <Link
              key={tenant.id}
              to={`/tenants/${tenant.id}`}
              className="flex items-center justify-between p-4 bg-gray-50 rounded-lg hover:bg-gray-100 transition-colors"
            >
              <div className="flex items-center space-x-3">
                {tenant.enabled ? (
                  <CheckCircle className="w-5 h-5 text-green-500" />
                ) : (
                  <XCircle className="w-5 h-5 text-red-500" />
                )}
                <div>
                  <p className="font-medium text-gray-900">{tenant.name}</p>
                  <p className="text-sm text-gray-500">{tenant.domains?.join(', ')}</p>
                </div>
              </div>
              <span className="text-xs text-gray-400">
                {new Date(tenant.created_at).toLocaleDateString('vi-VN')}
              </span>
            </Link>
          ))}
        </div>
      </div>
    </div>
  )
}

function StatCard({ icon: Icon, label, value, color }) {
  const colors = {
    blue: 'bg-blue-50 text-blue-600',
    green: 'bg-green-50 text-green-600',
    red: 'bg-red-50 text-red-600',
  }

  return (
    <div className="bg-white rounded-lg shadow p-6">
      <div className="flex items-center justify-between">
        <div>
          <p className="text-sm text-gray-600 mb-1">{label}</p>
          <p className="text-2xl font-bold text-gray-900">{value}</p>
        </div>
        <div className={`p-3 rounded-full ${colors[color]}`}>
          <Icon className="w-6 h-6" />
        </div>
      </div>
    </div>
  )
}

function InfoRow({ label, value }) {
  return (
    <div className="flex justify-between py-2 border-b border-gray-100">
      <span className="text-gray-600">{label}</span>
      <span className="font-medium text-gray-900">{value}</span>
    </div>
  )
}
