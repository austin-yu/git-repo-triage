<script setup lang="ts">
import { ref, computed } from 'vue'
import { AlertTriangle, Flame, Users, Code } from 'lucide-vue-next'
import { 
  Chart as ChartJS, CategoryScale, LinearScale, PointElement, 
  BarElement, Title, Tooltip, Legend 
} from 'chart.js'
import { Scatter, Bar } from 'vue-chartjs'

// Register Chart.js components
ChartJS.register(CategoryScale, LinearScale, PointElement, BarElement, Title, Tooltip, Legend)

// --- Type Definitions ---
interface FileRisk { name: string; churn: number; bugs: number }
interface Contributor { name: string; commits: number }
interface RepoReport {
  riskMatrix: FileRisk[];
  busFactor: Contributor[];
  firefightingIncidents: number;
  couplingAlerts: string[];
}

// --- State ---
const path = ref('')
const report = ref<RepoReport | null>(null)
const loading = ref(false)
const error = ref('')

// --- API Call ---
const handleAnalyze = async () => {
  if (!path.value) return
  loading.value = true
  error.value = ''
  
  try {
    const res = await fetch(`http://localhost:8080/api/analyze?path=${encodeURIComponent(path.value)}`)
    if (!res.ok) {
      const errData = await res.json()
      throw new Error(errData.error || 'Analysis failed')
    }
    report.value = await res.json()
  } catch (err: any) {
    error.value = err.message
  } finally {
    loading.value = false
  }
}

// --- Chart Configurations ---
const scatterData = computed(() => {
  if (!report.value) return { datasets: [] }
  return {
    datasets: [{
      label: 'Files',
      backgroundColor: '#ef4444',
      // Map Go API data to Chart.js {x, y} format and attach the filename
      data: report.value.riskMatrix.map(file => ({
        x: file.churn,
        y: file.bugs,
        rawFile: file.name
      }))
    }]
  }
})

const scatterOptions = {
  responsive: true,
  maintainAspectRatio: false,
  plugins: {
    legend: { display: false },
    tooltip: {
      callbacks: {
        label: (context: any) => {
          const point = context.raw
          return `${point.rawFile} (Churn: ${point.x}, Bugs: ${point.y})`
        }
      }
    }
  },
  scales: {
    x: { title: { display: true, text: 'Total Commits (Churn)' } },
    y: { title: { display: true, text: 'Bug Fixes' } }
  }
}

const barData = computed(() => {
  if (!report.value) return { labels: [], datasets: [] }
  const top5 = report.value.busFactor.slice(0, 5)
  return {
    labels: top5.map(c => c.name),
    datasets: [{
      label: 'Commits',
      backgroundColor: ['#a855f7', '#d8b4fe', '#e9d5ff', '#f3e8ff', '#faf5ff'],
      data: top5.map(c => c.commits),
      borderRadius: 4
    }]
  }
})

const barOptions = {
  indexAxis: 'y' as const, // Makes the bar chart horizontal
  responsive: true,
  maintainAspectRatio: false,
  plugins: { legend: { display: false } }
}
</script>

<template>
  <div class="min-h-screen bg-gray-50 p-8 font-sans text-gray-900">
    <div class="max-w-7xl mx-auto space-y-6">
      
      <div class="bg-white p-6 rounded-xl shadow-sm border border-gray-200">
        <h1 class="text-3xl font-bold mb-2">Codebase Diagnostic Map</h1>
        <p class="text-gray-500 mb-6">Enter an absolute local path to generate structural and social metrics.</p>
        
        <div class="flex gap-4">
          <input 
            type="text" 
            v-model="path"
            @keydown.enter="handleAnalyze"
            placeholder="e.g., /Users/name/workspace/target-repo" 
            class="flex-1 p-3 border border-gray-300 rounded-lg focus:outline-none focus:ring-2 focus:ring-blue-500"
          />
          <button 
            @click="handleAnalyze"
            :disabled="loading"
            class="px-6 py-3 bg-blue-600 hover:bg-blue-700 disabled:bg-blue-300 text-white font-semibold rounded-lg transition-colors flex items-center gap-2"
          >
            {{ loading ? 'Scanning...' : 'Analyze Repository' }}
          </button>
        </div>
        <p v-if="error" class="text-red-500 mt-3 flex items-center gap-2">
          <AlertTriangle :size="18" /> {{ error }}
        </p>
      </div>

      <div v-if="report" class="grid grid-cols-1 lg:grid-cols-3 gap-6">
        
        <div class="lg:col-span-2 bg-white p-6 rounded-xl shadow-sm border border-gray-200">
          <h2 class="text-xl font-semibold mb-1 flex items-center gap-2">
            <Code class="text-blue-500" /> Structural Risk Matrix
          </h2>
          <p class="text-sm text-gray-500 mb-6">Top right quadrant = High Churn + High Bugs (Refactor Targets)</p>
          
          <div class="h-96 w-full relative">
            <Scatter :data="scatterData" :options="scatterOptions" />
          </div>
        </div>

        <div class="space-y-6">
          
          <div class="bg-white p-6 rounded-xl shadow-sm border border-gray-200">
            <h2 class="text-xl font-semibold mb-4 flex items-center gap-2">
              <Users class="text-purple-500" /> Bus Factor
            </h2>
            <div class="h-64 w-full relative">
              <Bar :data="barData" :options="barOptions" />
            </div>
          </div>

          <div class="bg-white p-6 rounded-xl shadow-sm border border-gray-200">
            <h2 class="text-xl font-semibold mb-4 flex items-center gap-2">
              <Flame class="text-orange-500" /> Operational Health
            </h2>
            <div class="flex items-center justify-between p-4 bg-orange-50 rounded-lg border border-orange-100">
              <span class="text-orange-900 font-medium">Hotfixes / Reverts</span>
              <span class="text-3xl font-bold text-orange-600">{{ report.firefightingIncidents }}</span>
            </div>
            
            <div v-if="report.couplingAlerts?.length" class="mt-6">
              <h3 class="text-sm font-semibold text-gray-700 mb-2 uppercase tracking-wider">Coupling Alerts</h3>
              <ul class="text-sm space-y-2 text-gray-600">
                <li v-for="(alert, index) in report.couplingAlerts" :key="index" class="p-2 bg-gray-50 rounded border border-gray-100 truncate">
                  {{ alert }}
                </li>
              </ul>
            </div>
          </div>

        </div>
      </div>
    </div>
  </div>
</template>