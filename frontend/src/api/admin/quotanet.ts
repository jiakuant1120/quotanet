import { apiClient } from '../client'
import type { BasePaginationResponse, FetchOptions } from '@/types'

export interface QuotaNetCapability {
  provider: string
  models: string[]
  max_concurrency?: number
}

export interface QuotaNetNode {
  id: number
  node_key: string
  name: string
  owner_user_id?: number | null
  wallet_address: string
  status: string
  created_at?: string
  updated_at?: string
  last_seen_at?: string | null
}

export interface CreateQuotaNetNodeRequest {
  name: string
  wallet_address: string
  owner_user_id?: number | null
  status?: string
}

export interface QuotaNetNodeTokenResponse {
  node: QuotaNetNode
  token: string
}

export interface QuotaNetSession {
  session_id: string
  node_id: number
  node_key: string
  instance_id: string
  wallet_address: string
  client_version?: string
  protocol_version?: string
  capabilities: QuotaNetCapability[]
  status: string
  current_concurrency: number
  max_concurrency: number
  queue_size: number
  max_queue_size: number
  connected_at?: string
  last_heartbeat_at?: string
  disconnected_at?: string | null
  close_reason?: string
}

export interface QuotaNetProviderOverview {
  provider: string
  models: string[]
}

export interface QuotaNetNodeOverview {
  sessions: {
    total: number
    connected: number
    by_status: Record<string, number>
    ready: number
    busy: number
    offline: number
    stale: number
    stale_after: string
  }
  capacity: {
    current_concurrency: number
    max_concurrency: number
    available: number
    queue_size: number
    max_queue_size: number
  }
  providers: QuotaNetProviderOverview[]
  task_statuses: Record<string, number>
  recent_sessions: QuotaNetSession[]
}

export interface QuotaNetTask {
  id: number
  task_id: string
  request_id: string
  user_id?: number | null
  api_key_id?: number | null
  group_id?: number | null
  account_id?: number | null
  node_id?: number | null
  session_id?: string | null
  platform: string
  endpoint: string
  model: string
  stream: boolean
  status: string
  error_code?: string | null
  error_message?: string | null
  prompt_tokens: number
  completion_tokens: number
  total_tokens: number
  first_token_ms?: number | null
  duration_ms?: number | null
  created_at?: string
  updated_at?: string
  dispatched_at?: string | null
  completed_at?: string | null
}

export interface QuotaNetTaskEvent {
  id: number
  task_id: string
  event_type: string
  sequence: number
  payload: Record<string, unknown>
  created_at?: string
}

export interface QuotaNetTaskDispatchRequest {
  request_id?: string
  node_id?: number
  platform: string
  endpoint: string
  model: string
  stream?: boolean
  timeout_seconds?: number
  payload?: Record<string, unknown>
}

export interface QuotaNetTaskDispatchSyncResponse {
  task: QuotaNetTask
  response: {
    task_id: string
    status: string
    error_code?: string
    error_msg?: string
    usage: Record<string, number>
    payload?: Record<string, unknown>
    duration_ms?: number
    first_token_ms?: number
  }
}

export interface QuotaNetTaskTimeoutSweepRequest {
  older_than_seconds?: number
  limit?: number
}

export interface QuotaNetTaskTimeoutSweepResponse {
  count: number
  task_ids: string[]
}

export interface QuotaNetTaskListParams {
  page?: number
  page_size?: number
  status?: string
  platform?: string
  node_id?: number
  search?: string
}

export interface QuotaNetSettlementConfig {
  network: string
}

export interface QuotaNetContributionLedger {
  id: number
  task_id: string
  usage_log_id?: number | null
  node_id: number
  wallet_address: string
  account_id?: number | null
  platform: string
  model: string
  token_flow: number
  standard_cost_usd: number
  actual_cost_usd: number
  contribution_usd: number
  amount_cxs: number
  rate: number
  status: string
  payout_batch_id?: number | null
  settled_at?: string | null
  created_at?: string
  updated_at?: string
}

export interface QuotaNetSettlementSummary {
  ledger_count: number
  token_flow: number
  contribution_usd: number
  amount_cxs: number
}

export interface QuotaNetWalletSummary extends QuotaNetSettlementSummary {
  wallet_address: string
}

export interface QuotaNetPayoutBatch {
  id: number
  batch_key: string
  window_start?: string
  window_end?: string
  status: string
  network: string
  total_token_flow: number
  total_contribution_usd: number
  total_amount_cxs: number
  item_count: number
  created_by?: number | null
  approved_by?: number | null
  created_at?: string
  updated_at?: string
}

export interface QuotaNetPayoutItem {
  id: number
  item_key: string
  batch_id: number
  network?: string
  node_id?: number | null
  wallet_address: string
  token_flow: number
  contribution_usd: number
  amount_cxs: number
  status: string
  tx_hash?: string | null
  tx_url?: string
  error_message?: string | null
  finalized_at?: string | null
  created_at?: string
  updated_at?: string
}

export interface QuotaNetLedgerListParams {
  page?: number
  page_size?: number
  status?: string
  wallet_address?: string
  node_id?: number
  account_id?: number
  payout_batch_id?: number
}

export interface QuotaNetBatchListParams {
  page?: number
  page_size?: number
  status?: string
}

export interface QuotaNetBatchItemListParams {
  page?: number
  page_size?: number
  status?: string
  wallet_address?: string
  tx_hash?: string
}

export interface QuotaNetCreateBatchRequest {
  batch_key?: string
  window_start: string
  window_end: string
  network?: string
  rate?: number
}

export interface QuotaNetCreateBatchResponse {
  batch: QuotaNetPayoutBatch
  items: QuotaNetPayoutItem[]
  ledger_count: number
}

export interface QuotaNetUpdateItemStatusRequest {
  status: string
  tx_hash?: string
  error_message?: string
}

export async function getNodeOverview(options?: FetchOptions): Promise<QuotaNetNodeOverview> {
  const { data } = await apiClient.get<QuotaNetNodeOverview>('/admin/quotanet/overview', {
    signal: options?.signal
  })
  return data
}

export async function listNodes(params?: { page?: number; page_size?: number; status?: string; search?: string }, options?: FetchOptions): Promise<BasePaginationResponse<QuotaNetNode>> {
  const { data } = await apiClient.get<BasePaginationResponse<QuotaNetNode>>('/admin/quotanet/nodes', {
    params,
    signal: options?.signal
  })
  return data
}

export async function createNode(req: CreateQuotaNetNodeRequest): Promise<QuotaNetNodeTokenResponse> {
  const { data } = await apiClient.post<QuotaNetNodeTokenResponse>('/admin/quotanet/nodes', req)
  return data
}

export async function resetNodeToken(nodeID: number): Promise<QuotaNetNodeTokenResponse> {
  const { data } = await apiClient.post<QuotaNetNodeTokenResponse>(`/admin/quotanet/nodes/${nodeID}/reset-token`)
  return data
}

export async function listSessions(options?: FetchOptions): Promise<{ items: QuotaNetSession[] }> {
  const { data } = await apiClient.get<{ items: QuotaNetSession[] }>('/admin/quotanet/nodes/sessions', {
    signal: options?.signal
  })
  return data
}

export async function listTasks(params?: QuotaNetTaskListParams, options?: FetchOptions): Promise<BasePaginationResponse<QuotaNetTask>> {
  const { data } = await apiClient.get<BasePaginationResponse<QuotaNetTask>>('/admin/quotanet/tasks', {
    params,
    signal: options?.signal
  })
  return data
}

export async function listNodeTasks(nodeID: number, params?: Omit<QuotaNetTaskListParams, 'node_id'>, options?: FetchOptions): Promise<BasePaginationResponse<QuotaNetTask>> {
  const { data } = await apiClient.get<BasePaginationResponse<QuotaNetTask>>(`/admin/quotanet/nodes/${nodeID}/tasks`, {
    params,
    signal: options?.signal
  })
  return data
}

export async function dispatchTask(req: QuotaNetTaskDispatchRequest): Promise<QuotaNetTask> {
  const { data } = await apiClient.post<QuotaNetTask>('/admin/quotanet/tasks/dispatch', req)
  return data
}

export async function dispatchTaskSync(req: QuotaNetTaskDispatchRequest): Promise<QuotaNetTaskDispatchSyncResponse> {
  const { data } = await apiClient.post<QuotaNetTaskDispatchSyncResponse>('/admin/quotanet/tasks/dispatch-sync', req)
  return data
}

export async function timeoutSweep(req: QuotaNetTaskTimeoutSweepRequest): Promise<QuotaNetTaskTimeoutSweepResponse> {
  const { data } = await apiClient.post<QuotaNetTaskTimeoutSweepResponse>('/admin/quotanet/tasks/timeout-sweep', req)
  return data
}

export async function getTaskEvents(taskID: string, options?: FetchOptions): Promise<{ items: QuotaNetTaskEvent[] }> {
  const { data } = await apiClient.get<{ items: QuotaNetTaskEvent[] }>(`/admin/quotanet/tasks/${encodeURIComponent(taskID)}/events`, {
    signal: options?.signal
  })
  return data
}

export async function getSettlementConfig(options?: FetchOptions): Promise<QuotaNetSettlementConfig> {
  const { data } = await apiClient.get<QuotaNetSettlementConfig>('/admin/quotanet/settlements/config', {
    signal: options?.signal
  })
  return data
}

export async function updateSettlementConfig(req: QuotaNetSettlementConfig): Promise<QuotaNetSettlementConfig> {
  const { data } = await apiClient.put<QuotaNetSettlementConfig>('/admin/quotanet/settlements/config', req)
  return data
}

export async function listLedgers(params?: QuotaNetLedgerListParams, options?: FetchOptions): Promise<BasePaginationResponse<QuotaNetContributionLedger>> {
  const { data } = await apiClient.get<BasePaginationResponse<QuotaNetContributionLedger>>('/admin/quotanet/settlements/ledgers', {
    params,
    signal: options?.signal
  })
  return data
}

export async function getSettlementSummary(params?: QuotaNetLedgerListParams, options?: FetchOptions): Promise<QuotaNetSettlementSummary> {
  const { data } = await apiClient.get<QuotaNetSettlementSummary>('/admin/quotanet/settlements/summary', {
    params,
    signal: options?.signal
  })
  return data
}

export async function listWalletSummaries(params?: QuotaNetLedgerListParams, options?: FetchOptions): Promise<{ items: QuotaNetWalletSummary[] }> {
  const { data } = await apiClient.get<{ items: QuotaNetWalletSummary[] }>('/admin/quotanet/settlements/wallets', {
    params,
    signal: options?.signal
  })
  return data
}

export async function listBatches(params?: QuotaNetBatchListParams, options?: FetchOptions): Promise<BasePaginationResponse<QuotaNetPayoutBatch>> {
  const { data } = await apiClient.get<BasePaginationResponse<QuotaNetPayoutBatch>>('/admin/quotanet/settlements/batches', {
    params,
    signal: options?.signal
  })
  return data
}

export async function createBatch(req: QuotaNetCreateBatchRequest): Promise<QuotaNetCreateBatchResponse> {
  const { data } = await apiClient.post<QuotaNetCreateBatchResponse>('/admin/quotanet/settlements/batches', req)
  return data
}

export async function listBatchItems(batchID: number, params?: QuotaNetBatchItemListParams, options?: FetchOptions): Promise<BasePaginationResponse<QuotaNetPayoutItem>> {
  const { data } = await apiClient.get<BasePaginationResponse<QuotaNetPayoutItem>>(`/admin/quotanet/settlements/batches/${batchID}/items`, {
    params,
    signal: options?.signal
  })
  return data
}

export async function updatePayoutItemStatus(itemID: number, req: QuotaNetUpdateItemStatusRequest): Promise<QuotaNetPayoutItem> {
  const { data } = await apiClient.put<QuotaNetPayoutItem>(`/admin/quotanet/settlements/items/${itemID}/status`, req)
  return data
}

const quotanetAPI = {
  getNodeOverview,
  listNodes,
  createNode,
  resetNodeToken,
  listSessions,
  listTasks,
  listNodeTasks,
  dispatchTask,
  dispatchTaskSync,
  timeoutSweep,
  getTaskEvents,
  getSettlementConfig,
  updateSettlementConfig,
  listLedgers,
  getSettlementSummary,
  listWalletSummaries,
  listBatches,
  createBatch,
  listBatchItems,
  updatePayoutItemStatus
}

export default quotanetAPI
