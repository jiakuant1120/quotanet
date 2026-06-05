-- Add QuotaNet node, task, contribution, and payout persistence tables.

CREATE TABLE IF NOT EXISTS quotanet_nodes (
    id BIGSERIAL PRIMARY KEY,
    node_key VARCHAR(64) NOT NULL UNIQUE,
    name VARCHAR(100) NOT NULL DEFAULT '',
    owner_user_id BIGINT,
    wallet_address VARCHAR(128) NOT NULL,
    token_hash VARCHAR(128) NOT NULL,
    status VARCHAR(20) NOT NULL DEFAULT 'pending',
    protocol_version VARCHAR(40),
    client_version VARCHAR(40),
    last_seen_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    deleted_at TIMESTAMPTZ
);

CREATE INDEX IF NOT EXISTS quotanet_nodes_wallet_address
    ON quotanet_nodes (wallet_address);
CREATE INDEX IF NOT EXISTS quotanet_nodes_status
    ON quotanet_nodes (status);
CREATE INDEX IF NOT EXISTS quotanet_nodes_last_seen_at
    ON quotanet_nodes (last_seen_at);
CREATE INDEX IF NOT EXISTS quotanet_nodes_owner_user_id
    ON quotanet_nodes (owner_user_id);

CREATE TABLE IF NOT EXISTS quotanet_node_sessions (
    id BIGSERIAL PRIMARY KEY,
    session_id VARCHAR(64) NOT NULL UNIQUE,
    node_id BIGINT NOT NULL,
    instance_id VARCHAR(64) NOT NULL,
    status VARCHAR(20) NOT NULL DEFAULT 'connected',
    remote_addr VARCHAR(128),
    max_concurrency INTEGER NOT NULL DEFAULT 1,
    current_concurrency INTEGER NOT NULL DEFAULT 0,
    queue_size INTEGER NOT NULL DEFAULT 0,
    max_queue_size INTEGER NOT NULL DEFAULT 0,
    capabilities JSONB NOT NULL DEFAULT '{}'::jsonb,
    connected_at TIMESTAMPTZ NOT NULL,
    disconnected_at TIMESTAMPTZ,
    last_heartbeat_at TIMESTAMPTZ,
    close_reason TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS quotanet_node_sessions_node_id_connected_at
    ON quotanet_node_sessions (node_id, connected_at);
CREATE INDEX IF NOT EXISTS quotanet_node_sessions_status
    ON quotanet_node_sessions (status);
CREATE INDEX IF NOT EXISTS quotanet_node_sessions_instance_id
    ON quotanet_node_sessions (instance_id);
CREATE INDEX IF NOT EXISTS quotanet_node_sessions_last_heartbeat_at
    ON quotanet_node_sessions (last_heartbeat_at);

CREATE TABLE IF NOT EXISTS quotanet_tasks (
    id BIGSERIAL PRIMARY KEY,
    task_id VARCHAR(64) NOT NULL UNIQUE,
    request_id VARCHAR(64) NOT NULL,
    user_id BIGINT,
    api_key_id BIGINT,
    group_id BIGINT,
    account_id BIGINT,
    node_id BIGINT,
    session_id VARCHAR(64),
    platform VARCHAR(50) NOT NULL,
    endpoint VARCHAR(100) NOT NULL,
    model VARCHAR(100) NOT NULL,
    stream BOOLEAN NOT NULL DEFAULT FALSE,
    status VARCHAR(20) NOT NULL DEFAULT 'queued',
    error_code VARCHAR(64),
    error_message TEXT,
    prompt_tokens INTEGER NOT NULL DEFAULT 0,
    completion_tokens INTEGER NOT NULL DEFAULT 0,
    total_tokens INTEGER NOT NULL DEFAULT 0,
    first_token_ms INTEGER,
    duration_ms INTEGER,
    dispatched_at TIMESTAMPTZ,
    completed_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS quotanet_tasks_request_id
    ON quotanet_tasks (request_id);
CREATE INDEX IF NOT EXISTS quotanet_tasks_node_id_created_at
    ON quotanet_tasks (node_id, created_at);
CREATE INDEX IF NOT EXISTS quotanet_tasks_account_id_created_at
    ON quotanet_tasks (account_id, created_at);
CREATE INDEX IF NOT EXISTS quotanet_tasks_user_id_created_at
    ON quotanet_tasks (user_id, created_at);
CREATE INDEX IF NOT EXISTS quotanet_tasks_status_created_at
    ON quotanet_tasks (status, created_at);
CREATE INDEX IF NOT EXISTS quotanet_tasks_session_id
    ON quotanet_tasks (session_id);

CREATE TABLE IF NOT EXISTS quotanet_task_events (
    id BIGSERIAL PRIMARY KEY,
    task_id VARCHAR(64) NOT NULL,
    event_type VARCHAR(50) NOT NULL,
    sequence BIGINT NOT NULL,
    payload JSONB NOT NULL DEFAULT '{}'::jsonb,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT quotanet_task_events_task_id_sequence_key UNIQUE (task_id, sequence)
);

CREATE INDEX IF NOT EXISTS quotanet_task_events_task_id_created_at
    ON quotanet_task_events (task_id, created_at);
CREATE INDEX IF NOT EXISTS quotanet_task_events_event_type
    ON quotanet_task_events (event_type);

CREATE TABLE IF NOT EXISTS quotanet_contribution_ledger (
    id BIGSERIAL PRIMARY KEY,
    task_id VARCHAR(64) NOT NULL UNIQUE,
    usage_log_id BIGINT,
    node_id BIGINT NOT NULL,
    wallet_address VARCHAR(128) NOT NULL,
    account_id BIGINT,
    platform VARCHAR(50) NOT NULL,
    model VARCHAR(100) NOT NULL,
    token_flow BIGINT NOT NULL DEFAULT 0,
    amount_cxs DECIMAL(30,12) NOT NULL DEFAULT 0,
    rate DECIMAL(20,10) NOT NULL DEFAULT 0,
    status VARCHAR(20) NOT NULL DEFAULT 'pending',
    payout_batch_id BIGINT,
    settled_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS quotanet_contribution_ledger_node_id_created_at
    ON quotanet_contribution_ledger (node_id, created_at);
CREATE INDEX IF NOT EXISTS quotanet_contribution_ledger_wallet_address_status
    ON quotanet_contribution_ledger (wallet_address, status);
CREATE INDEX IF NOT EXISTS quotanet_contribution_ledger_payout_batch_id
    ON quotanet_contribution_ledger (payout_batch_id);
CREATE INDEX IF NOT EXISTS quotanet_contribution_ledger_status_created_at
    ON quotanet_contribution_ledger (status, created_at);

CREATE TABLE IF NOT EXISTS quotanet_payout_batches (
    id BIGSERIAL PRIMARY KEY,
    batch_key VARCHAR(64) NOT NULL UNIQUE,
    window_start TIMESTAMPTZ NOT NULL,
    window_end TIMESTAMPTZ NOT NULL,
    status VARCHAR(20) NOT NULL DEFAULT 'draft',
    network VARCHAR(40) NOT NULL DEFAULT 'solana-devnet',
    total_token_flow BIGINT NOT NULL DEFAULT 0,
    total_amount_cxs DECIMAL(30,12) NOT NULL DEFAULT 0,
    item_count INTEGER NOT NULL DEFAULT 0,
    created_by BIGINT,
    approved_by BIGINT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS quotanet_payout_batches_status
    ON quotanet_payout_batches (status);
CREATE INDEX IF NOT EXISTS quotanet_payout_batches_window_start_window_end
    ON quotanet_payout_batches (window_start, window_end);

CREATE TABLE IF NOT EXISTS quotanet_payout_items (
    id BIGSERIAL PRIMARY KEY,
    item_key VARCHAR(64) NOT NULL UNIQUE,
    batch_id BIGINT NOT NULL,
    node_id BIGINT,
    wallet_address VARCHAR(128) NOT NULL,
    token_flow BIGINT NOT NULL DEFAULT 0,
    amount_cxs DECIMAL(30,12) NOT NULL DEFAULT 0,
    status VARCHAR(20) NOT NULL DEFAULT 'pending',
    tx_hash VARCHAR(128),
    error_message TEXT,
    finalized_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS quotanet_payout_items_batch_id
    ON quotanet_payout_items (batch_id);
CREATE INDEX IF NOT EXISTS quotanet_payout_items_wallet_address_status
    ON quotanet_payout_items (wallet_address, status);
CREATE INDEX IF NOT EXISTS quotanet_payout_items_tx_hash
    ON quotanet_payout_items (tx_hash);
