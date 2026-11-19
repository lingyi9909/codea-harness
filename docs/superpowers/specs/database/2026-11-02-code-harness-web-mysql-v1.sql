-- Code Harness Web MVP / MySQL 8.4 / design DDL v1 / 2026-11-02
-- Review draft only. No product code exists yet; not applied to a database.
-- Operator creates an EMPTY database with utf8mb4_0900_ai_ci; select it before migration.
-- Do not run in a business database. No DROP, no credentials, no CREATE DATABASE.
-- UTC DATETIME(3) supplied by the application; mysql2 configured with UTC/string bigints.
-- All identifiers are UUID v4 lowercase; SHA/OID values are lowercase hex.
-- FK deletion uses RESTRICT; no implicit history deletion. CHECK requires this exact baseline.
SET NAMES utf8mb4;
SET time_zone = '+00:00';

CREATE TABLE wh_owners (
  id CHAR(36) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  subject VARCHAR(128) NOT NULL,
  display_name VARCHAR(100) NOT NULL,
  created_at DATETIME(3) NOT NULL,
  updated_at DATETIME(3) NOT NULL,
  PRIMARY KEY (id),
  UNIQUE KEY uq_owner_subject (subject)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

CREATE TABLE wh_repositories (
  id CHAR(36) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  owner_id CHAR(36) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  name VARCHAR(100) NOT NULL,
  canonical_path VARCHAR(4096) NOT NULL,
  path_sha256 CHAR(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  status VARCHAR(16) NOT NULL DEFAULT 'READY',
  created_at DATETIME(3) NOT NULL,
  updated_at DATETIME(3) NOT NULL,
  PRIMARY KEY (id),
  UNIQUE KEY uq_repo_path (owner_id,path_sha256),
  KEY ix_repo_owner (owner_id,created_at,id),
  FOREIGN KEY (owner_id) REFERENCES wh_owners(id),
  CHECK (status IN ('READY','UNAVAILABLE','DISABLED'))
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

CREATE TABLE wh_review_previews (
  id CHAR(36) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  owner_id CHAR(36) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  repository_id CHAR(36) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  base_ref_id VARCHAR(2048) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  base_ref VARCHAR(1024) COLLATE utf8mb4_bin NOT NULL,
  source_branch VARCHAR(1024) COLLATE utf8mb4_bin NULL,
  head_oid VARCHAR(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  base_oid VARCHAR(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  merge_base_oid VARCHAR(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  object_format VARCHAR(8) NOT NULL,
  summary JSON NOT NULL,
  expires_at DATETIME(3) NOT NULL,
  created_at DATETIME(3) NOT NULL,
  PRIMARY KEY (id),
  KEY ix_preview_expiry (expires_at),
  FOREIGN KEY (owner_id) REFERENCES wh_owners(id),
  FOREIGN KEY (repository_id) REFERENCES wh_repositories(id),
  CHECK (object_format IN ('sha1','sha256'))
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

CREATE TABLE wh_tasks (
  id CHAR(36) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  owner_id CHAR(36) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  repository_id CHAR(36) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  preview_id CHAR(36) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  scenario_id VARCHAR(32) NOT NULL DEFAULT 'code-review',
  title VARCHAR(160) NOT NULL,
  controller VARCHAR(128) CHARACTER SET ascii COLLATE ascii_bin NULL,
  notes TEXT NULL,
  next_event_seq BIGINT UNSIGNED NOT NULL DEFAULT 1,
  next_run_number INT UNSIGNED NOT NULL DEFAULT 1,
  created_at DATETIME(3) NOT NULL,
  updated_at DATETIME(3) NOT NULL,
  PRIMARY KEY (id),
  UNIQUE KEY uq_task_preview (preview_id),
  KEY ix_task_history (owner_id,created_at,id),
  FOREIGN KEY (owner_id) REFERENCES wh_owners(id),
  FOREIGN KEY (repository_id) REFERENCES wh_repositories(id),
  FOREIGN KEY (preview_id) REFERENCES wh_review_previews(id),
  CHECK (scenario_id = 'code-review')
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

CREATE TABLE wh_scope_snapshots (
  id CHAR(36) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  task_id CHAR(36) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  head_oid VARCHAR(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  base_oid VARCHAR(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  merge_base_oid VARCHAR(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  base_ref VARCHAR(1024) COLLATE utf8mb4_bin NOT NULL,
  source_branch VARCHAR(1024) COLLATE utf8mb4_bin NULL,
  object_format VARCHAR(8) NOT NULL,
  manifest JSON NOT NULL,
  manifest_sha256 CHAR(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  storage_key VARCHAR(1024) COLLATE utf8mb4_bin NOT NULL,
  status VARCHAR(16) NOT NULL DEFAULT 'PENDING',
  created_at DATETIME(3) NOT NULL,
  ready_at DATETIME(3) NULL,
  PRIMARY KEY (id),
  UNIQUE KEY uq_snapshot_task (task_id),
  FOREIGN KEY (task_id) REFERENCES wh_tasks(id),
  CHECK (status IN ('PENDING','READY','FAILED')),
  CHECK (object_format IN ('sha1','sha256'))
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

CREATE TABLE wh_runs (
  id CHAR(36) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  task_id CHAR(36) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  run_number INT UNSIGNED NOT NULL,
  kind VARCHAR(16) NOT NULL,
  status VARCHAR(24) NOT NULL DEFAULT 'QUEUED',
  retry_of_run_id CHAR(36) CHARACTER SET ascii COLLATE ascii_bin NULL,
  active_task_id CHAR(36) CHARACTER SET ascii COLLATE ascii_bin GENERATED ALWAYS AS (CASE WHEN status IN ('QUEUED','PREPARING','RUNNING','WAITING_INPUT','WAITING_PERMISSION','COLLECTING','STOP_REQUESTED','RECOVERING') THEN task_id ELSE NULL END) STORED,
  dispatch_state VARCHAR(16) NOT NULL DEFAULT 'NOT_SENT',
  native_input_id VARCHAR(128) CHARACTER SET ascii COLLATE ascii_bin NULL,
  instruction_digest CHAR(64) CHARACTER SET ascii COLLATE ascii_bin NULL,
  asset_versions JSON NOT NULL,
  error_code VARCHAR(64) NULL,
  error_detail JSON NULL,
  lease_fence BIGINT UNSIGNED NOT NULL DEFAULT 0,
  created_at DATETIME(3) NOT NULL,
  updated_at DATETIME(3) NOT NULL,
  started_at DATETIME(3) NULL,
  finished_at DATETIME(3) NULL,
  PRIMARY KEY (id),
  UNIQUE KEY uq_run_number (task_id,run_number),
  UNIQUE KEY uq_active_task (active_task_id),
  UNIQUE KEY uq_run_task (id,task_id),
  KEY ix_run_queue (status,created_at,id),
  FOREIGN KEY (task_id) REFERENCES wh_tasks(id),
  FOREIGN KEY (retry_of_run_id,task_id) REFERENCES wh_runs(id,task_id),
  CHECK (kind IN ('REVIEW','DISCUSS','RECHECK')),
  CHECK (status IN ('QUEUED','PREPARING','RUNNING','WAITING_INPUT','WAITING_PERMISSION','COLLECTING','STOP_REQUESTED','RECOVERING','SUCCEEDED','FAILED','STOPPED')),
  CHECK (dispatch_state IN ('NOT_SENT','SENDING','ACCEPTED','UNKNOWN','REJECTED'))
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

CREATE TABLE wh_session_bindings (
  task_id CHAR(36) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  service_identity VARCHAR(128) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  native_session_id VARCHAR(128) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  location_key VARCHAR(1024) COLLATE utf8mb4_bin NOT NULL,
  native_version VARCHAR(100) NOT NULL,
  capabilities JSON NOT NULL,
  native_log_seq VARCHAR(64) CHARACTER SET ascii COLLATE ascii_bin NULL,
  continuity VARCHAR(16) NOT NULL DEFAULT 'COMPLETE',
  created_at DATETIME(3) NOT NULL,
  updated_at DATETIME(3) NOT NULL,
  PRIMARY KEY (task_id),
  UNIQUE KEY uq_native_session (service_identity,native_session_id),
  FOREIGN KEY (task_id) REFERENCES wh_tasks(id),
  CHECK (continuity IN ('COMPLETE','GAP','UNKNOWN'))
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

CREATE TABLE wh_executor_slots (
  slot_key VARCHAR(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  run_id CHAR(36) CHARACTER SET ascii COLLATE ascii_bin NULL,
  lease_owner VARCHAR(128) CHARACTER SET ascii COLLATE ascii_bin NULL,
  fence BIGINT UNSIGNED NOT NULL DEFAULT 0,
  lease_until DATETIME(3) NULL,
  quarantined BOOLEAN NOT NULL DEFAULT FALSE,
  updated_at DATETIME(3) NOT NULL,
  PRIMARY KEY (slot_key),
  UNIQUE KEY uq_slot_run (run_id),
  FOREIGN KEY (run_id) REFERENCES wh_runs(id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

CREATE TABLE wh_events (
  task_id CHAR(36) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  seq BIGINT UNSIGNED NOT NULL,
  run_id CHAR(36) CHARACTER SET ascii COLLATE ascii_bin NULL,
  event_type VARCHAR(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  dedupe_key CHAR(64) CHARACTER SET ascii COLLATE ascii_bin NULL,
  payload JSON NOT NULL,
  created_at DATETIME(3) NOT NULL,
  PRIMARY KEY (task_id,seq),
  UNIQUE KEY uq_event_dedupe (task_id,dedupe_key),
  KEY ix_event_run (run_id,task_id,seq),
  FOREIGN KEY (task_id) REFERENCES wh_tasks(id),
  FOREIGN KEY (run_id,task_id) REFERENCES wh_runs(id,task_id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

CREATE TABLE wh_messages (
  id CHAR(36) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  task_id CHAR(36) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  run_id CHAR(36) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  native_message_id VARCHAR(128) CHARACTER SET ascii COLLATE ascii_bin NULL,
  role VARCHAR(16) NOT NULL,
  content MEDIUMTEXT NOT NULL,
  status VARCHAR(16) NOT NULL,
  report_artifact_id CHAR(36) CHARACTER SET ascii COLLATE ascii_bin NULL,
  created_at DATETIME(3) NOT NULL,
  updated_at DATETIME(3) NOT NULL,
  PRIMARY KEY (id),
  UNIQUE KEY uq_message_native (task_id,native_message_id),
  KEY ix_message_page (task_id,created_at,id),
  FOREIGN KEY (run_id,task_id) REFERENCES wh_runs(id,task_id),
  CHECK (role IN ('USER','ASSISTANT')),
  CHECK (status IN ('DRAFT','STREAMING','COMPLETE','INTERRUPTED'))
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

CREATE TABLE wh_artifacts (
  id CHAR(36) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  task_id CHAR(36) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  run_id CHAR(36) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  version INT UNSIGNED NOT NULL,
  kind VARCHAR(32) NOT NULL DEFAULT 'REVIEW_MARKDOWN',
  storage_key VARCHAR(1024) COLLATE utf8mb4_bin NOT NULL,
  sha256 CHAR(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  size_bytes BIGINT UNSIGNED NOT NULL,
  coverage VARCHAR(16) NOT NULL,
  validation JSON NOT NULL,
  native_write_message_id VARCHAR(128) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  created_at DATETIME(3) NOT NULL,
  PRIMARY KEY (id),
  UNIQUE KEY uq_artifact_version (task_id,version),
  UNIQUE KEY uq_artifact_run (run_id,kind),
  UNIQUE KEY uq_artifact_task (id,task_id),
  FOREIGN KEY (run_id,task_id) REFERENCES wh_runs(id,task_id),
  CHECK (kind = 'REVIEW_MARKDOWN'),
  CHECK (coverage IN ('FULL','PARTIAL'))
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

CREATE TABLE wh_interactions (
  id CHAR(36) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  task_id CHAR(36) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  run_id CHAR(36) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  native_request_id VARCHAR(128) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  kind VARCHAR(16) NOT NULL,
  revision INT UNSIGNED NOT NULL DEFAULT 1,
  status VARCHAR(16) NOT NULL DEFAULT 'PENDING',
  schema_json JSON NOT NULL,
  answer_json JSON NULL,
  created_at DATETIME(3) NOT NULL,
  updated_at DATETIME(3) NOT NULL,
  expires_at DATETIME(3) NOT NULL,
  resolved_at DATETIME(3) NULL,
  PRIMARY KEY (id),
  UNIQUE KEY uq_interaction_native (task_id,native_request_id),
  FOREIGN KEY (run_id,task_id) REFERENCES wh_runs(id,task_id),
  CHECK (kind IN ('PERMISSION','FORM')),
  CHECK (status IN ('PENDING','ANSWERING','ANSWERED','EXPIRED','CANCELLED','UNKNOWN'))
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

CREATE TABLE wh_operations (
  id CHAR(36) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  owner_id CHAR(36) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  operation_key CHAR(36) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  action VARCHAR(32) NOT NULL,
  request_sha256 CHAR(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  task_id CHAR(36) CHARACTER SET ascii COLLATE ascii_bin NULL,
  run_id CHAR(36) CHARACTER SET ascii COLLATE ascii_bin NULL,
  status VARCHAR(16) NOT NULL DEFAULT 'ACCEPTED',
  result_json JSON NULL,
  error_code VARCHAR(64) NULL,
  created_at DATETIME(3) NOT NULL,
  updated_at DATETIME(3) NOT NULL,
  PRIMARY KEY (id),
  UNIQUE KEY uq_operation (owner_id,operation_key),
  KEY ix_operation_task (task_id,created_at),
  FOREIGN KEY (owner_id) REFERENCES wh_owners(id),
  FOREIGN KEY (task_id) REFERENCES wh_tasks(id),
  FOREIGN KEY (run_id,task_id) REFERENCES wh_runs(id,task_id),
  CHECK (status IN ('ACCEPTED','PROCESSING','SUCCEEDED','FAILED','UNKNOWN'))
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

CREATE TABLE wh_outbox (
  id CHAR(36) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  operation_id CHAR(36) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  task_id CHAR(36) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  run_id CHAR(36) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  action VARCHAR(16) NOT NULL,
  payload JSON NOT NULL,
  status VARCHAR(16) NOT NULL DEFAULT 'PENDING',
  lease_owner VARCHAR(128) CHARACTER SET ascii COLLATE ascii_bin NULL,
  lease_fence BIGINT UNSIGNED NOT NULL DEFAULT 0,
  lease_until DATETIME(3) NULL,
  attempts INT UNSIGNED NOT NULL DEFAULT 0,
  available_at DATETIME(3) NOT NULL,
  created_at DATETIME(3) NOT NULL,
  updated_at DATETIME(3) NOT NULL,
  PRIMARY KEY (id),
  UNIQUE KEY uq_outbox_operation (operation_id),
  KEY ix_outbox_queue (status,available_at,created_at,id),
  FOREIGN KEY (operation_id) REFERENCES wh_operations(id),
  FOREIGN KEY (run_id,task_id) REFERENCES wh_runs(id,task_id),
  CHECK (action IN ('DISPATCH','INTERRUPT','ANSWER')),
  CHECK (status IN ('PENDING','CLAIMED','SENDING','SENT','UNKNOWN','FAILED','CANCELLED'))
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

-- Migration seed: exactly one local execution slot, no user credentials.
INSERT INTO wh_executor_slots(slot_key,updated_at) VALUES ('local-default',UTC_TIMESTAMP(3));
-- Add report reference FK only after both tables exist.
ALTER TABLE wh_messages ADD CONSTRAINT fk_message_report
  FOREIGN KEY (report_artifact_id,task_id) REFERENCES wh_artifacts(id,task_id);
