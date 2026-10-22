-- Optional delivery point a player picks at checkout. Y may be left out.
ALTER TABLE orders ADD COLUMN delivery_dimension TEXT NOT NULL DEFAULT '';
ALTER TABLE orders ADD COLUMN delivery_x INTEGER;
ALTER TABLE orders ADD COLUMN delivery_y INTEGER;
ALTER TABLE orders ADD COLUMN delivery_z INTEGER;
