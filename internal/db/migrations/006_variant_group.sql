-- Items with the same group, like every color of shulker box, share one
-- slot in the shop that expands into the variants.
ALTER TABLE items ADD COLUMN variant_group TEXT NOT NULL DEFAULT '';
