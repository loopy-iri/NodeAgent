# ۳) افزودن نود و فروش (اپراتور)

می‌توانی از **رابط وب پنل** (`http://PANEL_IP:8080/`) یا مستقیم از **API** (`/docs`) استفاده کنی. در ادامه هر دو نشان داده شده است.

## گام ۱: افزودن نود به پنل
چیزهایی که از خروجی نصب نود لازم داری: آدرس `https://NODE_IP:8090`، **master key**، و **گواهی** نود.

از رابط وب: بخش Nodes → Add → پر کردن نام/آدرس/master key و چسباندن گواهی.

از API:
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
- `cert_pem` را برای **pin** بده. اگر خالی بگذاری، پنل بار اول گواهی را خودکار می‌گیرد (TOFU).
- سلامت نود: `GET /api/v1/nodes/{id}/health`.

## گام ۲: ساخت پلن
```bash
curl -X POST http://PANEL_IP:8080/api/v1/plans \
  -H "Authorization: Bearer $PANEL_TOKEN" -H "Content-Type: application/json" \
  -d '{"name":"50GB-30d","quota_bytes":53687091200,"duration_days":30,"max_users":50}'
```
- `quota_bytes`: حجم کل (مثال: ۵۰ گیگ = 53687091200).
- `duration_days`: مدت اعتبار.
- `max_users`: حداکثر کاربر هم‌زمانِ مشتری.

## گام ۳: ساخت مشتری
```bash
curl -X POST http://PANEL_IP:8080/api/v1/customers \
  -H "Authorization: Bearer $PANEL_TOKEN" -H "Content-Type: application/json" \
  -d '{"name":"Ali Shop","external_ref":"bot-user-12345"}'
```
`external_ref` شناسه‌ی همان مشتری در ربات فروش است (اختیاری ولی مفید).

## گام ۴: ساخت اشتراک (تحویل کلید مشتری)
```bash
curl -X POST http://PANEL_IP:8080/api/v1/customers/<CUSTOMER_ID>/subscriptions \
  -H "Authorization: Bearer $PANEL_TOKEN" -H "Content-Type: application/json" \
  -d '{"plan_id":"<PLAN_ID>","node_id":"<NODE_ID>","credit_limit_bytes":0}'
```
پاسخ — **`api_key` فقط همین یک‌بار نمایش داده می‌شود**:
```json
{
  "subscription": { "id": "...", "status": "active", "quota_bytes": 53687091200, ... },
  "api_key": "<CUSTOMER_KEY>",
  "node_address": "NODE_IP:62050"
}
```
- `credit_limit_bytes`: مقدار مجاز overage (مصرف اضافه‌ی قابل‌گزارش). `0` یعنی بدون اعتبار اضافه.

## گام ۵: تحویل به مشتری
این سه چیز را به مشتری بده:
1. **آدرس gRPC نود:** `NODE_IP:62050`
2. **گواهی نود** (همان PEM)
3. **کلید مشتری** (`api_key` بالا)

> 💡 **میان‌بر:** به‌جای جمع‌کردن دستی این موارد، در رابط وب روی اشتراک دکمه‌ی **«اتصال مشتری»** را بزن (یا API زیر را صدا بزن). همه‌چیز — آدرس gRPC، گواهی، و **inboundهای واقعی نود** — یک‌جا با دکمه‌ی کپی آماده‌ی تحویل است:
> ```bash
> curl http://PANEL_IP:8080/api/v1/subscriptions/<SUB_ID>/connection \
>   -H "Authorization: Bearer $PANEL_TOKEN"
> ```
> پاسخ شامل `grpc_address`, `protocol`, `cert_pem` و `inbounds` است. کلید مشتری اینجا نیست (فقط هنگام ساخت اشتراک نمایش داده می‌شود).

مشتری با این‌ها نود را در پنل PasarGuard خودش اضافه می‌کند → [اتصال مشتری](05-Customer-Connect).

## دیدن/تحویل کانفیگ هسته
- **کانفیگ کامل زنده‌ی نود** (برای اپراتور): `GET /api/v1/nodes/{id}/config` — کانفیگ در حال اجرا را زنده از نود می‌گیرد (نه صرفاً آخرین push‌شده).
- **inboundهای قابل‌اشتراک با مشتری** (بدون outbound/routing): `GET /api/v1/nodes/{id}/inbounds` — فقط بخش inbounds (و اگر force-inbounds تنظیم شده، همان تگ‌ها) را برمی‌گرداند تا امن به مشتری بدهی.

## مدیریت اشتراک
| کار | API |
|-----|-----|
| تعلیق | `POST /api/v1/subscriptions/{id}/suspend` |
| ازسرگیری | `POST /api/v1/subscriptions/{id}/resume` |
| افزایش حجم | `POST /api/v1/subscriptions/{id}/topup-quota` `{"add_bytes":N}` |
| تمدید (ریست مصرف + تمدید انقضا) | `POST /api/v1/subscriptions/{id}/renew` |
| حذف (deprovision مستأجر از نود) | `DELETE /api/v1/subscriptions/{id}` |
| مصرف مشتری | `GET /api/v1/customers/{id}/usage` |
| فعال/غیرفعال کل مشتری | `POST /api/v1/customers/{id}/enable` \| `/disable` |

> **suspend ≠ delete:** تعلیق فقط کاربرها را از هسته برمی‌دارد ولی رکوردها می‌مانند؛ resume/renew برشان می‌گرداند. حذف، مستأجر و داده‌هایش را پاک می‌کند.
