<template>
  <AppLayout>
    <div class="space-y-6">
      <div class="flex flex-col gap-3 sm:flex-row sm:items-center sm:justify-between">
        <div>
          <h1 class="text-2xl font-semibold text-gray-900 dark:text-white">QuotaNet Nodes</h1>
          <p class="mt-1 text-sm text-gray-500 dark:text-dark-300">Client node sessions, capacity and task dispatch debugging.</p>
        </div>
        <div class="flex flex-wrap gap-2">
          <button class="btn btn-secondary" :disabled="loading" @click="reload">
            Refresh
          </button>
          <button class="btn btn-primary" :disabled="!selectedNodeID" @click="openDispatchDialog(selectedNodeID)">
            Dispatch Test
          </button>
        </div>
      </div>

      <div class="grid gap-4 md:grid-cols-2 xl:grid-cols-4">
        <div v-for="card in overviewCards" :key="card.label" class="rounded-lg border border-gray-200 bg-white p-4 dark:border-dark-700 dark:bg-dark-900">
          <p class="text-xs font-medium uppercase text-gray-500 dark:text-dark-400">{{ card.label }}</p>
          <p class="mt-2 text-2xl font-semibold text-gray-900 dark:text-white">{{ card.value }}</p>
          <p class="mt-1 text-sm text-gray-500 dark:text-dark-300">{{ card.detail }}</p>
        </div>
      </div>

      <div class="grid gap-6 xl:grid-cols-[minmax(0,1.15fr)_minmax(360px,0.85fr)]">
        <section class="rounded-lg border border-gray-200 bg-white dark:border-dark-700 dark:bg-dark-900">
          <div class="flex flex-col gap-3 border-b border-gray-200 p-4 dark:border-dark-700 sm:flex-row sm:items-center sm:justify-between">
            <div>
              <h2 class="text-base font-semibold text-gray-900 dark:text-white">Online Sessions</h2>
              <p class="text-sm text-gray-500 dark:text-dark-300">{{ sessions.length }} sessions reported by registry</p>
            </div>
            <select v-model="statusFilter" class="input w-full sm:w-40">
              <option value="">All statuses</option>
              <option value="ready">Ready</option>
              <option value="busy">Busy</option>
              <option value="offline">Offline</option>
            </select>
          </div>
          <DataTable :columns="sessionColumns" :data="filteredSessions" :loading="loading">
            <template #cell-status="{ row }">
              <span :class="statusBadgeClass(row.status)">{{ row.status || 'ready' }}</span>
            </template>
            <template #cell-node="{ row }">
              <div class="min-w-0">
                <p class="font-medium text-gray-900 dark:text-white">#{{ row.node_id }} {{ nodeName(row.node_id) }}</p>
                <p class="truncate text-xs text-gray-500 dark:text-dark-400">{{ row.session_id }}</p>
              </div>
            </template>
            <template #cell-capacity="{ row }">
              <span>{{ row.current_concurrency }} / {{ row.max_concurrency }}</span>
            </template>
            <template #cell-queue="{ row }">
              <span>{{ row.queue_size }} / {{ row.max_queue_size }}</span>
            </template>
            <template #cell-models="{ row }">
              <div class="max-w-md whitespace-normal text-xs text-gray-600 dark:text-dark-300">
                {{ capabilitySummary(row.capabilities) }}
              </div>
            </template>
            <template #cell-actions="{ row }">
              <div class="flex flex-wrap gap-2">
                <button class="btn btn-secondary btn-sm" @click="selectNode(row.node_id)">Tasks</button>
                <button class="btn btn-primary btn-sm" @click="openDispatchDialog(row.node_id)">Dispatch</button>
              </div>
            </template>
            <template #empty>
              <EmptyState title="No sessions" description="No QuotaNet Client nodes are connected right now." />
            </template>
          </DataTable>
        </section>

        <section class="rounded-lg border border-gray-200 bg-white dark:border-dark-700 dark:bg-dark-900">
          <div class="border-b border-gray-200 p-4 dark:border-dark-700">
            <h2 class="text-base font-semibold text-gray-900 dark:text-white">Providers</h2>
            <p class="text-sm text-gray-500 dark:text-dark-300">Available capability summary from connected nodes</p>
          </div>
          <div class="space-y-3 p-4">
            <div v-if="providers.length === 0" class="text-sm text-gray-500 dark:text-dark-300">No provider capability reported.</div>
            <div v-for="provider in providers" :key="provider.provider" class="rounded-md border border-gray-200 p-3 dark:border-dark-700">
              <div class="font-medium text-gray-900 dark:text-white">{{ provider.provider }}</div>
              <div class="mt-2 flex flex-wrap gap-1.5">
                <span v-for="model in provider.models" :key="model" class="rounded bg-gray-100 px-2 py-1 text-xs text-gray-700 dark:bg-dark-800 dark:text-dark-200">
                  {{ model }}
                </span>
              </div>
            </div>
          </div>
        </section>
      </div>

      <section class="rounded-lg border border-gray-200 bg-white dark:border-dark-700 dark:bg-dark-900">
        <div class="flex flex-col gap-3 border-b border-gray-200 p-4 dark:border-dark-700 md:flex-row md:items-center md:justify-between">
          <div>
            <h2 class="text-base font-semibold text-gray-900 dark:text-white">Task Debugger</h2>
            <p class="text-sm text-gray-500 dark:text-dark-300">{{ selectedNodeID ? `Showing tasks for node #${selectedNodeID}` : 'Showing recent QuotaNet tasks' }}</p>
          </div>
          <div class="flex flex-wrap gap-2">
            <select v-model="selectedNodeIDText" class="input w-44" @change="resetAndLoadTasks">
              <option value="">All nodes</option>
              <option v-for="node in nodeOptions" :key="node.id" :value="String(node.id)">#{{ node.id }} {{ node.name }}</option>
            </select>
            <select v-model="taskStatusFilter" class="input w-40" @change="resetAndLoadTasks">
              <option value="">All statuses</option>
              <option value="queued">Queued</option>
              <option value="running">Running</option>
              <option value="success">Success</option>
              <option value="failed">Failed</option>
              <option value="timeout">Timeout</option>
            </select>
            <button class="btn btn-secondary" :disabled="tasksLoading" @click="loadTasks">Reload Tasks</button>
          </div>
        </div>
        <DataTable :columns="taskColumns" :data="tasks" :loading="tasksLoading">
          <template #cell-task="{ row }">
            <div>
              <p class="font-medium text-gray-900 dark:text-white">{{ row.task_id }}</p>
              <p class="text-xs text-gray-500 dark:text-dark-400">{{ row.request_id }}</p>
            </div>
          </template>
          <template #cell-node_id="{ value }">
            <span>{{ value ? `#${value}` : '-' }}</span>
          </template>
          <template #cell-status="{ row }">
            <span :class="statusBadgeClass(row.status)">{{ row.status }}</span>
          </template>
          <template #cell-error="{ row }">
            <span class="whitespace-normal text-xs text-red-600 dark:text-red-300">{{ row.error_code || row.error_message || '-' }}</span>
          </template>
          <template #cell-created_at="{ value }">
            <span>{{ formatTime(value) }}</span>
          </template>
          <template #empty>
            <EmptyState title="No tasks" description="No tasks match the current filter." />
          </template>
        </DataTable>
        <div v-if="taskPagination.total > taskPagination.page_size" class="border-t border-gray-200 p-4 dark:border-dark-700">
          <Pagination
            :page="taskPagination.page"
            :total="taskPagination.total"
            :page-size="taskPagination.page_size"
            @update:page="onTaskPageChange"
            @update:pageSize="onTaskPageSizeChange"
          />
        </div>
      </section>
    </div>

    <BaseDialog :show="showDispatchDialog" title="Dispatch QuotaNet Task" width="wide" @close="closeDispatchDialog">
      <div class="grid gap-4 md:grid-cols-2">
        <Input v-model="dispatchForm.nodeID" label="Node ID" readonly />
        <Input v-model="dispatchForm.platform" label="Provider" required />
        <Input v-model="dispatchForm.endpoint" label="Endpoint" required />
        <Input v-model="dispatchForm.model" label="Model" required />
        <Input v-model="dispatchForm.timeoutSeconds" label="Timeout Seconds" type="number" />
        <label class="flex items-center gap-2 text-sm text-gray-700 dark:text-dark-200">
          <input v-model="dispatchForm.sync" type="checkbox" class="rounded border-gray-300 text-primary-600 focus:ring-primary-500" />
          Wait for response
        </label>
      </div>
      <div class="mt-4">
        <TextArea v-model="dispatchForm.payloadText" label="Payload JSON" :rows="8" :error="payloadError" />
      </div>
      <div v-if="dispatchResult" class="mt-4 rounded-md border border-gray-200 bg-gray-50 p-3 dark:border-dark-700 dark:bg-dark-800">
        <pre class="max-h-72 overflow-auto text-xs text-gray-800 dark:text-dark-100">{{ dispatchResult }}</pre>
      </div>
      <template #footer>
        <button class="btn btn-secondary" @click="closeDispatchDialog">Cancel</button>
        <button class="btn btn-primary" :disabled="dispatching" @click="submitDispatch">
          {{ dispatching ? 'Dispatching...' : 'Dispatch' }}
        </button>
      </template>
    </BaseDialog>
  </AppLayout>
</template>

<script setup lang="ts">
import { computed, onMounted, reactive, ref } from 'vue'
import { useAppStore } from '@/stores/app'
import { adminAPI } from '@/api/admin'
import type { QuotaNetCapability, QuotaNetNode, QuotaNetNodeOverview, QuotaNetSession, QuotaNetTask, QuotaNetTaskDispatchRequest } from '@/api/admin/quotanet'
import type { Column } from '@/components/common/types'
import { extractApiErrorMessage } from '@/utils/apiError'
import AppLayout from '@/components/layout/AppLayout.vue'
import BaseDialog from '@/components/common/BaseDialog.vue'
import DataTable from '@/components/common/DataTable.vue'
import EmptyState from '@/components/common/EmptyState.vue'
import Input from '@/components/common/Input.vue'
import Pagination from '@/components/common/Pagination.vue'
import TextArea from '@/components/common/TextArea.vue'

const appStore = useAppStore()

const loading = ref(false)
const tasksLoading = ref(false)
const nodes = ref<QuotaNetNode[]>([])
const sessions = ref<QuotaNetSession[]>([])
const overview = ref<QuotaNetNodeOverview | null>(null)
const tasks = ref<QuotaNetTask[]>([])
const statusFilter = ref('')
const taskStatusFilter = ref('')
const selectedNodeIDText = ref('')
const showDispatchDialog = ref(false)
const dispatching = ref(false)
const dispatchResult = ref('')
const payloadError = ref('')
const taskPagination = reactive({ page: 1, page_size: 20, total: 0 })
const dispatchForm = reactive({
  nodeID: '',
  platform: 'openai',
  endpoint: '/v1/chat/completions',
  model: 'gpt-4.1',
  timeoutSeconds: '60',
  sync: true,
  payloadText: JSON.stringify({ messages: [{ role: 'user', content: 'ping' }] }, null, 2)
})

const selectedNodeID = computed(() => {
  const id = Number(selectedNodeIDText.value)
  return Number.isFinite(id) && id > 0 ? id : null
})

const providers = computed(() => overview.value?.providers || [])
const nodeOptions = computed(() => nodes.value.length > 0 ? nodes.value : uniqueNodesFromSessions.value)
const uniqueNodesFromSessions = computed<QuotaNetNode[]>(() => {
  const map = new Map<number, QuotaNetNode>()
  for (const session of sessions.value) {
    if (!map.has(session.node_id)) {
      map.set(session.node_id, {
        id: session.node_id,
        node_key: session.node_key,
        name: session.node_key,
        wallet_address: session.wallet_address,
        status: session.status || 'ready',
      })
    }
  }
  return Array.from(map.values())
})

const overviewCards = computed(() => {
  const s = overview.value?.sessions
  const c = overview.value?.capacity
  const statuses = overview.value?.task_statuses || {}
  return [
    { label: 'Connected', value: s?.connected ?? 0, detail: `ready ${s?.ready ?? 0} / busy ${s?.busy ?? 0} / stale ${s?.stale ?? 0}` },
    { label: 'Capacity', value: c ? `${c.available}/${c.max_concurrency}` : '0/0', detail: `running ${c?.current_concurrency ?? 0}, queued ${c?.queue_size ?? 0}/${c?.max_queue_size ?? 0}` },
    { label: 'Providers', value: providers.value.length, detail: `${providers.value.reduce((sum, item) => sum + item.models.length, 0)} models available` },
    { label: 'Tasks', value: statuses.running || 0, detail: `queued ${statuses.queued || 0}, success ${statuses.success || 0}, failed ${statuses.failed || 0}` },
  ]
})

const filteredSessions = computed(() => {
  if (!statusFilter.value) return sessions.value
  return sessions.value.filter((item) => (item.status || 'ready') === statusFilter.value)
})

const sessionColumns: Column[] = [
  { key: 'status', label: 'Status' },
  { key: 'node', label: 'Node' },
  { key: 'capacity', label: 'Capacity' },
  { key: 'queue', label: 'Queue' },
  { key: 'models', label: 'Capabilities' },
  { key: 'last_heartbeat_at', label: 'Heartbeat', formatter: (value) => formatTime(value) },
  { key: 'actions', label: 'Actions' },
]

const taskColumns: Column[] = [
  { key: 'task', label: 'Task' },
  { key: 'node_id', label: 'Node' },
  { key: 'platform', label: 'Provider' },
  { key: 'model', label: 'Model' },
  { key: 'status', label: 'Status' },
  { key: 'error', label: 'Error' },
  { key: 'created_at', label: 'Created' },
]

async function reload() {
  loading.value = true
  try {
    const [overviewRes, sessionsRes, nodesRes] = await Promise.all([
      adminAPI.quotanet.getNodeOverview(),
      adminAPI.quotanet.listSessions(),
      adminAPI.quotanet.listNodes({ page: 1, page_size: 100 }),
    ])
    overview.value = overviewRes
    sessions.value = sessionsRes.items || []
    nodes.value = nodesRes.items || []
    await loadTasks()
  } catch (err) {
    appStore.showError(extractApiErrorMessage(err, 'Failed to load QuotaNet nodes'))
  } finally {
    loading.value = false
  }
}

async function loadTasks() {
  tasksLoading.value = true
  try {
    const params = {
      page: taskPagination.page,
      page_size: taskPagination.page_size,
      status: taskStatusFilter.value || undefined,
    }
    const res = selectedNodeID.value
      ? await adminAPI.quotanet.listNodeTasks(selectedNodeID.value, params)
      : await adminAPI.quotanet.listTasks(params)
    tasks.value = res.items || []
    taskPagination.total = res.total || 0
  } catch (err) {
    appStore.showError(extractApiErrorMessage(err, 'Failed to load QuotaNet tasks'))
  } finally {
    tasksLoading.value = false
  }
}

function selectNode(nodeID: number) {
  selectedNodeIDText.value = String(nodeID)
  taskPagination.page = 1
  loadTasks()
}

function openDispatchDialog(nodeID: number | null) {
  if (!nodeID) return
  selectedNodeIDText.value = String(nodeID)
  dispatchForm.nodeID = String(nodeID)
  const session = sessions.value.find((item) => item.node_id === nodeID)
  const firstCapability = session?.capabilities?.[0]
  if (firstCapability) {
    dispatchForm.platform = firstCapability.provider || dispatchForm.platform
    dispatchForm.model = firstCapability.models?.[0] || dispatchForm.model
  }
  payloadError.value = ''
  dispatchResult.value = ''
  showDispatchDialog.value = true
}

function closeDispatchDialog() {
  if (dispatching.value) return
  showDispatchDialog.value = false
}

async function submitDispatch() {
  payloadError.value = ''
  let payload: Record<string, unknown>
  try {
    payload = JSON.parse(dispatchForm.payloadText || '{}') as Record<string, unknown>
  } catch {
    payloadError.value = 'Payload must be valid JSON.'
    return
  }
  const req: QuotaNetTaskDispatchRequest = {
    node_id: Number(dispatchForm.nodeID),
    platform: dispatchForm.platform.trim(),
    endpoint: dispatchForm.endpoint.trim(),
    model: dispatchForm.model.trim(),
    timeout_seconds: Number(dispatchForm.timeoutSeconds) || 60,
    payload,
  }
  dispatching.value = true
  try {
    const res = dispatchForm.sync
      ? await adminAPI.quotanet.dispatchTaskSync(req)
      : await adminAPI.quotanet.dispatchTask(req)
    dispatchResult.value = JSON.stringify(res, null, 2)
    appStore.showSuccess('QuotaNet task dispatched')
    await reload()
  } catch (err) {
    appStore.showError(extractApiErrorMessage(err, 'QuotaNet dispatch failed'))
  } finally {
    dispatching.value = false
  }
}

function onTaskPageChange(page: number) {
  taskPagination.page = page
  loadTasks()
}

function resetAndLoadTasks() {
  taskPagination.page = 1
  loadTasks()
}

function onTaskPageSizeChange(pageSize: number) {
  taskPagination.page_size = pageSize
  taskPagination.page = 1
  loadTasks()
}

function nodeName(nodeID: number): string {
  return nodes.value.find((node) => node.id === nodeID)?.name || ''
}

function capabilitySummary(capabilities: QuotaNetCapability[] = []): string {
  if (!capabilities.length) return '-'
  return capabilities.map((item) => `${item.provider}: ${(item.models || []).join(', ')}`).join(' / ')
}

function formatTime(value?: string | null): string {
  if (!value) return '-'
  const date = new Date(value)
  if (Number.isNaN(date.getTime())) return value
  return date.toLocaleString()
}

function statusBadgeClass(status?: string): string {
  const base = 'inline-flex rounded-md px-2 py-0.5 text-xs font-medium'
  switch (status) {
    case 'ready':
    case 'success':
      return `${base} bg-green-100 text-green-700 dark:bg-green-900/30 dark:text-green-300`
    case 'busy':
    case 'running':
      return `${base} bg-blue-100 text-blue-700 dark:bg-blue-900/30 dark:text-blue-300`
    case 'queued':
      return `${base} bg-amber-100 text-amber-700 dark:bg-amber-900/30 dark:text-amber-300`
    case 'failed':
    case 'timeout':
    case 'offline':
      return `${base} bg-red-100 text-red-700 dark:bg-red-900/30 dark:text-red-300`
    default:
      return `${base} bg-gray-100 text-gray-700 dark:bg-dark-800 dark:text-dark-200`
  }
}

onMounted(() => {
  reload()
})
</script>
