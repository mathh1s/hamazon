# Hamazon

A private item shop for one Minecraft server, styled like the game's own inventory screens. Players pick items, pay with diamonds, emeralds and gold ingots in game, and every order lands in Discord for the admins.

Orders are purchase requests: nothing is taken from or added to anyone's inventory automatically. An admin collects the payment and hands over the items in game, then marks the order as fulfilled.

## How it works

- Players open the shop with a shared server password, then browse a creative-inventory style catalog, fill a chest-style cart and check out with their Minecraft username (Bedrock players via Floodgate use the `.` prefix).
- Each order is saved first, then posted to Discord as an embed. If Discord is down the message is retried with backoff; admins can retry by hand after that.
- The player gets a book-style receipt with a private link. It can be printed or saved as a PDF, and the receipt list remembers past orders on that device.
- Admins log in at `/admin` to manage orders (Pending, Approved, Fulfilled, Cancelled) and items. Status changes edit the original Discord message.
- Every item has a per order cap and can have a per player limit over a rolling number of days, for things that are hard to get.

## Requirements

- Docker with Compose
- A Minecraft Java client jar of the version your server runs (for textures and the font)
- A Discord webhook for the channel that should receive orders

## Setup

### 1. Configure

```sh
cp .env.example .env
```

Fill in `.env`:

| Variable | What it does |
| --- | --- |
| `HAMAZON_PORT` | Host port the shop is published on. Required, there is no default. |
| `HAMAZON_SHOP_PASSWORD` | Password players enter once per device. Changing it logs everyone out. |
| `HAMAZON_SESSION_SECRET` | At least 32 random characters, e.g. `openssl rand -hex 32`. Signs cookies and receipt links, keep it stable. |
| `HAMAZON_DISCORD_WEBHOOK_URL` | Discord channel webhook (Server Settings > Integrations > Webhooks). Only used by the server, never sent to browsers. |
| `HAMAZON_PUBLIC_URL` | Public URL of the shop, e.g. `https://shop.example.com` (a bare host name means https). Optional. Enables the admin link and item thumbnail in Discord messages. |
| `HAMAZON_TRUST_PROXY` | `true` when running behind a reverse proxy that sets `X-Forwarded-For` and `X-Forwarded-Proto`. Needed for correct rate limits and secure cookies. |
| `HAMAZON_FLOODGATE_PREFIX` | Username prefix of Bedrock players, `.` by default. Empty disables Bedrock names. |

### 2. Build and extract the game assets

Mojang's textures and font cannot be shipped in this repository, so Hamazon renders them from your own client jar. Use the jar of the version your server runs, for example from PrismLauncher:

```sh
docker compose build
docker compose run --rm \
  -v "$HOME/.local/share/PrismLauncher/libraries/com/mojang/minecraft/26.2/minecraft-26.2-client.jar:/client.jar:ro" \
  hamazon extract --jar /client.jar
```

The jar path differs per launcher. The official launcher keeps it at `.minecraft/versions/<version>/<version>.jar`. If you do not have one at hand, download it from the version manifest:

```sh
curl -s https://piston-meta.mojang.com/mc/game/version_manifest_v2.json \
  | jq -r '.versions[] | select(.id=="26.2") | .url' \
  | xargs curl -s | jq -r '.downloads.client.url' \
  | xargs curl -o client.jar
```

The extract step renders every item icon (block items with their inventory angle and lighting), copies the GUI sprites and converts the bitmap font into a TrueType file. It takes about half a minute and writes into the data volume. Run it again after a Minecraft update.

### 3. Create an admin

```sh
docker compose run --rm hamazon admin add mathis
```

It asks for a password (at least 10 characters). Other commands: `admin passwd <name>`, `admin remove <name>`, `admin list`.

### 4. Start

```sh
docker compose up -d
```

The shop runs on `http://<host>:$HAMAZON_PORT`, the admin area on `/admin`. On first start the catalog is filled with about 150 items and balanced prices. After that the catalog only changes through the admin area, restarts never overwrite it.

### Behind a reverse proxy

Point the proxy at `127.0.0.1:$HAMAZON_PORT`, serve it over HTTPS, set `HAMAZON_TRUST_PROXY=true` and make sure the proxy sets `X-Forwarded-For` and `X-Forwarded-Proto`. Caddy example:

```
shop.example.com {
	reverse_proxy 127.0.0.1:8123
}
```

## Prices and the economy

- A price is one or two currencies that are both paid, e.g. 2 diamonds and 8 emeralds. The shop never converts between currencies.
- Prices are per set: an item can be sold as single pieces or in sets (64 oak planks, 16 ender pearls).
- Rough baseline used for the default catalog: 1 diamond is worth about 4 emeralds or 8 gold ingots. Change anything in the admin area to match your server.
- When a price changes while a player has the item in their cart, checkout is blocked until they confirm the new price.

To start with your own catalog instead of the default one, point `HAMAZON_SEED_FILE` at a YAML file in the same format as `internal/catalog/seed.yaml` before the first start.

## Data and backups

Everything lives in the `hamazon-data` volume: the SQLite database (`hamazon.db`) and the extracted assets. Back up the database with:

```sh
docker compose exec hamazon sh -c 'cp /data/hamazon.db /data/backup.db'
docker compose cp hamazon:/data/backup.db ./hamazon-backup.db
```

## Security notes

- The shop password, admin sessions and receipt tokens are checked on the server. Admin pages are protected by a server side session and a CSRF token, not by a hidden URL.
- Only hashes of admin sessions and receipt tokens are stored.
- Player input in Discord messages is escaped and mentions are disabled, so a username or note cannot ping anyone.
- Order creation, password attempts and receipt lookups are rate limited per IP. Identical orders within two minutes are rejected and a retried checkout returns the same order.

## Development

```sh
# backend
HAMAZON_DATA_DIR=./data HAMAZON_SHOP_PASSWORD=dev HAMAZON_SESSION_SECRET=dev-secret-dev-secret-dev-secret-00 go run ./cmd/hamazon

# frontend with hot reload, proxies /api and /mc to the backend on :8080
cd web && npm install && npm run dev

go test ./...
```

Not an official Minecraft product. Not approved by or associated with Mojang or Microsoft.
