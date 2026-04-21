-- ein holds an AES-256-GCM blob (nonce || ciphertext || tag) produced
-- by the application. NULL means no EIN is recorded.
ALTER TABLE corporations
  ADD COLUMN ein BYTEA;
