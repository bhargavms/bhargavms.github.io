# Umami analytics

See also [`bhargavms/infra` docs/umami.md](https://github.com/bhargavms/infra/blob/main/docs/umami.md).

## Site config

Configured in [`hugo.toml`](../hugo.toml):

- `params.analytics.umamiWebsiteId` — seeded UUID for `mogra.dev`
- `params.analytics.scriptName` — `t.js` (proxied by nginx to Umami)

## Routing

| Path | Handler |
|------|---------|
| `GET /t.js` | Site nginx → Umami |
| `POST /api/send` | Cloudflare tunnel → site nginx → Umami, or mogra-proxy → Umami |

The tracking script is only injected when `enabled = true` and `umamiWebsiteId` is set.
