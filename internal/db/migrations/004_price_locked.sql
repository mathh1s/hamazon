-- Set when an admin edits an item's price or set size, so default price
-- updates leave those items alone unless asked.
ALTER TABLE items ADD COLUMN price_locked INTEGER NOT NULL DEFAULT 0;
