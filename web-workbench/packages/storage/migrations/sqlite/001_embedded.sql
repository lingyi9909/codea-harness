CREATE TABLE wh_owners (id TEXT PRIMARY KEY, subject TEXT NOT NULL UNIQUE);
CREATE TABLE wh_embed_tickets (
  digest TEXT PRIMARY KEY, owner_id TEXT NOT NULL REFERENCES wh_owners(id), client_id TEXT NOT NULL,
  parent_origin TEXT NOT NULL, expires INTEGER NOT NULL, consumed INTEGER NOT NULL DEFAULT 0
);
CREATE TABLE wh_embed_sessions (
  id TEXT PRIMARY KEY, digest TEXT NOT NULL UNIQUE, owner_id TEXT NOT NULL REFERENCES wh_owners(id),
  parent_origin TEXT NOT NULL, expires INTEGER NOT NULL, native_session_id TEXT, service_identity TEXT,
  access_mode TEXT NOT NULL DEFAULT 'HOST' CHECK(access_mode IN ('HOST','ANONYMOUS')),
  state TEXT NOT NULL DEFAULT 'NEW' CHECK(state IN ('NEW','CREATING','BOUND','FAILED')),
  revoked INTEGER NOT NULL DEFAULT 0, cleaned INTEGER NOT NULL DEFAULT 0, recycled INTEGER NOT NULL DEFAULT 0,
  query_fingerprint TEXT, query_state TEXT NOT NULL DEFAULT 'UNUSED' CHECK(query_state IN ('UNUSED','RUNNING','READY','FAILED')),
  query_evidence TEXT,
  CHECK(state <> 'BOUND' OR (native_session_id IS NOT NULL AND service_identity IS NOT NULL))
);
CREATE INDEX ix_embed_slot ON wh_embed_sessions(owner_id, cleaned, recycled);
CREATE INDEX ix_embed_expiry ON wh_embed_sessions(expires);
CREATE TABLE wh_chat_session_bindings (
  service_identity TEXT NOT NULL, native_session_id TEXT NOT NULL, owner_id TEXT NOT NULL REFERENCES wh_owners(id),
  scenario TEXT NOT NULL, directory TEXT NOT NULL, deleted INTEGER NOT NULL DEFAULT 0,
  PRIMARY KEY(service_identity,native_session_id)
);
CREATE TABLE wh_embed_locks (owner_id TEXT PRIMARY KEY, nonce TEXT NOT NULL, pid INTEGER NOT NULL);
