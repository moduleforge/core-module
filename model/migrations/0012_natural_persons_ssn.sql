-- ssn holds an AES-256-GCM blob (nonce || ciphertext || tag) produced
-- by the application. NULL means no SSN is recorded. The database never
-- sees plaintext.
ALTER TABLE natural_persons
  ADD COLUMN ssn BYTEA;
