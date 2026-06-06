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
          <button class="btn btn-secondary" @click="openCreateNodeDialog">
            Register Node
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
          <DataTable :columns="sessionColumns" :data="filteredSessions" :loading="loading" :virtualized="false">
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

        <div class="space-y-6">
          <section class="rounded-lg border border-gray-200 bg-white dark:border-dark-700 dark:bg-dark-900">
            <div class="flex items-center justify-between gap-3 border-b border-gray-200 p-4 dark:border-dark-700">
              <div>
                <h2 class="text-base font-semibold text-gray-900 dark:text-white">Registered Nodes</h2>
                <p class="text-sm text-gray-500 dark:text-dark-300">{{ nodes.length }} records in database</p>
              </div>
            </div>
            <div class="max-h-[360px] space-y-3 overflow-auto p-4">
              <div v-if="nodes.length === 0" class="text-sm text-gray-500 dark:text-dark-300">No nodes registered.</div>
              <div v-for="node in nodes" :key="node.id" class="rounded-md border border-gray-200 p-3 dark:border-dark-700">
                <div class="flex items-start justify-between gap-3">
                  <div class="min-w-0">
                    <div class="font-medium text-gray-900 dark:text-white">#{{ node.id }} {{ node.name }}</div>
                    <div class="mt-1 truncate text-xs text-gray-500 dark:text-dark-400">{{ node.node_key }}</div>
                    <div class="mt-1 truncate text-xs text-gray-500 dark:text-dark-400">{{ node.wallet_address }}</div>
                  </div>
                  <span :class="statusBadgeClass(node.status)">{{ node.status }}</span>
                </div>
                <div class="mt-3 flex flex-wrap gap-2">
                  <button class="btn btn-secondary btn-sm" @click="selectNode(node.id)">Tasks</button>
                  <button class="btn btn-secondary btn-sm" @click="confirmResetToken(node)">Reset Token</button>
                </div>
              </div>
            </div>
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
            <button class="btn btn-secondary" :disabled="tasksLoading" @click="showTimeoutSweepConfirm = true">Sweep Timeouts</button>
          </div>
        </div>
        <DataTable :columns="taskColumns" :data="tasks" :loading="tasksLoading" :virtualized="false">
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
          <template #cell-actions="{ row }">
            <button class="btn btn-secondary btn-sm" @click="openTaskEvents(row)">Events</button>
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

    <BaseDialog :show="showNodeDialog" title="Register QuotaNet Node" width="normal" @close="closeNodeDialog">
      <div v-if="!issuedToken" class="space-y-4">
        <Input v-model="nodeForm.name" label="Node Name" required />
        <Input v-model="nodeForm.walletAddress" label="Wallet Address" required />
        <Input v-model="nodeForm.ownerUserID" label="Owner User ID" type="number" />
        <select v-model="nodeForm.status" class="input w-full">
          <option value="active">Active</option>
          <option value="pending">Pending</option>
          <option value="disabled">Disabled</option>
        </select>
      </div>
      <div v-if="issuedToken" class="mt-4 rounded-md border border-amber-200 bg-amber-50 p-3 dark:border-amber-800 dark:bg-amber-900/20">
        <div class="text-sm font-medium text-amber-900 dark:text-amber-100">Node token is shown once.</div>
        <pre class="mt-2 max-h-40 overflow-auto whitespace-pre-wrap break-all text-xs text-amber-900 dark:text-amber-100">{{ issuedToken }}</pre>
        <button class="btn btn-secondary btn-sm mt-3" @click="copyIssuedToken">Copy Token</button>
      </div>
      <template #footer>
        <button class="btn btn-secondary" @click="closeNodeDialog">Close</button>
        <button v-if="!issuedToken" class="btn btn-primary" :disabled="nodeSubmitting" @click="submitCreateNode">
          {{ nodeSubmitting ? 'Creating...' : 'Create Node' }}
        </button>
      </template>
    </BaseDialog>

    <BaseDialog :show="showEventsDialog" title="QuotaNet Task Events" width="wide" @close="showEventsDialog = false">
      <div v-if="selectedTask" class="mb-4 rounded-md border border-gray-200 bg-gray-50 p-3 text-sm dark:border-dark-700 dark:bg-dark-800">
        <div class="font-medium text-gray-900 dark:text-white">{{ selectedTask.task_id }}</div>
        <div class="mt-1 text-gray-500 dark:text-dark-300">
          node {{ selectedTask.node_id ? `#${selectedTask.node_id}` : '-' }} / {{ selectedTask.platform }} / {{ selectedTask.model }} / {{ selectedTask.status }}
        </div>
      </div>
      <div v-if="eventsLoading" class="py-8 text-center text-sm text-gray-500 dark:text-dark-300">Loading events...</div>
      <div v-else-if="taskEvents.length === 0" class="py-8 text-center text-sm text-gray-500 dark:text-dark-300">No events recorded for this task.</div>
      <div v-else class="space-y-3">
        <div v-for="event in taskEvents" :key="event.id" class="rounded-md border border-gray-200 p-3 dark:border-dark-700">
          <div class="flex flex-wrap items-center justify-between gap-2">
            <div class="font-medium text-gray-900 dark:text-white">#{{ event.sequence }} {{ event.event_type }}</div>
            <div class="text-xs text-gray-500 dark:text-dark-400">{{ formatTime(event.created_at) }}</div>
          </div>
          <pre class="mt-3 max-h-64 overflow-auto rounded bg-gray-50 p-3 text-xs text-gray-800 dark:bg-dark-800 dark:text-dark-100">{{ formatJSON(event.payload) }}</pre>
        </div>
      </div>
      <template #footer>
        <button class="btn btn-secondary" @click="showEventsDialog = false">Close</button>
      </template>
    </BaseDialog>

    <ConfirmDialog
      :show="showResetTokenConfirm"
      title="Reset Node Token"
      :message="resetTokenMessage"
      confirm-text="Reset Token"
      cancel-text="Cancel"
      :danger="true"
      @confirm="resetSelectedNodeToken"
      @cancel="showResetTokenConfirm = false"
    />

    <ConfirmDialog
      :show="showTimeoutSweepConfirm"
      title="Sweep Running Tasks"
      message="Mark running QuotaNet tasks dispatched more than 5 minutes ago as timeout? This is useful when a client node disconnected or never returned a response."
      confirm-text="Sweep"
      cancel-text="Cancel"
      :danger="true"
      @confirm="runTimeoutSweep"
      @cancel="showTimeoutSweepConfirm = false"
    />
  </AppLayout>
</template>

<script setup lang="ts">
import { computed, onMounted, reactive, ref } from 'vue'
import { useAppStore } from '@/stores/app'
import { adminAPI } from '@/api/admin'
import type { QuotaNetCapability, QuotaNetNode, QuotaNetNodeOverview, QuotaNetSession, QuotaNetTask, QuotaNetTaskDispatchRequest, QuotaNetTaskEvent } from '@/api/admin/quotanet'
import type { Column } from '@/components/common/types'
import { useClipboard } from '@/composables/useClipboard'
import { extractApiErrorMessage } from '@/utils/apiError'
import AppLayout from '@/components/layout/AppLayout.vue'
import BaseDialog from '@/components/common/BaseDialog.vue'
import ConfirmDialog from '@/components/common/ConfirmDialog.vue'
import DataTable from '@/components/common/DataTable.vue'
import EmptyState from '@/components/common/EmptyState.vue'
import Input from '@/components/common/Input.vue'
import Pagination from '@/components/common/Pagination.vue'
import TextArea from '@/components/common/TextArea.vue'

const appStore = useAppStore()
const { copyToClipboard } = useClipboard()

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
const showEventsDialog = ref(false)
const eventsLoading = ref(false)
const selectedTask = ref<QuotaNetTask | null>(null)
const taskEvents = ref<QuotaNetTaskEvent[]>([])
const showNodeDialog = ref(false)
const nodeSubmitting = ref(false)
const issuedToken = ref('')
const showResetTokenConfirm = ref(false)
const resettingNode = ref<QuotaNetNode | null>(null)
const showTimeoutSweepConfirm = ref(false)
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
const nodeForm = reactive({
  name: '',
  walletAddress: '',
  ownerUserID: '',
  status: 'active'
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
const resetTokenMessage = computed(() => {
  const node = resettingNode.value
  return node
    ? `Reset token for node #${node.id} ${node.name}? Existing clients using the old token will no longer be able to reconnect.`
    : 'Reset this node token?'
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
  { key: 'actions', label: 'Actions' },
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

function openCreateNodeDialog() {
  nodeForm.name = ''
  nodeForm.walletAddress = ''
  nodeForm.ownerUserID = ''
  nodeForm.status = 'active'
  issuedToken.value = ''
  showNodeDialog.value = true
}

function closeNodeDialog() {
  if (nodeSubmitting.value) return
  showNodeDialog.value = false
  issuedToken.value = ''
}

async function submitCreateNode() {
  nodeSubmitting.value = true
  try {
    const ownerID = Number(nodeForm.ownerUserID)
    const res = await adminAPI.quotanet.createNode({
      name: nodeForm.name.trim(),
      wallet_address: nodeForm.walletAddress.trim(),
      owner_user_id: Number.isFinite(ownerID) && ownerID > 0 ? ownerID : undefined,
      status: nodeForm.status
    })
    issuedToken.value = res.token
    appStore.showSuccess('QuotaNet node created')
    await reload()
  } catch (err) {
    appStore.showError(extractApiErrorMessage(err, 'Failed to create QuotaNet node'))
  } finally {
    nodeSubmitting.value = false
  }
}

function confirmResetToken(node: QuotaNetNode) {
  resettingNode.value = node
  showResetTokenConfirm.value = true
}

async function resetSelectedNodeToken() {
  const node = resettingNode.value
  if (!node) return
  showResetTokenConfirm.value = false
  try {
    const res = await adminAPI.quotanet.resetNodeToken(node.id)
    nodeForm.name = node.name
    nodeForm.walletAddress = node.wallet_address
    nodeForm.ownerUserID = node.owner_user_id ? String(node.owner_user_id) : ''
    nodeForm.status = node.status
    issuedToken.value = res.token
    showNodeDialog.value = true
    appStore.showSuccess('QuotaNet node token reset')
    await reload()
  } catch (err) {
    appStore.showError(extractApiErrorMessage(err, 'Failed to reset QuotaNet node token'))
  } finally {
    resettingNode.value = null
  }
}

function copyIssuedToken() {
  copyToClipboard(issuedToken.value, 'Node token copied')
}

async function runTimeoutSweep() {
  showTimeoutSweepConfirm.value = false
  try {
    const res = await adminAPI.quotanet.timeoutSweep({ older_than_seconds: 300, limit: 100 })
    appStore.showSuccess(`Marked ${res.count} QuotaNet tasks as timeout`)
    await reload()
  } catch (err) {
    appStore.showError(extractApiErrorMessage(err, 'Failed to sweep QuotaNet timeouts'))
  }
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

async function openTaskEvents(task: QuotaNetTask) {
  selectedTask.value = task
  taskEvents.value = []
  showEventsDialog.value = true
  eventsLoading.value = true
  try {
    const res = await adminAPI.quotanet.getTaskEvents(task.task_id)
    taskEvents.value = res.items || []
  } catch (err) {
    appStore.showError(extractApiErrorMessage(err, 'Failed to load QuotaNet task events'))
  } finally {
    eventsLoading.value = false
  }
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

function formatJSON(value: unknown): string {
  try {
    return JSON.stringify(value ?? {}, null, 2)
  } catch {
    return String(value)
  }
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
