import { useState, useEffect } from 'react'
import { Save, RefreshCw } from 'lucide-react'
import api from '../api'

export default function Settings() {
  const [cacheStatus, setCacheStatus] = useState(null)
  const [loading, setLoading] = useState(false)

  useEffect(() => {
    loadCacheStatus()
  }, [])

  const loadCacheStatus = async () => {
    try {
      const res = await api.getCacheStatus()
      setCacheStatus(res.data)
    } catch (error) {
      console.error('Failed to load cache status:', error)
    }
  }

  const handleRefresh = async () => {
    setLoading(true)
    await loadCacheStatus()
    setLoading(false)
  }

  return (
    <div className="p-8">
      <div className="mb-8">
        <h1 className="text-3xl font-bold text-gray-900">Settings</h1>
        <p className="text-gray-600 mt-2">Cấu hình hệ thống WAF</p>
      </div>

      <div className="grid grid-cols-1 lg:grid-cols-2 gap-6">
        {/* Cache Status */}
        <div className="bg-white rounded-lg shadow p-6">
          <div className="flex justify-between items-center mb-4">
            <h2 className="text-xl font-semibold">Cache Status</h2>
            <button
              onClick={handleRefresh}
              disabled={loading}
              className="p-2 text-gray-600 hover:text-gray-900 disabled:opacity-50"
            >
              <RefreshCw className={`w-5 h-5 ${loading ? 'animate-spin' : ''}`} />
            </button>
          </div>
          
          {cacheStatus ? (
            <div className="space-y-4">
              <div className="flex justify-between py-2 border-b border-gray-100">
                <span className="text-gray-600">Cache Type</span>
                <span className="font-medium text-gray-900">{cacheStatus.type}</span>
              </div>
              <div className="flex justify-between py-2 border-b border-gray-100">
                <span className="text-gray-600">Total Tenants Cached</span>
                <span className="font-medium text-gray-900">{cacheStatus.total_cached}</span>
              </div>
              <div className="flex justify-between py-2 border-b border-gray-100">
                <span className="text-gray-600">Cache Hit Rate</span>
                <span className="font-medium text-green-600">{cacheStatus.hit_rate || 'N/A'}</span>
              </div>
              
              {cacheStatus.tenants && (
                <div className="mt-4">
                  <h3 className="text-sm font-medium text-gray-700 mb-2">Cached Tenants</h3>
                  <div className="space-y-2">
                    {cacheStatus.tenants.map((tenant, idx) => (
                      <div key={idx} className="flex items-center justify-between p-2 bg-gray-50 rounded">
                        <span className="text-sm text-gray-900">{tenant.id}</span>
                        <span className="text-xs text-gray-500">{tenant.domains?.length || 0} domains</span>
                      </div>
                    ))}
                  </div>
                </div>
              )}
            </div>
          ) : (
            <div className="text-center text-gray-500 py-8">
              Loading cache status...
            </div>
          )}
        </div>

        {/* System Info */}
        <div className="bg-white rounded-lg shadow p-6">
          <h2 className="text-xl font-semibold mb-4">System Configuration</h2>
          <div className="space-y-4">
            <div className="flex justify-between py-2 border-b border-gray-100">
              <span className="text-gray-600">WAF Version</span>
              <span className="font-medium text-gray-900">0.4.0</span>
            </div>
            <div className="flex justify-between py-2 border-b border-gray-100">
              <span className="text-gray-600">Mode</span>
              <span className="font-medium text-gray-900">Multi-Tenant</span>
            </div>
            <div className="flex justify-between py-2 border-b border-gray-100">
              <span className="text-gray-600">Storage Backend</span>
              <span className="font-medium text-gray-900">PostgreSQL</span>
            </div>
            <div className="flex justify-between py-2 border-b border-gray-100">
              <span className="text-gray-600">Cache Backend</span>
              <span className="font-medium text-gray-900">Redis</span>
            </div>
            <div className="flex justify-between py-2 border-b border-gray-100">
              <span className="text-gray-600">Metrics</span>
              <span className="font-medium text-green-600">Enabled</span>
            </div>
            <div className="flex justify-between py-2 border-b border-gray-100">
              <span className="text-gray-600">Rate Limiting</span>
              <span className="font-medium text-green-600">Enabled</span>
            </div>
          </div>
        </div>

        {/* API Endpoints */}
        <div className="bg-white rounded-lg shadow p-6 lg:col-span-2">
          <h2 className="text-xl font-semibold mb-4">API Endpoints</h2>
          <div className="space-y-3">
            <EndpointRow method="GET" path="/health" description="Health check" />
            <EndpointRow method="GET" path="/api/v1/tenants" description="List all tenants" />
            <EndpointRow method="POST" path="/api/v1/tenants" description="Create new tenant" />
            <EndpointRow method="GET" path="/api/v1/tenants/:id" description="Get tenant details" />
            <EndpointRow method="PUT" path="/api/v1/tenants/:id" description="Update tenant" />
            <EndpointRow method="DELETE" path="/api/v1/tenants/:id" description="Delete tenant" />
            <EndpointRow method="GET" path="/api/v1/cache/status" description="Cache statistics" />
            <EndpointRow method="GET" path="/metrics" description="Prometheus metrics" />
          </div>
        </div>
      </div>
    </div>
  )
}

function EndpointRow({ method, path, description }) {
  const methodColors = {
    GET: 'bg-green-100 text-green-800',
    POST: 'bg-blue-100 text-blue-800',
    PUT: 'bg-orange-100 text-orange-800',
    DELETE: 'bg-red-100 text-red-800',
  }

  return (
    <div className="flex items-center justify-between p-3 bg-gray-50 rounded-lg">
      <div className="flex items-center space-x-3">
        <span className={`px-2 py-1 rounded text-xs font-medium ${methodColors[method]}`}>
          {method}
        </span>
        <code className="text-sm text-gray-900">{path}</code>
      </div>
      <span className="text-sm text-gray-600">{description}</span>
    </div>
  )
}
