-- A category_rule with a NULL category_id is a "block": a merchant the user
-- never wants offered as a routing rule (e.g. an ambiguous provider like PAYME).
-- It never routes — MatchRule/ResolveForIngest inner-join categories, so NULL
-- rows are excluded automatically.
ALTER TABLE category_rules ALTER COLUMN category_id DROP NOT NULL;

-- Blocks are unique per merchant (stored lower-cased); routing rules are unaffected.
CREATE UNIQUE INDEX category_rules_block_uniq
    ON category_rules (pattern) WHERE category_id IS NULL;
