-- Additive migration. Do not change 001 or automatically reassign legacy owners.
CREATE TABLE wh_user_profiles (
  owner_id CHAR(36) CHARACTER SET ascii COLLATE ascii_bin NOT NULL PRIMARY KEY,
  username VARCHAR(80) COLLATE utf8mb4_bin NOT NULL,
  phone_normalized VARCHAR(16) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  status VARCHAR(16) NOT NULL DEFAULT 'ACTIVE',
  created_at DATETIME(3) NOT NULL,
  updated_at DATETIME(3) NOT NULL,
  UNIQUE KEY uq_profile_phone (phone_normalized),
  FOREIGN KEY (owner_id) REFERENCES wh_owners(id),
  CHECK (status IN ('ACTIVE','DISABLED'))
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

CREATE TABLE wh_web_sessions (
  id CHAR(36) CHARACTER SET ascii COLLATE ascii_bin NOT NULL PRIMARY KEY,
  owner_id CHAR(36) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  token_digest CHAR(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  csrf_digest CHAR(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  expires_at DATETIME(3) NOT NULL,
  revoked_at DATETIME(3) NULL,
  created_at DATETIME(3) NOT NULL,
  UNIQUE KEY uq_web_token (token_digest),
  KEY ix_web_owner (owner_id, expires_at),
  KEY ix_web_expiry (expires_at),
  FOREIGN KEY (owner_id) REFERENCES wh_owners(id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

CREATE TABLE wh_chat_session_bindings (
  service_identity VARCHAR(128) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  native_session_id VARCHAR(128) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  owner_id CHAR(36) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  scenario VARCHAR(32) NOT NULL,
  directory VARCHAR(4096) COLLATE utf8mb4_bin NOT NULL,
  created_at DATETIME(3) NOT NULL,
  deleted_at DATETIME(3) NULL,
  PRIMARY KEY (service_identity,native_session_id),
  KEY ix_chat_owner (owner_id,scenario,created_at),
  FOREIGN KEY (owner_id) REFERENCES wh_owners(id),
  CHECK (scenario IN ('direct-chat','log-inspection','message-inspection','data-analysis'))
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;
