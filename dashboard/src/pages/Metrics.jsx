import { useState, useEffect } from 'react'
import { Activity, TrendingUp, Shield, AlertTriangle } from 'lucide-react'
import { LineChart, Line, XAxis, YAxis, CartesianGrid, Tooltip, ResponsiveContainer } from 'recharts'
import api from '../api'

export default function Metrics() {
  const [metrics, setMetrics] = useState({
    total: 0,
    blocked: 0,
    allowed: 0,
    latency: 0,
  })
  const [chartData, setChartData] = useState([])

  useEffect(() => {
    loadMetrics()
    const interval = setInterval(loadMetrics, 5000)
    return () => clearInterval(interval)
  }, [])

  const loadMetrics = async () => {
    try {
      const res = await api.getMetrics()
      const text = res.data
      
      // Parse Prometheus format
      const parseMetric = (name) => {
        const match = text.match(new RegExp(`${name}\\s+(\\d+)`))
        return match ? parseInt(match[1]) : 0
      }

      const newMetrics = {
        total: parseMetric('waf_requests_total'),
        blocked: parseMetric('waf_requests_blocked'),
        allowed: parseMetric('waf_requests_allowed'),
        latency: parseFloat(text.match(/waf_request_latency_seconds\s+([\d.]+)/)?.[1] || 0),
      }

      setMetrics(newMetrics)
      
      // Update chart data
      setChartData(prev => {
        const newData = [...prev, {
          time: new Date().toLocaleTimeString('vi-VN'),
          total: newMetrics.total,
          blocked: newMetrics.blocked,
          allowed: newMetrics.allowed,
        }]
        return newData.slice(-20) // Keep last 20 points
      })
    } catch (error) {
      console.error('Failed to load metrics:', error)
    }
  }

  const blockRate = metrics.total > 0 
    ? ((metrics.blocked / metrics.total) * 100).toFixed(2)
    : 0

  return (
    <div className="p-8">
      <div className="mb-8">
        <h1 className="text-3xl font-bold text-gray-900">Metrics</h1>
        <p className="text-gray-600 mt-2">Giám sát hiệu suất và bảo mật WAF</p>
      </div>

      {/* Stats Grid */}
      <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-4 gap-6 mb-8">
        <MetricCard
          icon={Activity}
          label="Total Requests"
          value={metrics.total.toLocaleString()}
          color="blue"
        />
        <MetricCard
          icon={Shield}
          label="Allowed"
          value={metrics.allowed.toLocaleString()}
          color="green"
        />
        <MetricCard
          icon={AlertTriangle}
          label="Blocked"
          value={metrics.blocked.toLocaleString()}
          color="red"
        />
        <MetricCard
          icon={TrendingUp}
          label="Block Rate"
          value={`${blockRate}%`}
          color="orange"
        />
      </div>

      {/* Charts */}
      <div className="grid grid-cols-1 lg:grid-cols-2 gap-6 mb-8">
        <div className="bg-white rounded-lg shadow p-6">
          <h2 className="text-xl font-semibold mb-4">Request Trend</h2>
          <ResponsiveContainer width="100%" height={300}>
            <LineChart data={chartData}>
              <CartesianGrid strokeDasharray="3 3" />
              <XAxis dataKey="time" />
              <YAxis />
              <Tooltip />
              <Line type="monotone" dataKey="total" stroke="#3b82f6" name="Total" />
              <Line type="monotone" dataKey="allowed" stroke="#10b981" name="Allowed" />
              <Line type="monotone" dataKey="blocked" stroke="#ef4444" name="Blocked" />
            </LineChart>
          </ResponsiveContainer>
        </div>

        <div className="bg-white rounded-lg shadow p-6">
          <h2 className="text-xl font-semibold mb-4">Performance</h2>
          <div className="space-y-6">
            <div>
              <div className="flex justify-between mb-2">
                <span className="text-gray-600">Average Latency</span>
                <span className="font-medium">{(metrics.latency * 1000).toFixed(2)} ms</span>
              </div>
              <div className="w-full bg-gray-200 rounded-full h-2">
                <div 
                  className="bg-blue-600 h-2 rounded-full transition-all"
                  style={{ width: `${Math.min(metrics.latency * 1000 / 10, 100)}%` }}
                ></div>
              </div>
            </div>
            
            <div>
              <div className="flex justify-between mb-2">
                <span className="text-gray-600">Success Rate</span>
                <span className="font-medium">{(100 - blockRate).toFixed(2)}%</span>
              </div>
              <div className="w-full bg-gray-200 rounded-full h-2">
                <div 
                  className="bg-green-600 h-2 rounded-full transition-all"
                  style={{ width: `${100 - blockRate}%` }}
                ></div>
              </div>
            </div>

            <div>
              <div className="flex justify-between mb-2">
                <span className="text-gray-600">Threat Detection</span>
                <span className="font-medium">{metrics.blocked} threats blocked</span>
              </div>
              <div className="w-full bg-gray-200 rounded-full h-2">
                <div 
                  className="bg-red-600 h-2 rounded-full transition-all"
                  style={{ width: `${blockRate}%` }}
                ></div>
              </div>
            </div>
          </div>
        </div>
      </div>

      {/* Raw Metrics */}
      <div className="bg-white rounded-lg shadow p-6">
        <h2 className="text-xl font-semibold mb-4">Raw Metrics Data</h2>
        <pre className="bg-gray-50 p-4 rounded-lg overflow-x-auto text-sm">
          {JSON.stringify(metrics, null, 2)}
        </pre>
      </div>
    </div>
  )
}

function MetricCard({ icon: Icon, label, value, color }) {
  const colors = {
    blue: 'bg-blue-50 text-blue-600',
    green: 'bg-green-50 text-green-600',
    red: 'bg-red-50 text-red-600',
    orange: 'bg-orange-50 text-orange-600',
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
