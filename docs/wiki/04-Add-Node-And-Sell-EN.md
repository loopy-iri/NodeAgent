**🇬🇧 English** · [🇮🇷 فارسی](04-Add-Node-And-Sell)

# 3) Add a node and sell (operator)

Use the **web panel** (`http://PANEL_IP:8080/`) or the **API** (`/docs`). Both are shown below.

## Step 1: add the node to the panel
You need from the node install output: address `https://NODE_IP:8090`, the **master key**, and the **certificate**.

From the web UI: Nodes → Add → fill name/address/master key and paste the certificate.

> After adding, click **"Details"** on the node to see all its connection info with copy buttons: **IP/host, service (HTTP) port, gRPC address & port, protocol, and certificate**. If you installed the node on a non-default gRPC port, set the **"gRPC port"** field when registering (default `62050`).

From the API:
```bash
curl -X POST http://PANEL_IP:8080/api/v1/nodes \
  -H "Authorization: Bearer $PANEL_TOKEN" -H "Content-Type: application/json" \
  -d '{
    "name": "node-de-1",
    "address": "https://NODE_IP:8090",
    "master_key": "<MASTER_KEY>",
    "cert_pem": "-----BEGIN CERTIFICATE-----\n...\n-----END CERTIFICATE-----"
  }'
```
- Provide `cert_pem` to **pin** it. If empty, the panel fetches and pins the node's current cert automatically (TOFU).
- Health: `GET /api/v1/nodes/{id}/health`.

## Step 2: create a plan
```bash
curl -X POST http://PANEL_IP:8080/api/v1/plans \
  -H "Authorization: Bearer $PANEL_TOKEN" -H "Content-Type: application/json" \
  -d '{"name":"50GB-30d","quota_bytes":53687091200,"duration_days":30,"max_users":50}'
```

## Step 3: create a customer
```bash
curl -X POST http://PANEL_IP:8080/api/v1/customers \
  -H "Authorization: Bearer $PANEL_TOKEN" -H "Content-Type: application/json" \
  -d '{"name":"Ali Shop","external_ref":"bot-user-12345"}'
```

## Step 4: create a subscription (delivers the customer key)
```bash
curl -X POST http://PANEL_IP:8080/api/v1/customers/<CUSTOMER_ID>/subscriptions \
  -H "Authorization: Bearer $PANEL_TOKEN" -H "Content-Type: application/json" \
  -d '{"plan_id":"<PLAN_ID>","node_id":"<NODE_ID>","credit_limit_bytes":0}'
```
Response — **`api_key` is shown only once**:
```json
{ "subscription": {...}, "api_key": "<CUSTOMER_KEY>", "node_address": "NODE_IP:62050" }
```

## Step 5: hand off to the customer
Give the customer:
1. **Node gRPC address:** `NODE_IP:62050`
2. **Node certificate** (the PEM)
3. **Customer key** (`api_key` above)

> 💡 **Shortcut:** instead of collecting these manually, click **"Customer connection"** on the subscription in the web UI (or call the API below). Everything — gRPC address, certificate, and the **node's real inbounds** — is ready with copy buttons:
> ```bash
> curl http://PANEL_IP:8080/api/v1/subscriptions/<SUB_ID>/connection \
>   -H "Authorization: Bearer $PANEL_TOKEN"
> ```
> The response has `grpc_address`, `protocol`, `cert_pem` and `inbounds`. The customer key is not included (shown only at creation).

The customer adds the node in their own PasarGuard panel → [Connect from PasarGuard](05-Customer-Connect-EN).

## Viewing/sharing the core config
- **Full live node config** (operator): `GET /api/v1/nodes/{id}/config` — fetches the running config live (not just the last pushed).
- **Customer-shareable inbounds** (no outbounds/routing): `GET /api/v1/nodes/{id}/inbounds` — returns only inbounds (and only forced tags if set), safe to give the customer. The Reality private key is stripped and the public key added.

## Subscription management
| Action | API |
|--------|-----|
| Suspend | `POST /api/v1/subscriptions/{id}/suspend` |
| Resume | `POST /api/v1/subscriptions/{id}/resume` |
| Add quota | `POST /api/v1/subscriptions/{id}/topup-quota` `{"add_bytes":N}` |
| Renew (reset usage + extend expiry) | `POST /api/v1/subscriptions/{id}/renew` |
| Delete (deprovision tenant) | `DELETE /api/v1/subscriptions/{id}` |
| Customer usage | `GET /api/v1/customers/{id}/usage` |
| Enable/disable whole customer | `POST /api/v1/customers/{id}/enable` \| `/disable` |

> **suspend ≠ delete:** suspending only removes users from the core but keeps records; resume/renew restores them. Delete wipes the tenant and its data.
