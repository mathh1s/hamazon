-- Dimensions besides the overworld a player needs access to for the item,
-- comma separated, e.g. "nether" or "nether,end".
ALTER TABLE items ADD COLUMN dimensions TEXT NOT NULL DEFAULT '';
