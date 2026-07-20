-- Test fixture: deliberately not idempotent (no IF NOT EXISTS) so a probe
-- race between concurrent Run calls fails loudly instead of hiding.
CREATE TABLE smig_run_fixture (
    id VARCHAR(32) NOT NULL PRIMARY KEY,
    payload VARCHAR(64) NOT NULL,
    INDEX idx_smig_run_fixture__payload (payload)
);
