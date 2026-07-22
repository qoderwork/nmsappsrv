-- ============================================================================
-- migrate_license_to_tenant.sql
-- ----------------------------------------------------------------------------
-- One-time schema migration: rename the `license` table to `tenant` and unify
-- all per-tenant foreign-key columns onto `tenant_id`.
--
-- WHAT THIS DOES
--   1. Renames table  `license`            -> `tenant`
--   2. Renames column `tenant.license_id`  -> `tenant.tenant_code`  (varchar)
--   3. Drops the signed-license activation columns from `tenant` that were
--      removed from the model:
--        slot, subject, issuer, features, capacity, not_before,
--        machine_fingerprint, status, verified_at, license_file_path
--   4. Renames `license_id` (int FK -> tenant.id) -> `tenant_id` in every
--      referencing table.
--   5. Renames `tenancy_id` (int FK -> tenant.id) -> `tenant_id` in every
--      referencing table.
--   6. Special case `entra_endpoint` (see its section below).
--
-- IDEMPOTENCY
--   The script is idempotent-safe WHERE POSSIBLE. Every rename/drop is guarded
--   by an information_schema check inside a helper stored procedure, so re-running
--   the script after a partial failure will simply skip steps that already
--   completed. MySQL `ALTER TABLE ... CHANGE/RENAME COLUMN` has no native
--   IF EXISTS, which is why the procedure pattern is used.
--
--   NOTE ON TRANSACTIONS: MySQL DDL (RENAME TABLE / ALTER TABLE) causes an
--   implicit commit and is NOT transactional, so these changes cannot be wrapped
--   in a single atomic transaction. Re-runnability (idempotency) is used instead
--   of a transaction to make partial completion recoverable.
--
-- MYSQL VERSION
--   Targets MySQL 8.0+ (the deployment image is mysql:8.0). Column renames use
--   `ALTER TABLE ... RENAME COLUMN old TO new`, which preserves the existing
--   column definition (type / NULL-ability / default) automatically -- so we do
--   not need to restate each column's type. If you must run this on MySQL 5.7,
--   replace the RENAME COLUMN inside `migrate_rename_column` with a
--   `CHANGE COLUMN <old> <new> <full column definition>` statement.
--
-- RUN ONCE
--   Although guarded, this is designed as a one-time migration for existing
--   deployments that still have the old schema. Fresh deployments created by the
--   current model already have the new schema and do not need this script.
-- ============================================================================

-- ----------------------------------------------------------------------------
-- Helper procedures (recreated on each run so the script itself stays re-runnable)
-- ----------------------------------------------------------------------------
DROP PROCEDURE IF EXISTS migrate_rename_table;
DROP PROCEDURE IF EXISTS migrate_rename_column;
DROP PROCEDURE IF EXISTS migrate_drop_column;

DELIMITER $$

-- Rename a table only if the old table exists and the new one does not.
CREATE PROCEDURE migrate_rename_table(IN p_old VARCHAR(64), IN p_new VARCHAR(64))
BEGIN
    IF EXISTS (SELECT 1 FROM information_schema.TABLES
               WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = p_old)
       AND NOT EXISTS (SELECT 1 FROM information_schema.TABLES
               WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = p_new)
    THEN
        SET @ddl = CONCAT('RENAME TABLE `', p_old, '` TO `', p_new, '`');
        PREPARE stmt FROM @ddl;
        EXECUTE stmt;
        DEALLOCATE PREPARE stmt;
    END IF;
END$$

-- Rename a column only if the old column exists and the new one does not.
-- Uses RENAME COLUMN (MySQL 8.0+) so the column definition is preserved.
CREATE PROCEDURE migrate_rename_column(IN p_table VARCHAR(64), IN p_old VARCHAR(64), IN p_new VARCHAR(64))
BEGIN
    IF EXISTS (SELECT 1 FROM information_schema.COLUMNS
               WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = p_table AND COLUMN_NAME = p_old)
       AND NOT EXISTS (SELECT 1 FROM information_schema.COLUMNS
               WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = p_table AND COLUMN_NAME = p_new)
    THEN
        SET @ddl = CONCAT('ALTER TABLE `', p_table, '` RENAME COLUMN `', p_old, '` TO `', p_new, '`');
        PREPARE stmt FROM @ddl;
        EXECUTE stmt;
        DEALLOCATE PREPARE stmt;
    END IF;
END$$

-- Drop a column only if it exists.
CREATE PROCEDURE migrate_drop_column(IN p_table VARCHAR(64), IN p_col VARCHAR(64))
BEGIN
    IF EXISTS (SELECT 1 FROM information_schema.COLUMNS
               WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = p_table AND COLUMN_NAME = p_col)
    THEN
        SET @ddl = CONCAT('ALTER TABLE `', p_table, '` DROP COLUMN `', p_col, '`');
        PREPARE stmt FROM @ddl;
        EXECUTE stmt;
        DEALLOCATE PREPARE stmt;
    END IF;
END$$

DELIMITER ;

-- ============================================================================
-- STEP 1: Rename table `license` -> `tenant`
-- ============================================================================
CALL migrate_rename_table('license', 'tenant');

-- ============================================================================
-- STEP 2: Rename `tenant.license_id` (varchar) -> `tenant.tenant_code`
-- ============================================================================
CALL migrate_rename_column('tenant', 'license_id', 'tenant_code');

-- ============================================================================
-- STEP 3: Drop removed signed-license activation columns from `tenant`
-- ============================================================================
CALL migrate_drop_column('tenant', 'slot');
CALL migrate_drop_column('tenant', 'subject');
CALL migrate_drop_column('tenant', 'issuer');
CALL migrate_drop_column('tenant', 'features');
CALL migrate_drop_column('tenant', 'capacity');
CALL migrate_drop_column('tenant', 'not_before');
CALL migrate_drop_column('tenant', 'machine_fingerprint');
CALL migrate_drop_column('tenant', 'status');
CALL migrate_drop_column('tenant', 'verified_at');
CALL migrate_drop_column('tenant', 'license_file_path');

-- ============================================================================
-- STEP 4: Rename `license_id` (int FK -> tenant.id) -> `tenant_id`
-- ----------------------------------------------------------------------------
-- Tables marked (*) were not in the original migration list but are renamed here
-- because the current application model maps them to `tenant_id`; skipping them
-- would break those queries. `gnbstatistic_data` is a Java-side table included
-- per the migration spec (the guard makes it a no-op if the column is absent).
-- ============================================================================
CALL migrate_rename_column('alarm',                     'license_id', 'tenant_id');
CALL migrate_rename_column('alarm_filter',              'license_id', 'tenant_id');
CALL migrate_rename_column('batch_process_file',        'license_id', 'tenant_id');
CALL migrate_rename_column('batch_process_file_send_log','license_id', 'tenant_id'); -- (*)
CALL migrate_rename_column('black_list_operation_log',  'license_id', 'tenant_id');
CALL migrate_rename_column('cbsd_info',                 'license_id', 'tenant_id');
CALL migrate_rename_column('cbsd_sas_config',           'license_id', 'tenant_id'); -- (*)
CALL migrate_rename_column('config_upload_log',         'license_id', 'tenant_id');
CALL migrate_rename_column('cpe_element',               'license_id', 'tenant_id');
CALL migrate_rename_column('device_group',              'license_id', 'tenant_id');
CALL migrate_rename_column('element_black_list',        'license_id', 'tenant_id');
CALL migrate_rename_column('gnbstatistic_data',         'license_id', 'tenant_id'); -- (Java-side)
CALL migrate_rename_column('login_log',                 'license_id', 'tenant_id'); -- (*)
CALL migrate_rename_column('mml_set',                   'license_id', 'tenant_id');
CALL migrate_rename_column('mnormal_file',              'license_id', 'tenant_id'); -- (*)
CALL migrate_rename_column('monitor_task',              'license_id', 'tenant_id');
CALL migrate_rename_column('ne_log',                    'license_id', 'tenant_id'); -- (*)
CALL migrate_rename_column('north_report',              'license_id', 'tenant_id');
CALL migrate_rename_column('parameter',                 'license_id', 'tenant_id');
CALL migrate_rename_column('parameter_monitor_config',  'license_id', 'tenant_id');
CALL migrate_rename_column('parameter_set',             'license_id', 'tenant_id');
CALL migrate_rename_column('psap_id',                   'license_id', 'tenant_id'); -- (*)
CALL migrate_rename_column('remote_upload',             'license_id', 'tenant_id');
CALL migrate_rename_column('sas_config',                'license_id', 'tenant_id');
CALL migrate_rename_column('site_info',                 'license_id', 'tenant_id');
CALL migrate_rename_column('spatial_file_market',       'license_id', 'tenant_id'); -- (*)
CALL migrate_rename_column('ssh_label',                 'license_id', 'tenant_id');
CALL migrate_rename_column('sys_user',                  'license_id', 'tenant_id');
CALL migrate_rename_column('tbg',                       'license_id', 'tenant_id'); -- (*)

-- ============================================================================
-- STEP 5: Rename `tenancy_id` (int FK -> tenant.id) -> `tenant_id`
-- ----------------------------------------------------------------------------
-- `aos_used_data`, `psap_id_sync_log` (listed as "sync_psap_id_log") and
-- `ztp_log` are Java-side tables included per the migration spec; their Go
-- models have no tenant column, and the guard makes the call a safe no-op if
-- the column is absent. `cbsd_cert_file_send_task` is the correct table name
-- for the entry listed as "cbsdcert_file_send_task".
-- ============================================================================
CALL migrate_rename_column('alarm_library',                 'tenancy_id', 'tenant_id');
CALL migrate_rename_column('alarm_template',                'tenancy_id', 'tenant_id');
CALL migrate_rename_column('aos_used_data',                 'tenancy_id', 'tenant_id'); -- (Java-side)
CALL migrate_rename_column('backup_or_restore_task',        'tenancy_id', 'tenant_id');
CALL migrate_rename_column('batch_add_object_task',         'tenancy_id', 'tenant_id');
CALL migrate_rename_column('batch_configuration_log',       'tenancy_id', 'tenant_id');
CALL migrate_rename_column('ca_task',                       'tenancy_id', 'tenant_id');
CALL migrate_rename_column('cbsd_cert_file_send_task',      'tenancy_id', 'tenant_id');
CALL migrate_rename_column('core_network',                  'tenancy_id', 'tenant_id');
CALL migrate_rename_column('dashboard_pm_statistic_data',   'tenancy_id', 'tenant_id');
CALL migrate_rename_column('device_m_normal_file',          'tenancy_id', 'tenant_id');
CALL migrate_rename_column('error_info',                    'tenancy_id', 'tenant_id');
CALL migrate_rename_column('eu_and_ru_batch_upgrade_log',   'tenancy_id', 'tenant_id');
CALL migrate_rename_column('kpi_alarm_template',            'tenancy_id', 'tenant_id');
CALL migrate_rename_column('mr_upload_task',                'tenancy_id', 'tenant_id');
CALL migrate_rename_column('nms_backup_and_revert_task',    'tenancy_id', 'tenant_id');
CALL migrate_rename_column('north_interface_log',           'tenancy_id', 'tenant_id');
CALL migrate_rename_column('parameter_deployment_log',      'tenancy_id', 'tenant_id');
CALL migrate_rename_column('parameter_deployment_template', 'tenancy_id', 'tenant_id');
CALL migrate_rename_column('parameter_template',            'tenancy_id', 'tenant_id');
CALL migrate_rename_column('pdcp_traffic',                  'tenancy_id', 'tenant_id');
CALL migrate_rename_column('performance_kpi',               'tenancy_id', 'tenant_id');
CALL migrate_rename_column('performance_kpi_set',           'tenancy_id', 'tenant_id');
CALL migrate_rename_column('performance_kpi_template',      'tenancy_id', 'tenant_id');
CALL migrate_rename_column('pm_file_log',                   'tenancy_id', 'tenant_id');
CALL migrate_rename_column('pm_replenish_task',             'tenancy_id', 'tenant_id');
CALL migrate_rename_column('psap_id_sync_log',              'tenancy_id', 'tenant_id'); -- (Java-side; listed as sync_psap_id_log)
CALL migrate_rename_column('radius',                        'tenancy_id', 'tenant_id');
CALL migrate_rename_column('reboot_task',                   'tenancy_id', 'tenant_id');
CALL migrate_rename_column('reset_task',                    'tenancy_id', 'tenant_id');
CALL migrate_rename_column('role',                          'tenancy_id', 'tenant_id');
CALL migrate_rename_column('rollback_task',                 'tenancy_id', 'tenant_id');
CALL migrate_rename_column('shutdown_my_task',              'tenancy_id', 'tenant_id');
CALL migrate_rename_column('ssh_access_timer_task',         'tenancy_id', 'tenant_id');
CALL migrate_rename_column('sys_area',                      'tenancy_id', 'tenant_id');
CALL migrate_rename_column('system_operator_log',           'tenancy_id', 'tenant_id');
CALL migrate_rename_column('upgrade_auto_task',             'tenancy_id', 'tenant_id');
CALL migrate_rename_column('upgrade_file',                  'tenancy_id', 'tenant_id');
CALL migrate_rename_column('upgrade_log',                   'tenancy_id', 'tenant_id');
CALL migrate_rename_column('upgrade_task',                  'tenancy_id', 'tenant_id');
CALL migrate_rename_column('ztp_log',                       'tenancy_id', 'tenant_id'); -- (Java-side)

-- ============================================================================
-- STEP 6: entra_endpoint -- special case
-- ----------------------------------------------------------------------------
-- entra_endpoint does NOT have a `license_id` column. It has two columns that
-- both reference the tenant concept and both need renaming, so it cannot go
-- through the single-column bulk rules above:
--   * tenancy_id        VARCHAR(255) -> tenant_id          (Entra cloud tenant
--                         GUID; this is a string, NOT an int FK to tenant.id)
--   * tenancy_id_in_nms INT          -> tenant_id_in_nms   (the actual int FK to
--                         tenant.id; renamed to tenant_id_in_nms, not tenant_id,
--                         to avoid colliding with the varchar column above)
-- ============================================================================
CALL migrate_rename_column('entra_endpoint', 'tenancy_id',        'tenant_id');
CALL migrate_rename_column('entra_endpoint', 'tenancy_id_in_nms', 'tenant_id_in_nms');

-- ============================================================================
-- Cleanup helper procedures
-- ============================================================================
DROP PROCEDURE IF EXISTS migrate_rename_table;
DROP PROCEDURE IF EXISTS migrate_rename_column;
DROP PROCEDURE IF EXISTS migrate_drop_column;

-- ============================================================================
-- Done. The script is safe to re-run; completed steps are skipped automatically.
-- ============================================================================
