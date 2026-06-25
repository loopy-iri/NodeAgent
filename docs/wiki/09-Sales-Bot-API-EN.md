**🇬🇧 English** · [🇮🇷 فارسی](09-Sales-Bot-API)

# API & Webhooks for the sales bot (developer)

The external sales bot owns the money/wallet logic; the panel only manages bytes/time/status. The bot talks to the panel via REST API and webhooks.

## Authentication
All `/api/v1/...` routes are protected by a Bearer token:
```
Authorization: Bearer <PANEL_TOKEN>
```
Interactive docs (Swagger): `http://PANEL_IP:8080/docs`.

## Typical sales flow
1. A user buys in the bot.
2. Bot → `POST /api/v1/customers` (if new; `external_ref` = the user's id in the bot).
3. Bot → `POST /api/v1/customers/{id}/subscriptions` with `plan_id` and `node_id` → gets `api_key` and `node_address` (**only once**).
4. Bot delivers to the user: `node_address` (gRPC), the node certificate, and `api_key`.
5. Tracks usage via webhook or `GET /api/v1/customers/{id}/usage`.
6. On renew/topup: `renew` or `topup-quota`. On debt: `suspend`/`disable`.

## Key endpoints
| Action | Method & path |
|--------|---------------|
| Create customer | `POST /api/v1/customers` |
| Create subscription (customer key) | `POST /api/v1/customers/{id}/subscriptions` |
| Customer connection info (gRPC/cert/inbounds) | `GET /api/v1/subscriptions/{id}/connection` |
| Node detail (host/ports/cert) | `GET /api/v1/nodes/{id}` |
| Node shareable inbounds | `GET /api/v1/nodes/{id}/inbounds` |
| Live node config | `GET /api/v1/nodes/{id}/config` |
| Customer usage (+overage) | `GET /api/v1/customers/{id}/usage` |
| Suspend/resume subscription | `POST /api/v1/subscriptions/{id}/suspend` \| `/resume` |
| Add quota | `POST /api/v1/subscriptions/{id}/topup-quota` |
| Renew | `POST /api/v1/subscriptions/{id}/renew` |
| Enable/disable customer | `POST /api/v1/customers/{id}/enable` \| `/disable` |
| Create plan/node | `POST /api/v1/plans` \| `/api/v1/nodes` |

## Webhooks (usage/overage events)
Register an endpoint:
```bash
curl -X POST http://PANEL_IP:8080/api/v1/webhooks \
  -H "Authorization: Bearer $PANEL_TOKEN" -H "Content-Type: application/json" \
  -d '{"url":"https://bot.example.com/hook","secret":"<SHARED_SECRET>","events":"*"}'
```
- The panel signs each event body with **HMAC-SHA256** (using `secret`) in the signature header.
- The bot must recompute and **verify** the signature with the same secret to reject forgeries.
- Use these events to deduct the wallet when thresholds/overage are crossed.

## Security note
- Keep the panel token and webhook secrets private.
- Put the panel behind TLS.
- The customer `api_key` is returned only once at subscription creation; deliver it securely (it can't be retrieved again; if lost, delete and recreate the subscription).
