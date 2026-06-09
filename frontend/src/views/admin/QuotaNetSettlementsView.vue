<template>
  <AppLayout>
    <div class="space-y-6">
      <div class="flex flex-col gap-3 lg:flex-row lg:items-center lg:justify-between">
        <div>
          <h1 class="text-2xl font-semibold text-gray-900 dark:text-white">QuotaNet Settlements</h1>
          <p class="mt-1 text-sm text-gray-500 dark:text-dark-300">Contribution ledger, wallet aggregation and manual settlement batches.</p>
        </div>
        <div class="flex flex-wrap gap-2">
          <button class="btn btn-secondary" :disabled="loading" @click="reload">Refresh</button>
          <button class="btn btn-primary" @click="openBatchDialog">Create Batch</button>
        </div>
      </div>

      <section class="rounded-lg border border-gray-200 bg-white dark:border-dark-700 dark:bg-dark-900">
        <div class="grid gap-4 p-4 lg:grid-cols-[minmax(0,1fr)_minmax(320px,420px)]">
          <div class="grid gap-4 md:grid-cols-3">
            <div v-for="card in summaryCards" :key="card.label" class="rounded-md border border-gray-200 p-4 dark:border-dark-700">
              <p class="text-xs font-medium uppercase text-gray-500 dark:text-dark-400">{{ card.label }}</p>
              <p class="mt-2 text-2xl font-semibold text-gray-900 dark:text-white">{{ card.value }}</p>
              <p class="mt-1 text-sm text-gray-500 dark:text-dark-300">{{ card.detail }}</p>
            </div>
          </div>
          <form class="rounded-md border border-gray-200 p-4 dark:border-dark-700" @submit.prevent="saveConfig">
            <div class="flex items-center justify-between gap-3">
              <div>
                <h2 class="text-base font-semibold text-gray-900 dark:text-white">Settlement Config</h2>
                <p class="text-sm text-gray-500 dark:text-dark-300">Contribution USD is calculated from Sub2API billing rules.</p>
              </div>
              <button class="btn btn-secondary btn-sm" :disabled="savingConfig" type="submit">
                {{ savingConfig ? 'Saving...' : 'Save' }}
              </button>
            </div>
            <div class="mt-4 grid gap-3">
              <label class="space-y-1">
                <span class="text-xs font-medium text-gray-500 dark:text-dark-300">Settlement Method</span>
                <input v-model="configForm.network" class="input w-full" placeholder="manual" />
              </label>
            </div>
          </form>
        </div>
      </section>

      <div class="grid gap-6 xl:grid-cols-[minmax(0,1fr)_minmax(360px,0.55fr)]">
        <section class="rounded-lg border border-gray-200 bg-white dark:border-dark-700 dark:bg-dark-900">
          <div class="flex flex-col gap-3 border-b border-gray-200 p-4 dark:border-dark-700 md:flex-row md:items-center md:justify-between">
            <div>
              <h2 class="text-base font-semibold text-gray-900 dark:text-white">Contribution Ledger</h2>
              <p class="text-sm text-gray-500 dark:text-dark-300">Successful QuotaNet tasks create one pending ledger row.</p>
            </div>
            <div class="flex flex-wrap gap-2">
              <select v-model="ledgerFilter.status" class="input w-36" @change="resetLedgers">
                <option value="">All statuses</option>
                <option value="pending">Pending</option>
                <option value="finalized">Finalized</option>
                <option value="failed">Failed</option>
              </select>
              <input v-model.trim="ledgerFilter.wallet_address" class="input w-56" placeholder="Wallet address" @keyup.enter="resetLedgers" />
              <button class="btn btn-secondary" @click="resetLedgers">Apply</button>
            </div>
          </div>
          <DataTable :columns="ledgerColumns" :data="ledgers" :loading="loading" :virtualized="false">
            <template #cell-task="{ row }">
              <div class="max-w-[260px]">
                <p class="truncate font-mono text-xs text-gray-900 dark:text-white">{{ row.task_id }}</p>
                <p class="text-xs text-gray-500 dark:text-dark-400">node #{{ row.node_id }}</p>
              </div>
            </template>
            <template #cell-wallet_address="{ value }">
              <span class="font-mono text-xs">{{ shortText(value) }}</span>
            </template>
            <template #cell-token_flow="{ value }">{{ formatNumber(value) }}</template>
            <template #cell-contribution_usd="{ value }">{{ formatUSD(value) }}</template>
            <template #cell-rate="{ value }">{{ formatMultiplier(value) }}</template>
            <template #cell-status="{ value }">
              <span :class="statusBadgeClass(value)">{{ value }}</span>
            </template>
            <template #cell-created_at="{ value }">{{ formatTime(value) }}</template>
            <template #empty>
              <EmptyState title="No ledger rows" description="No contribution ledger rows match the current filters." />
            </template>
          </DataTable>
          <Pagination
            v-if="ledgerPagination.total > ledgerPagination.page_size"
            :page="ledgerPagination.page"
            :total="ledgerPagination.total"
            :page-size="ledgerPagination.page_size"
            @update:page="onLedgerPage"
            @update:pageSize="onLedgerPageSize"
          />
        </section>

        <section class="rounded-lg border border-gray-200 bg-white dark:border-dark-700 dark:bg-dark-900">
          <div class="border-b border-gray-200 p-4 dark:border-dark-700">
            <h2 class="text-base font-semibold text-gray-900 dark:text-white">Wallet Summary</h2>
            <p class="text-sm text-gray-500 dark:text-dark-300">Aggregated with the same ledger filters.</p>
          </div>
          <div class="max-h-[520px] divide-y divide-gray-200 overflow-auto dark:divide-dark-700">
            <div v-if="wallets.length === 0" class="p-4 text-sm text-gray-500 dark:text-dark-300">No wallet summary.</div>
            <button
              v-for="wallet in wallets"
              :key="wallet.wallet_address"
              class="block w-full p-4 text-left hover:bg-gray-50 dark:hover:bg-dark-800"
              @click="filterWallet(wallet.wallet_address)"
            >
              <div class="flex items-start justify-between gap-3">
                <span class="font-mono text-xs text-gray-900 dark:text-white">{{ shortText(wallet.wallet_address) }}</span>
                <span class="text-sm font-semibold text-primary-600 dark:text-primary-400">{{ formatUSD(wallet.contribution_usd) }}</span>
              </div>
              <div class="mt-2 flex justify-between text-xs text-gray-500 dark:text-dark-300">
                <span>{{ formatNumber(wallet.token_flow) }} tokens</span>
                <span>{{ wallet.ledger_count }} ledgers</span>
              </div>
            </button>
          </div>
        </section>
      </div>

      <section class="rounded-lg border border-gray-200 bg-white dark:border-dark-700 dark:bg-dark-900">
        <div class="flex flex-col gap-3 border-b border-gray-200 p-4 dark:border-dark-700 md:flex-row md:items-center md:justify-between">
          <div>
            <h2 class="text-base font-semibold text-gray-900 dark:text-white">Payout Batches</h2>
            <p class="text-sm text-gray-500 dark:text-dark-300">Manual settlement batches. Solana payout is intentionally not enabled.</p>
          </div>
          <select v-model="batchFilter.status" class="input w-36" @change="resetBatches">
            <option value="">All statuses</option>
            <option value="pending">Pending</option>
            <option value="finalized">Finalized</option>
            <option value="failed">Failed</option>
          </select>
        </div>
        <DataTable :columns="batchColumns" :data="batches" :loading="loading" :virtualized="false">
          <template #cell-batch_key="{ row }">
            <button class="font-mono text-xs text-primary-600 hover:underline dark:text-primary-400" @click="selectBatch(row)">
              {{ row.batch_key }}
            </button>
          </template>
          <template #cell-window="{ row }">{{ formatTime(row.window_start) }} - {{ formatTime(row.window_end) }}</template>
          <template #cell-total_token_flow="{ value }">{{ formatNumber(value) }}</template>
          <template #cell-total_contribution_usd="{ value }">{{ formatUSD(value) }}</template>
          <template #cell-status="{ value }">
            <span :class="statusBadgeClass(value)">{{ value }}</span>
          </template>
          <template #cell-actions="{ row }">
            <button class="btn btn-secondary btn-sm" @click="selectBatch(row)">Items</button>
          </template>
          <template #empty>
            <EmptyState title="No batches" description="Create a settlement batch from pending ledger rows." />
          </template>
        </DataTable>
        <Pagination
          v-if="batchPagination.total > batchPagination.page_size"
          :page="batchPagination.page"
          :total="batchPagination.total"
          :page-size="batchPagination.page_size"
          @update:page="onBatchPage"
          @update:pageSize="onBatchPageSize"
        />
      </section>

      <section v-if="selectedBatch" class="rounded-lg border border-gray-200 bg-white dark:border-dark-700 dark:bg-dark-900">
        <div class="flex items-center justify-between gap-3 border-b border-gray-200 p-4 dark:border-dark-700">
          <div>
            <h2 class="text-base font-semibold text-gray-900 dark:text-white">Batch Items</h2>
            <p class="text-sm text-gray-500 dark:text-dark-300">{{ selectedBatch.batch_key }} · {{ selectedBatch.item_count }} wallets</p>
          </div>
          <button class="btn btn-secondary btn-sm" @click="selectedBatch = null">Close</button>
        </div>
        <DataTable :columns="itemColumns" :data="batchItems" :loading="itemsLoading" :virtualized="false">
          <template #cell-wallet_address="{ value }">
            <span class="font-mono text-xs">{{ shortText(value) }}</span>
          </template>
          <template #cell-token_flow="{ value }">{{ formatNumber(value) }}</template>
          <template #cell-contribution_usd="{ value }">{{ formatUSD(value) }}</template>
          <template #cell-status="{ value }">
            <span :class="statusBadgeClass(value)">{{ value }}</span>
          </template>
          <template #cell-actions="{ row }">
            <div class="flex flex-wrap gap-2">
              <button class="btn btn-secondary btn-sm" @click="markItem(row, 'pending')">Pending</button>
              <button class="btn btn-primary btn-sm" @click="markItem(row, 'finalized')">Finalize</button>
              <button class="btn btn-danger btn-sm" @click="openFailDialog(row)">Fail</button>
            </div>
          </template>
        </DataTable>
      </section>
    </div>

    <BaseDialog :show="showBatchDialog" title="Create Settlement Batch" width="normal" @close="showBatchDialog = false">
      <div class="space-y-4">
        <label class="space-y-1">
          <span class="text-sm font-medium text-gray-700 dark:text-dark-200">Window Start</span>
          <input v-model="batchForm.window_start" class="input w-full" type="datetime-local" />
        </label>
        <label class="space-y-1">
          <span class="text-sm font-medium text-gray-700 dark:text-dark-200">Window End</span>
          <input v-model="batchForm.window_end" class="input w-full" type="datetime-local" />
        </label>
        <label class="space-y-1">
          <span class="text-sm font-medium text-gray-700 dark:text-dark-200">Settlement Method</span>
          <input v-model="batchForm.network" class="input w-full" />
        </label>
      </div>
      <template #footer>
        <button class="btn btn-secondary" @click="showBatchDialog = false">Cancel</button>
        <button class="btn btn-primary" :disabled="creatingBatch" @click="createSettlementBatch">
          {{ creatingBatch ? 'Creating...' : 'Create' }}
        </button>
      </template>
    </BaseDialog>

    <BaseDialog :show="!!failingItem" title="Mark Settlement Failed" width="normal" @close="failingItem = null">
      <label class="space-y-1">
        <span class="text-sm font-medium text-gray-700 dark:text-dark-200">Error Message</span>
        <textarea v-model="failMessage" class="input min-h-28 w-full" />
      </label>
      <template #footer>
        <button class="btn btn-secondary" @click="failingItem = null">Cancel</button>
        <button class="btn btn-danger" @click="submitFailItem">Mark Failed</button>
      </template>
    </BaseDialog>
  </AppLayout>
</template>

<script setup lang="ts">
import { computed, onMounted, reactive, ref } from 'vue'
import AppLayout from '@/components/layout/AppLayout.vue'
import BaseDialog from '@/components/common/BaseDialog.vue'
import DataTable from '@/components/common/DataTable.vue'
import EmptyState from '@/components/common/EmptyState.vue'
import Pagination from '@/components/common/Pagination.vue'
import { adminAPI } from '@/api/admin'
import type {
  QuotaNetContributionLedger,
  QuotaNetPayoutBatch,
  QuotaNetPayoutItem,
  QuotaNetSettlementConfig,
  QuotaNetSettlementSummary,
  QuotaNetWalletSummary
} from '@/api/admin/quotanet'

const loading = ref(false)
const itemsLoading = ref(false)
const savingConfig = ref(false)
const creatingBatch = ref(false)
const showBatchDialog = ref(false)
const selectedBatch = ref<QuotaNetPayoutBatch | null>(null)
const failingItem = ref<QuotaNetPayoutItem | null>(null)
const failMessage = ref('')

const configForm = reactive<QuotaNetSettlementConfig>({
  network: 'manual'
})
const summary = ref<QuotaNetSettlementSummary>({ ledger_count: 0, token_flow: 0, contribution_usd: 0, amount_cxs: 0 })
const wallets = ref<QuotaNetWalletSummary[]>([])
const ledgers = ref<QuotaNetContributionLedger[]>([])
const batches = ref<QuotaNetPayoutBatch[]>([])
const batchItems = ref<QuotaNetPayoutItem[]>([])

const ledgerFilter = reactive({ status: 'pending', wallet_address: '' })
const batchFilter = reactive({ status: '' })
const ledgerPagination = reactive({ page: 1, page_size: 20, total: 0 })
const batchPagination = reactive({ page: 1, page_size: 10, total: 0 })
const batchForm = reactive({
  window_start: '',
  window_end: '',
  network: 'manual'
})

const summaryCards = computed(() => [
  { label: 'Ledger Rows', value: formatNumber(summary.value.ledger_count), detail: 'Rows matching current filters' },
  { label: 'Contribution Tokens', value: formatNumber(summary.value.token_flow), detail: 'Successful task usage' },
  { label: 'Contribution USD', value: formatUSD(summary.value.contribution_usd), detail: 'Model standard cost' }
])

const ledgerColumns = [
  { key: 'task', label: 'Task' },
  { key: 'wallet_address', label: 'Wallet' },
  { key: 'model', label: 'Model' },
  { key: 'token_flow', label: 'Tokens' },
  { key: 'contribution_usd', label: 'USD' },
  { key: 'rate', label: 'Multiplier' },
  { key: 'status', label: 'Status' },
  { key: 'created_at', label: 'Created' }
]

const batchColumns = [
  { key: 'batch_key', label: 'Batch' },
  { key: 'window', label: 'Window' },
  { key: 'network', label: 'Network' },
  { key: 'total_token_flow', label: 'Tokens' },
  { key: 'total_contribution_usd', label: 'USD' },
  { key: 'item_count', label: 'Items' },
  { key: 'status', label: 'Status' },
  { key: 'actions', label: 'Actions' }
]

const itemColumns = [
  { key: 'wallet_address', label: 'Wallet' },
  { key: 'node_id', label: 'Node' },
  { key: 'token_flow', label: 'Tokens' },
  { key: 'contribution_usd', label: 'USD' },
  { key: 'status', label: 'Status' },
  { key: 'actions', label: 'Actions' }
]

onMounted(() => {
  setDefaultWindow()
  void reload()
})

async function reload() {
  loading.value = true
  try {
    const [config, ledgerResponse, summaryResponse, walletResponse, batchResponse] = await Promise.all([
      adminAPI.quotanet.getSettlementConfig(),
      adminAPI.quotanet.listLedgers(ledgerParams()),
      adminAPI.quotanet.getSettlementSummary(ledgerParams()),
      adminAPI.quotanet.listWalletSummaries(ledgerParams()),
      adminAPI.quotanet.listBatches(batchParams())
    ])
    Object.assign(configForm, config)
    batchForm.network = config.network
    ledgers.value = ledgerResponse.items
    ledgerPagination.total = ledgerResponse.total
    summary.value = summaryResponse
    wallets.value = walletResponse.items
    batches.value = batchResponse.items
    batchPagination.total = batchResponse.total
  } finally {
    loading.value = false
  }
}

async function saveConfig() {
  savingConfig.value = true
  try {
    const config = await adminAPI.quotanet.updateSettlementConfig({
      network: configForm.network || 'manual'
    })
    Object.assign(configForm, config)
    batchForm.network = config.network
  } finally {
    savingConfig.value = false
  }
}

function ledgerParams() {
  return {
    page: ledgerPagination.page,
    page_size: ledgerPagination.page_size,
    status: ledgerFilter.status || undefined,
    wallet_address: ledgerFilter.wallet_address || undefined
  }
}

function batchParams() {
  return {
    page: batchPagination.page,
    page_size: batchPagination.page_size,
    status: batchFilter.status || undefined
  }
}

function resetLedgers() {
  ledgerPagination.page = 1
  void reload()
}

function resetBatches() {
  batchPagination.page = 1
  void reload()
}

function onLedgerPage(page: number) {
  ledgerPagination.page = page
  void reload()
}

function onLedgerPageSize(pageSize: number) {
  ledgerPagination.page_size = pageSize
  ledgerPagination.page = 1
  void reload()
}

function onBatchPage(page: number) {
  batchPagination.page = page
  void reload()
}

function onBatchPageSize(pageSize: number) {
  batchPagination.page_size = pageSize
  batchPagination.page = 1
  void reload()
}

function filterWallet(wallet: string) {
  ledgerFilter.wallet_address = wallet
  resetLedgers()
}

function openBatchDialog() {
  setDefaultWindow()
  batchForm.network = configForm.network
  showBatchDialog.value = true
}

async function createSettlementBatch() {
  creatingBatch.value = true
  try {
    const result = await adminAPI.quotanet.createBatch({
      window_start: toRFC3339(batchForm.window_start),
      window_end: toRFC3339(batchForm.window_end),
      network: batchForm.network || 'manual'
    })
    showBatchDialog.value = false
    selectedBatch.value = result.batch
    batchItems.value = result.items
    await reload()
  } finally {
    creatingBatch.value = false
  }
}

async function selectBatch(batch: QuotaNetPayoutBatch) {
  selectedBatch.value = batch
  itemsLoading.value = true
  try {
    const response = await adminAPI.quotanet.listBatchItems(batch.id, { page: 1, page_size: 100 })
    batchItems.value = response.items
  } finally {
    itemsLoading.value = false
  }
}

async function markItem(item: QuotaNetPayoutItem, status: string) {
  await adminAPI.quotanet.updatePayoutItemStatus(item.id, { status })
  if (selectedBatch.value) {
    await selectBatch(selectedBatch.value)
  }
  await reload()
}

function openFailDialog(item: QuotaNetPayoutItem) {
  failingItem.value = item
  failMessage.value = item.error_message || ''
}

async function submitFailItem() {
  if (!failingItem.value) {
    return
  }
  await adminAPI.quotanet.updatePayoutItemStatus(failingItem.value.id, {
    status: 'failed',
    error_message: failMessage.value || 'manual settlement failed'
  })
  failingItem.value = null
  if (selectedBatch.value) {
    await selectBatch(selectedBatch.value)
  }
  await reload()
}

function setDefaultWindow() {
  const end = new Date()
  const start = new Date(end.getTime() - 24 * 60 * 60 * 1000)
  batchForm.window_start = toLocalInput(start)
  batchForm.window_end = toLocalInput(end)
}

function toLocalInput(date: Date) {
  const pad = (n: number) => String(n).padStart(2, '0')
  return `${date.getFullYear()}-${pad(date.getMonth() + 1)}-${pad(date.getDate())}T${pad(date.getHours())}:${pad(date.getMinutes())}`
}

function toRFC3339(value: string) {
  const date = value ? new Date(value) : new Date()
  return date.toISOString()
}

function formatNumber(value: number | string | null | undefined) {
  return Number(value || 0).toLocaleString()
}

function formatUSD(value: number | string | null | undefined) {
  return `$${Number(value || 0).toFixed(6)}`
}

function formatMultiplier(value: number | string | null | undefined) {
  return Number(value || 0).toFixed(6)
}

function formatTime(value?: string | null) {
  if (!value) {
    return '-'
  }
  return new Date(value).toLocaleString()
}

function shortText(value?: string | null) {
  const text = value || '-'
  if (text.length <= 18) {
    return text
  }
  return `${text.slice(0, 8)}...${text.slice(-6)}`
}

function statusBadgeClass(status: string) {
  switch (status) {
    case 'finalized':
      return 'inline-flex rounded bg-green-100 px-2 py-0.5 text-xs font-medium text-green-700 dark:bg-green-900/30 dark:text-green-300'
    case 'failed':
      return 'inline-flex rounded bg-red-100 px-2 py-0.5 text-xs font-medium text-red-700 dark:bg-red-900/30 dark:text-red-300'
    default:
      return 'inline-flex rounded bg-yellow-100 px-2 py-0.5 text-xs font-medium text-yellow-700 dark:bg-yellow-900/30 dark:text-yellow-300'
  }
}
</script>
