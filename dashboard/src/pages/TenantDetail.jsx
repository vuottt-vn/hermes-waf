import { useState, useEffect } from 'react'
import { useParams, Link } from 'react-router-dom'
import { ArrowLeft, CheckCircle, XCircle, Edit, Save } from 'lucide-react'
import api from '../api'

export default function TenantDetail() {
  const { id } = useParams()
  const [tenant, setTenant] = useState(null)
  const [loading, setLoading] = useState(true)
  const [editing, setEditing] = useState(false)
  const [formData, setFormData] = useState(null)

  useEffect(() => {
    loadTenant()
  }, [id])

  const loadTenant = async () => {
    try {
      const res = await api.getTenant(id)
      setTenant(res.data)
      setFormData(res.data)
    } catch (error) {
      console.error('Failed to load tenant:', error)
    } finally {
      setLoading(false)
    }
  }

  const handleSave = async () => {
    try {
      await api.updateTenant(id, formData)
      setTenant(formData)
      setEditing(false)
      alert('Tenant updated successfully')
    } catch (error) {
      alert('Failed to update tenant: ' + error.message)
    }
  }

  if (loading) {
    return (
      <div className="flex items-center justify-center h-screen">
        <div className="animate-spin rounded-full h-12 w-12 border-b-2 border-primary-600"></div>
      </div>
    )
  }

  if (!tenant) {
    return (
      <div className="p-8">
        <div className="text-center text-gray-500">Tenant not found</div>
      </div>
    )
  }

  return (
    <div className="p-8">
      <Link to="/tenants" className="flex items-center text-gray-600 hover:text-gray-900 mb-6">
        <ArrowLeft className="w-5 h-5 mr-2" />
        Back to Tenants
      </Link>

      <div className="flex justify-between items-center mb-8">
        <div className="flex items-center space-x-4">
          {tenant.enabled ? (
            <CheckCircle className="w-8 h-8 text-green-500" />
          ) : (
            <XCircle className="w-8 h-8 text-red-500" />
          )}
          <div>
            <h1 className="text-3xl font-bold text-gray-900">{tenant.name}</h1>
            <p className="text-gray-600">{tenant.id}</p>
          </div>
        </div>
        {!editing ? (
          <button
            onClick={() => setEditing(true)}
            className="flex items-center px-4 py-2 bg-primary-600 text-white rounded-lg hover:bg-primary-700"
          >
            <Edit className="w-5 h-5 mr-2" />
            Edit
          </button>
        ) : (
          <button
            onClick={handleSave}
            className="flex items-center px-4 py-2 bg-green-600 text-white rounded-lg hover:bg-green-700"
          >
            <Save className="w-5 h-5 mr-2" />
            Save
          </button>
        )}
      </div>

      <div className="grid grid-cols-1 lg:grid-cols-2 gap-6">
        {/* Basic Info */}
        <div className="bg-white rounded-lg shadow p-6">
          <h2 className="text-xl font-semibold mb-4">Basic Information</h2>
          <div className="space-y-4">
            <div>
              <label className="block text-sm font-medium text-gray-700 mb-1">Name</label>
              {editing ? (
                <input
                  type="text"
                  value={formData.name}
                  onChange={(e) => setFormData({ ...formData, name: e.target.value })}
                  className="w-full px-3 py-2 border border-gray-300 rounded-lg"
                />
              ) : (
                <p className="text-gray-900">{tenant.name}</p>
              )}
            </div>
            <div>
              <label className="block text-sm font-medium text-gray-700 mb-1">Status</label>
              {editing ? (
                <select
                  value={formData.enabled}
                  onChange={(e) => setFormData({ ...formData, enabled: e.target.checked })}
                  className="w-full px-3 py-2 border border-gray-300 rounded-lg"
                >
                  <option value={true}>Active</option>
                  <option value={false}>Inactive</option>
                </select>
              ) : (
                <span className={`px-2 py-1 rounded-full text-sm ${
                  tenant.enabled ? 'bg-green-100 text-green-800' : 'bg-red-100 text-red-800'
                }`}>
                  {tenant.enabled ? 'Active' : 'Inactive'}
                </span>
              )}
            </div>
            <div>
              <label className="block text-sm font-medium text-gray-700 mb-1">Created</label>
              <p className="text-gray-900">
                {new Date(tenant.created_at).toLocaleString('vi-VN')}
              </p>
            </div>
            <div>
              <label className="block text-sm font-medium text-gray-700 mb-1">Updated</label>
              <p className="text-gray-900">
                {new Date(tenant.updated_at).toLocaleString('vi-VN')}
              </p>
            </div>
          </div>
        </div>

        {/* Domains */}
        <div className="bg-white rounded-lg shadow p-6">
          <h2 className="text-xl font-semibold mb-4">Domains</h2>
          {editing ? (
            <textarea
              value={formData.domains?.join('\n')}
              onChange={(e) => setFormData({ 
                ...formData, 
                domains: e.target.value.split('\n').map(d => d.trim()).filter(d => d)
              })}
              className="w-full px-3 py-2 border border-gray-300 rounded-lg h-32"
              placeholder="One domain per line"
            />
          ) : (
            <div className="space-y-2">
              {tenant.domains?.map((domain, idx) => (
                <div key={idx} className="flex items-center p-3 bg-gray-50 rounded-lg">
                  <span className="text-gray-900">{domain}</span>
                </div>
              ))}
            </div>
          )}
        </div>

        {/* WAF Config */}
        <div className="bg-white rounded-lg shadow p-6 lg:col-span-2">
          <h2 className="text-xl font-semibold mb-4">WAF Configuration</h2>
          <div className="grid grid-cols-1 md:grid-cols-2 gap-4">
            <ConfigItem
              label="Default Action"
              value={tenant.config?.default_action}
            />
            <ConfigItem
              label="Rate Limit"
              value={`${tenant.config?.rate_limit_per_min || 0} req/min`}
            />
            <ConfigItem
              label="Request Body Access"
              value={tenant.config?.request_body_access ? 'Enabled' : 'Disabled'}
            />
            <ConfigItem
              label="Request Body Limit"
              value={`${((tenant.config?.request_body_limit || 0) / 1024 / 1024).toFixed(2)} MB`}
            />
            <ConfigItem
              label="Response Body Access"
              value={tenant.config?.response_body_access ? 'Enabled' : 'Disabled'}
            />
            <ConfigItem
              label="Response Body Limit"
              value={`${((tenant.config?.response_body_limit || 0) / 1024).toFixed(2)} KB`}
            />
            <ConfigItem
              label="Audit Log"
              value={tenant.config?.audit_log_enabled ? 'Enabled' : 'Disabled'}
            />
          </div>
        </div>
      </div>
    </div>
  )
}

function ConfigItem({ label, value }) {
  return (
    <div className="p-4 bg-gray-50 rounded-lg">
      <p className="text-sm text-gray-600 mb-1">{label}</p>
      <p className="text-lg font-medium text-gray-900">{value}</p>
    </div>
  )
}
