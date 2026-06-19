DROP INDEX IF EXISTS category_rules_block_uniq;
DELETE FROM category_rules WHERE category_id IS NULL;
ALTER TABLE category_rules ALTER COLUMN category_id SET NOT NULL;
