CREATE TABLE meta (
	key   TEXT PRIMARY KEY,
	value TEXT NOT NULL
);

CREATE TABLE currencies (
	id      TEXT PRIMARY KEY,
	name    TEXT NOT NULL,
	texture TEXT NOT NULL,
	sort    INTEGER NOT NULL DEFAULT 0
);

CREATE TABLE categories (
	id   TEXT PRIMARY KEY,
	name TEXT NOT NULL,
	icon TEXT NOT NULL,
	sort INTEGER NOT NULL DEFAULT 0
);

CREATE TABLE items (
	id                INTEGER PRIMARY KEY,
	name              TEXT NOT NULL,
	texture           TEXT NOT NULL,
	category_id       TEXT NOT NULL REFERENCES categories (id),
	count             INTEGER NOT NULL CHECK (count BETWEEN 1 AND 64),
	description       TEXT NOT NULL DEFAULT '',
	max_per_order     INTEGER NOT NULL CHECK (max_per_order BETWEEN 1 AND 999),
	player_limit      INTEGER CHECK (player_limit IS NULL OR player_limit >= 1),
	player_limit_days INTEGER CHECK (player_limit_days IS NULL OR player_limit_days BETWEEN 1 AND 365),
	enabled           INTEGER NOT NULL DEFAULT 1,
	sort              INTEGER NOT NULL DEFAULT 0,
	created_at        TEXT NOT NULL,
	updated_at        TEXT NOT NULL,
	CHECK ((player_limit IS NULL) = (player_limit_days IS NULL))
);

CREATE INDEX items_category ON items (category_id, sort);

CREATE TABLE item_prices (
	item_id     INTEGER NOT NULL REFERENCES items (id) ON DELETE CASCADE,
	currency_id TEXT NOT NULL REFERENCES currencies (id),
	amount      INTEGER NOT NULL CHECK (amount BETWEEN 1 AND 100000),
	PRIMARY KEY (item_id, currency_id)
);

CREATE TABLE orders (
	id                 INTEGER PRIMARY KEY,
	code               TEXT NOT NULL UNIQUE,
	username           TEXT NOT NULL,
	username_key       TEXT NOT NULL,
	note               TEXT NOT NULL DEFAULT '',
	status             TEXT NOT NULL CHECK (status IN ('pending', 'approved', 'fulfilled', 'cancelled')),
	receipt_hash       BLOB NOT NULL UNIQUE,
	idempotency_key    TEXT NOT NULL UNIQUE,
	content_hash       BLOB NOT NULL,
	client_ip          TEXT NOT NULL,
	discord_message_id TEXT,
	created_at         TEXT NOT NULL,
	updated_at         TEXT NOT NULL
);

CREATE INDEX orders_created ON orders (created_at);
CREATE INDEX orders_status ON orders (status, created_at);
CREATE INDEX orders_username ON orders (username_key, created_at);

-- Lines snapshot name, texture and unit price so later catalog edits never
-- rewrite what a player agreed to pay.
CREATE TABLE order_lines (
	order_id   INTEGER NOT NULL REFERENCES orders (id) ON DELETE CASCADE,
	position   INTEGER NOT NULL,
	item_id    INTEGER REFERENCES items (id) ON DELETE SET NULL,
	name       TEXT NOT NULL,
	texture    TEXT NOT NULL,
	count      INTEGER NOT NULL,
	quantity   INTEGER NOT NULL CHECK (quantity >= 1),
	unit_price TEXT NOT NULL,
	PRIMARY KEY (order_id, position)
);

CREATE INDEX order_lines_item ON order_lines (item_id);

CREATE TABLE order_events (
	id         INTEGER PRIMARY KEY,
	order_id   INTEGER NOT NULL REFERENCES orders (id) ON DELETE CASCADE,
	status     TEXT NOT NULL,
	admin_name TEXT,
	note       TEXT NOT NULL DEFAULT '',
	created_at TEXT NOT NULL
);

CREATE INDEX order_events_order ON order_events (order_id, id);

CREATE TABLE notifications (
	id              INTEGER PRIMARY KEY,
	order_id        INTEGER NOT NULL REFERENCES orders (id) ON DELETE CASCADE,
	kind            TEXT NOT NULL CHECK (kind IN ('order_created', 'order_updated')),
	state           TEXT NOT NULL CHECK (state IN ('pending', 'sent', 'failed')),
	attempts        INTEGER NOT NULL DEFAULT 0,
	last_error      TEXT NOT NULL DEFAULT '',
	next_attempt_at TEXT NOT NULL,
	created_at      TEXT NOT NULL,
	updated_at      TEXT NOT NULL
);

CREATE INDEX notifications_due ON notifications (state, next_attempt_at);
CREATE INDEX notifications_order ON notifications (order_id);

CREATE TABLE admins (
	id            INTEGER PRIMARY KEY,
	name          TEXT NOT NULL UNIQUE COLLATE NOCASE,
	password_hash TEXT NOT NULL,
	created_at    TEXT NOT NULL
);

CREATE TABLE admin_sessions (
	token_hash BLOB PRIMARY KEY,
	admin_id   INTEGER NOT NULL REFERENCES admins (id) ON DELETE CASCADE,
	csrf_token TEXT NOT NULL,
	created_at TEXT NOT NULL,
	expires_at TEXT NOT NULL
);
