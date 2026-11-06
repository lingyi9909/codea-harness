-- DBA reference ONLY. First create NEW dedicated password-protected accounts via
-- company provisioning, on the isolated server. Change localhost to the exact
-- approved client host. Do NOT use %, existing users, inherited roles or root DSNs.
-- No credentials here. SHOW GRANTS must show no other privileges or roles.
GRANT SELECT ON dbassist_t1_orders.v_orders TO 'dbassist_orders_ro'@'localhost';
GRANT SELECT ON dbassist_t1_orders.v_items TO 'dbassist_orders_ro'@'localhost';
GRANT SELECT ON dbassist_t1_orders.v_notes TO 'dbassist_orders_ro'@'localhost';
GRANT SELECT ON dbassist_t1_orders.v_canary TO 'dbassist_orders_ro'@'localhost';
GRANT SELECT ON dbassist_t1_orders.write_probe TO 'dbassist_orders_ro'@'localhost';
GRANT SELECT ON dbassist_t1_payments.v_payments TO 'dbassist_payments_ro'@'localhost';
GRANT SELECT ON dbassist_t1_payments.v_canary TO 'dbassist_payments_ro'@'localhost';
SHOW GRANTS FOR 'dbassist_orders_ro'@'localhost';
SHOW GRANTS FOR 'dbassist_payments_ro'@'localhost';
-- Observer is a SEPARATE DBA session, never an Agent/DBHub account.
-- DBA must inspect view definers, roles and indirect privileges independently.
