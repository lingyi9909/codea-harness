-- Anonymous iframe grants retain their execution slot across API restarts.
ALTER TABLE wh_embed_sessions
  ADD COLUMN access_mode ENUM('HOST','ANONYMOUS') NOT NULL DEFAULT 'HOST',
  ADD COLUMN recycled_at DATETIME(3) NULL,
  ADD INDEX ix_embed_slot (owner_id, cleaned_at, recycled_at);

ALTER TABLE wh_embed_sessions
  ADD COLUMN query_fingerprint CHAR(64) NULL,
  ADD COLUMN query_state ENUM('UNUSED','RUNNING','READY','FAILED') NOT NULL DEFAULT 'UNUSED',
  ADD COLUMN query_evidence MEDIUMTEXT NULL;
