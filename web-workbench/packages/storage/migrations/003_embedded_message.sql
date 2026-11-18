-- Embedded pages use host-issued identity, never a shared login cookie.
CREATE TABLE wh_embed_tickets (
  digest CHAR(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL PRIMARY KEY,
  owner_id CHAR(36) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  client_id VARCHAR(80) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  parent_origin VARCHAR(512) COLLATE utf8mb4_bin NOT NULL,
  expires_at DATETIME(3) NOT NULL,
  consumed_at DATETIME(3) NULL,
  FOREIGN KEY (owner_id) REFERENCES wh_owners(id),
  KEY ix_embed_ticket_expiry (expires_at)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

CREATE TABLE wh_embed_sessions (
  id CHAR(36) CHARACTER SET ascii COLLATE ascii_bin NOT NULL PRIMARY KEY,
  digest CHAR(64) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  owner_id CHAR(36) CHARACTER SET ascii COLLATE ascii_bin NOT NULL,
  parent_origin VARCHAR(512) COLLATE utf8mb4_bin NOT NULL,
  native_session_id VARCHAR(128) CHARACTER SET ascii COLLATE ascii_bin NULL,
  service_identity VARCHAR(128) CHARACTER SET ascii COLLATE ascii_bin NULL,
  state VARCHAR(16) NOT NULL DEFAULT 'NEW',
  expires_at DATETIME(3) NOT NULL,
  revoked_at DATETIME(3) NULL,
  cleaned_at DATETIME(3) NULL,
  UNIQUE KEY uq_embed_digest (digest),
  FOREIGN KEY (owner_id) REFERENCES wh_owners(id),
  CHECK (state IN ('NEW','CREATING','BOUND','FAILED')),
  CHECK (state <> 'BOUND' OR (native_session_id IS NOT NULL AND service_identity IS NOT NULL)),
  KEY ix_embed_session_expiry (expires_at)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;
