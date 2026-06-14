DROP TABLE IF EXISTS category_rules;
DROP INDEX IF EXISTS categories_system_key_uniq;
ALTER TABLE categories DROP COLUMN IF EXISTS system_key;
DROP INDEX IF EXISTS accounts_card_last4_uniq;
ALTER TABLE accounts DROP COLUMN IF EXISTS card_last4;
