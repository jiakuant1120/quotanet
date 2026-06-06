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

export interface QuotaNetTaskListParams {
  page?: number
  page_size?: number
  status?: string
  platform?: string
  node_id?: number
  search?: string
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

export async function getTaskEvents(taskID: string, options?: FetchOptions): Promise<{ items: QuotaNetTaskEvent[] }> {
  const { data } = await apiClient.get<{ items: QuotaNetTaskEvent[] }>(`/admin/quotanet/tasks/${encodeURIComponent(taskID)}/events`, {
    signal: options?.signal
  })
  return data
}

const quotanetAPI = {
  getNodeOverview,
  listNodes,
  listSessions,
  listTasks,
  listNodeTasks,
  dispatchTask,
  dispatchTaskSync,
  getTaskEvents
}

export default quotanetAPI
