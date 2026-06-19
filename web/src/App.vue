<script setup lang="ts">
import { ref, computed, onMounted } from 'vue'
import { AlertTriangle, Flame, Users, Code, Ghost, TrendingUp, Sun, Moon } from 'lucide-vue-next'
import { 
  Chart as ChartJS, CategoryScale, LinearScale, PointElement, 
  BarElement, LineElement, ArcElement, BubbleController,
  Title, Tooltip, Legend, Filler, LineController
} from 'chart.js'
import { Scatter, Doughnut, Bubble, Line } from 'vue-chartjs'

// Register Chart.js components
ChartJS.register(
  CategoryScale, LinearScale, PointElement, BarElement, 
  LineElement, ArcElement, BubbleController, LineController,
  Title, Tooltip, Legend, Filler
)

// --- Type Definitions ---
interface FileRisk { name: string; churn: number; bugs: number }
interface Contributor { name: string; commits: number }
interface SleepingGiant { name: string; lines: number; daysSinceLastCommit: number; complexity: number }
interface MonthlyActivity { month: string; commits: number; hotfixes: number }
interface CouplingAlert { sha: string; subject: string; filesChanged: number; insertions: number; deletions: number }
interface RepoReport {
  riskMatrix: FileRisk[];
  busFactor: Contributor[];
  firefightingIncidents: number;
  couplingAlerts: CouplingAlert[];
  sleepingGiants: SleepingGiant[];
  monthlyActivity: MonthlyActivity[];
}

// --- Dark Mode ---
const isDark = ref(true)

const toggleDarkMode = () => {
  isDark.value = !isDark.value
  document.documentElement.classList.toggle('dark', isDark.value)
}

onMounted(() => {
  document.documentElement.classList.toggle('dark', isDark.value)
})

// Chart colors that adapt to dark/light mode
const chartColors = computed(() => ({
  grid: isDark.value ? 'rgba(255,255,255,0.1)' : 'rgba(0,0,0,0.1)',
  text: isDark.value ? '#94a3b8' : '#64748b',
  border: isDark.value ? 'rgba(255,255,255,0.15)' : 'rgba(0,0,0,0.1)',
  legendText: isDark.value ? '#cbd5e1' : '#374151',
}))

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
    const res = await fetch(`/api/analyze?path=${encodeURIComponent(path.value)}`)
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

// --- Helpers ---
const shortName = (path: string) => path.split('/').pop() || path

const padMax = (values: number[], minPad = 1) => {
  const max = Math.max(...values, 0)
  return max + Math.max(Math.ceil(max * 0.2), minPad)
}

// --- 1. Risk Matrix: Scatter Plot (Churn vs Bugs) ---
const scatterData = computed(() => {
  if (!report.value?.riskMatrix?.length) return { datasets: [] }
  return {
    datasets: [{
      label: 'Files',
      backgroundColor: '#ef4444',
      pointRadius: 6,
      pointHoverRadius: 9,
      data: report.value.riskMatrix.map(file => ({
        x: file.churn,
        y: file.bugs,
        rawFile: file.name
      }))
    }]
  }
})

const scatterOptions = computed(() => {
  const rm = report.value?.riskMatrix || []
  const c = chartColors.value
  return {
    responsive: true,
    maintainAspectRatio: false,
    plugins: {
      legend: { display: false },
      tooltip: {
        callbacks: {
          label: (context: any) => {
            const point = context.raw
            return `${shortName(point.rawFile)} (Churn: ${point.x}, Bugs: ${point.y})`
          }
        }
      }
    },
    scales: {
      x: {
        title: { display: true, text: 'Total Commits (Churn)', color: c.text },
        beginAtZero: true,
        max: padMax(rm.map(f => f.churn)),
        ticks: { color: c.text },
        grid: { color: c.grid }
      },
      y: {
        title: { display: true, text: 'Bug Fixes', color: c.text },
        beginAtZero: true,
        max: padMax(rm.map(f => f.bugs)),
        ticks: { color: c.text },
        grid: { color: c.grid }
      }
    }
  }
})

// --- 2. Bus Factor: Donut Chart (Contributor Share) ---
const donutColors = ['#a855f7', '#3b82f6', '#ef4444', '#f59e0b', '#10b981', '#6366f1', '#ec4899', '#14b8a6']

const donutData = computed(() => {
  if (!report.value?.busFactor?.length) return { labels: [], datasets: [] }
  const sorted = [...report.value.busFactor].sort((a, b) => b.commits - a.commits)
  return {
    labels: sorted.map(c => c.name),
    datasets: [{
      data: sorted.map(c => c.commits),
      backgroundColor: sorted.map((_, i) => donutColors[i % donutColors.length]),
      borderWidth: 2,
      borderColor: isDark.value ? '#1e293b' : '#ffffff',
      hoverOffset: 8
    }]
  }
})

const donutOptions = computed(() => ({
  responsive: true,
  maintainAspectRatio: false,
  cutout: '55%',
  plugins: {
    legend: { position: 'bottom' as const, labels: { boxWidth: 12, padding: 16, color: chartColors.value.legendText } },
    tooltip: {
      callbacks: {
        label: (context: any) => {
          const total = context.dataset.data.reduce((a: number, b: number) => a + b, 0)
          const pct = ((context.parsed / total) * 100).toFixed(1)
          return `${context.label}: ${context.parsed} commits (${pct}%)`
        }
      }
    }
  }
}))

// --- 3. Sleeping Giants: Bubble Chart (Age vs Lines, size = complexity) ---
const bubbleData = computed(() => {
  if (!report.value?.sleepingGiants?.length) return { datasets: [] }
  return {
    datasets: [{
      label: 'Files',
      backgroundColor: 'rgba(99, 102, 241, 0.5)',
      borderColor: '#6366f1',
      borderWidth: 1,
      data: report.value.sleepingGiants.map(g => ({
        x: g.daysSinceLastCommit,
        y: g.lines,
        r: Math.max(4, Math.min(g.complexity * 2, 30)),
        rawFile: g.name,
        complexity: g.complexity
      }))
    }]
  }
})

const bubbleOptions = computed(() => {
  const sg = report.value?.sleepingGiants || []
  const c = chartColors.value
  return {
    responsive: true,
    maintainAspectRatio: false,
    plugins: {
      legend: { display: false },
      tooltip: {
        callbacks: {
          label: (context: any) => {
            const p = context.raw
            return [
              shortName(p.rawFile),
              `Lines: ${p.y}`,
              `Age: ${p.x} days`,
              `Complexity: ${p.complexity}`
            ]
          }
        }
      }
    },
    scales: {
      x: {
        title: { display: true, text: 'Days Since Last Commit', color: c.text },
        beginAtZero: true,
        max: padMax(sg.map(g => g.daysSinceLastCommit), 7),
        ticks: { color: c.text },
        grid: { color: c.grid }
      },
      y: {
        title: { display: true, text: 'Lines of Code', color: c.text },
        beginAtZero: true,
        max: padMax(sg.map(g => g.lines), 50),
        ticks: { color: c.text },
        grid: { color: c.grid }
      }
    }
  }
})

// --- 4. Team Momentum: Dual-Axis Line Chart (Commits + Hotfixes per Month) ---
const lineData = computed(() => {
  if (!report.value?.monthlyActivity?.length) return { labels: [], datasets: [] }
  const months = report.value.monthlyActivity
  return {
    labels: months.map(m => m.month),
    datasets: [
      {
        label: 'Commits',
        data: months.map(m => m.commits),
        borderColor: '#3b82f6',
        backgroundColor: 'rgba(59, 130, 246, 0.1)',
        fill: true,
        tension: 0.3,
        pointRadius: 5,
        pointHoverRadius: 7,
        pointBackgroundColor: '#3b82f6',
        yAxisID: 'y'
      },
      {
        label: 'Hotfixes / Reverts',
        data: months.map(m => m.hotfixes),
        borderColor: '#ef4444',
        backgroundColor: 'rgba(239, 68, 68, 0.1)',
        fill: true,
        tension: 0.3,
        borderDash: [5, 5],
        pointRadius: 5,
        pointHoverRadius: 7,
        pointBackgroundColor: '#ef4444',
        yAxisID: 'y1'
      }
    ]
  }
})

const lineOptions = computed(() => {
  const ma = report.value?.monthlyActivity || []
  const c = chartColors.value
  return {
    responsive: true,
    maintainAspectRatio: false,
    interaction: { mode: 'index' as const, intersect: false },
    plugins: {
      legend: { position: 'top' as const, labels: { boxWidth: 12, color: c.legendText } }
    },
    scales: {
      x: { title: { display: true, text: 'Month', color: c.text }, ticks: { color: c.text }, grid: { color: c.grid } },
      y: {
        type: 'linear' as const,
        display: true,
        position: 'left' as const,
        title: { display: true, text: 'Commits', color: c.text },
        beginAtZero: true,
        max: padMax(ma.map(m => m.commits), 2),
        ticks: { color: c.text },
        grid: { color: c.grid }
      },
      y1: {
        type: 'linear' as const,
        display: true,
        position: 'right' as const,
        title: { display: true, text: 'Hotfixes', color: c.text },
        beginAtZero: true,
        max: padMax(ma.map(m => m.hotfixes), 1),
        ticks: { color: c.text },
        grid: { drawOnChartArea: false }
      }
    }
  }
})
</script>

<template>
  <div class="min-h-screen bg-gray-50 dark:bg-gray-950 p-8 font-sans text-gray-900 dark:text-gray-100 transition-colors">
    <div class="max-w-7xl mx-auto space-y-6">
      
      <!-- Search Bar -->
      <div class="bg-white dark:bg-gray-900 p-6 rounded-xl shadow-sm border border-gray-200 dark:border-gray-800">
        <div class="flex items-center justify-between mb-2">
          <h1 class="text-3xl font-bold">Codebase Diagnostic Map</h1>
          <button
            @click="toggleDarkMode"
            class="p-2 rounded-lg bg-gray-100 dark:bg-gray-800 hover:bg-gray-200 dark:hover:bg-gray-700 transition-colors"
            :title="isDark ? 'Switch to light mode' : 'Switch to dark mode'"
          >
            <Sun v-if="isDark" :size="20" class="text-yellow-400" />
            <Moon v-else :size="20" class="text-gray-600" />
          </button>
        </div>
        <p class="text-gray-500 dark:text-gray-400 mb-6">Enter an absolute local path to generate structural and social metrics.</p>
        
        <div class="flex gap-4">
          <input 
            type="text" 
            v-model="path"
            @keydown.enter="handleAnalyze"
            placeholder="e.g., /Users/name/workspace/target-repo" 
            class="flex-1 p-3 border border-gray-300 dark:border-gray-700 bg-white dark:bg-gray-800 text-gray-900 dark:text-gray-100 rounded-lg focus:outline-none focus:ring-2 focus:ring-blue-500 placeholder-gray-400 dark:placeholder-gray-500"
          />
          <button 
            @click="handleAnalyze"
            :disabled="loading"
            class="px-6 py-3 bg-blue-600 hover:bg-blue-700 disabled:bg-blue-300 dark:disabled:bg-blue-900 text-white font-semibold rounded-lg transition-colors flex items-center gap-2"
          >
            {{ loading ? 'Scanning...' : 'Analyze Repository' }}
          </button>
        </div>
        <p v-if="error" class="text-red-500 mt-3 flex items-center gap-2">
          <AlertTriangle :size="18" /> {{ error }}
        </p>
      </div>

      <div v-if="report" class="space-y-6">

        <!-- Row 1: Risk Matrix + Bus Factor -->
        <div class="grid grid-cols-1 lg:grid-cols-3 gap-6">
          
          <!-- 1. Risk Matrix Scatter Plot -->
          <div class="lg:col-span-2 bg-white dark:bg-gray-900 p-6 rounded-xl shadow-sm border border-gray-200 dark:border-gray-800">
            <h2 class="text-xl font-semibold mb-1 flex items-center gap-2">
              <Code class="text-blue-500" /> Structural Risk Matrix
            </h2>
            <p class="text-sm text-gray-500 dark:text-gray-400 mb-6">Top-right quadrant = High Churn + High Bugs (Refactor Targets)</p>
            <div class="h-96 w-full relative">
              <Scatter :data="scatterData" :options="scatterOptions" />
            </div>
          </div>

          <!-- 2. Bus Factor Donut Chart -->
          <div class="bg-white dark:bg-gray-900 p-6 rounded-xl shadow-sm border border-gray-200 dark:border-gray-800">
            <h2 class="text-xl font-semibold mb-1 flex items-center gap-2">
              <Users class="text-purple-500" /> Bus Factor
            </h2>
            <p class="text-sm text-gray-500 dark:text-gray-400 mb-4">Commit share per contributor — one large slice signals a knowledge bottleneck</p>
            <div v-if="report.busFactor?.length" class="h-80 w-full relative">
              <Doughnut :data="donutData" :options="donutOptions" />
            </div>
            <p v-else class="text-gray-400 dark:text-gray-500 italic text-sm">No contributor data available.</p>
          </div>
        </div>

        <!-- Row 2: Sleeping Giants + Team Momentum -->
        <div class="grid grid-cols-1 lg:grid-cols-2 gap-6">

          <!-- 3. Sleeping Giants Bubble Chart -->
          <div class="bg-white dark:bg-gray-900 p-6 rounded-xl shadow-sm border border-gray-200 dark:border-gray-800">
            <h2 class="text-xl font-semibold mb-1 flex items-center gap-2">
              <Ghost class="text-indigo-500" /> Sleeping Giants
            </h2>
            <p class="text-sm text-gray-500 dark:text-gray-400 mb-4">Large bubbles in the top-right are complex, stale files the team may be avoiding</p>
            <div v-if="report.sleepingGiants?.length" class="h-96 w-full relative">
              <Bubble :data="bubbleData" :options="bubbleOptions" />
            </div>
            <p v-else class="text-gray-400 dark:text-gray-500 italic text-sm">No sleeping giants detected.</p>
          </div>

          <!-- 4. Team Momentum Line Chart -->
          <div class="bg-white dark:bg-gray-900 p-6 rounded-xl shadow-sm border border-gray-200 dark:border-gray-800">
            <h2 class="text-xl font-semibold mb-1 flex items-center gap-2">
              <TrendingUp class="text-blue-500" /> Team Momentum & Firefighting
            </h2>
            <p class="text-sm text-gray-500 dark:text-gray-400 mb-4">Drops in commits may signal departures — hotfix spikes indicate brittle areas</p>
            <div v-if="report.monthlyActivity?.length" class="h-96 w-full relative">
              <Line :data="lineData" :options="lineOptions" />
            </div>
            <p v-else class="text-gray-400 dark:text-gray-500 italic text-sm">Not enough history for trend analysis.</p>
          </div>
        </div>

        <!-- Row 3: Operational Health Summary -->
        <div class="bg-white dark:bg-gray-900 p-6 rounded-xl shadow-sm border border-gray-200 dark:border-gray-800">
          <h2 class="text-xl font-semibold mb-4 flex items-center gap-2">
            <Flame class="text-orange-500" /> Operational Health
          </h2>
          <div class="grid grid-cols-1 md:grid-cols-2 gap-6">
            <div class="flex items-center justify-between p-4 bg-orange-50 dark:bg-orange-950 rounded-lg border border-orange-100 dark:border-orange-900">
              <span class="text-orange-900 dark:text-orange-200 font-medium">Total Hotfixes / Reverts (12 mo)</span>
              <span class="text-3xl font-bold text-orange-600 dark:text-orange-400">{{ report.firefightingIncidents }}</span>
            </div>
            <div v-if="report.couplingAlerts?.length" class="space-y-2">
              <h3 class="text-sm font-semibold text-gray-700 dark:text-gray-300 uppercase tracking-wider">Highest Blast-Radius Commits</h3>
              <ul class="text-sm space-y-2 text-gray-600 dark:text-gray-400">
                <li v-for="(alert, index) in report.couplingAlerts" :key="index" class="p-3 bg-gray-50 dark:bg-gray-800 rounded border border-gray-100 dark:border-gray-700">
                  <div class="flex items-center gap-2 mb-1">
                    <code class="text-xs bg-gray-200 dark:bg-gray-700 text-gray-700 dark:text-gray-300 px-1.5 py-0.5 rounded font-mono">{{ alert.sha.substring(0, 8) }}</code>
                    <span class="text-sm text-gray-800 dark:text-gray-200 truncate">{{ alert.subject }}</span>
                  </div>
                  <div class="flex gap-3 text-xs text-gray-500 dark:text-gray-400">
                    <span>{{ alert.filesChanged }} files</span>
                    <span class="text-green-600 dark:text-green-400">+{{ alert.insertions }}</span>
                    <span class="text-red-500 dark:text-red-400">-{{ alert.deletions }}</span>
                  </div>
                </li>
              </ul>
            </div>
          </div>
        </div>

      </div>
    </div>
  </div>
</template>