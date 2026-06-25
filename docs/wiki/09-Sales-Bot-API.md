# API و Webhook برای ربات فروش (توسعه‌دهنده)

ربات فروشِ بیرونی منطق پول/کیف‌پول را دارد؛ پنل فقط بایت/زمان/وضعیت را مدیریت می‌کند. ربات از طریق REST API و Webhook با پنل حرف می‌زند.

## احراز هویت
همه‌ی مسیرهای `/api/v1/...` با Bearer token محافظت می‌شوند:
```
Authorization: Bearer <PANEL_TOKEN>
```
مستندات تعاملی (Swagger): `http://PANEL_IP:8080/docs`.

## جریان معمول فروش
1. کاربر در ربات خرید می‌کند.
2. ربات → `POST /api/v1/customers` (اگر مشتری جدید است؛ `external_ref` = آیدی کاربر در ربات).
3. ربات → `POST /api/v1/customers/{id}/subscriptions` با `plan_id` و `node_id` → `api_key` و `node_address` را می‌گیرد (**فقط یک‌بار**).
4. ربات این سه چیز را به کاربر تحویل می‌دهد: `node_address` (gRPC)، گواهی نود، و `api_key`.
5. مصرف را با webhook یا `GET /api/v1/customers/{id}/usage` دنبال می‌کند.
6. هنگام تمدید/شارژ: `renew` یا `topup-quota`. هنگام بدهی: `suspend`/`disable`.

## مسیرهای کلیدی
| کار | متد و مسیر |
|-----|-----------|
| ساخت مشتری | `POST /api/v1/customers` |
| ساخت اشتراک (کلید مشتری) | `POST /api/v1/customers/{id}/subscriptions` |
| مصرف مشتری (+overage) | `GET /api/v1/customers/{id}/usage` |
| تعلیق/ازسرگیری اشتراک | `POST /api/v1/subscriptions/{id}/suspend` \| `/resume` |
| افزایش حجم | `POST /api/v1/subscriptions/{id}/topup-quota` |
| تمدید | `POST /api/v1/subscriptions/{id}/renew` |
| فعال/غیرفعال مشتری | `POST /api/v1/customers/{id}/enable` \| `/disable` |
| ساخت پلن/نود | `POST /api/v1/plans` \| `/api/v1/nodes` |

## Webhook (رویدادهای مصرف/overage)
ثبت endpoint:
```bash
curl -X POST http://PANEL_IP:8080/api/v1/webhooks \
  -H "Authorization: Bearer $PANEL_TOKEN" -H "Content-Type: application/json" \
  -d '{"url":"https://bot.example.com/hook","secret":"<SHARED_SECRET>","events":"*"}'
```
- پنل هر رویداد را با **HMAC-SHA256** روی بدنه امضا می‌کند (با `secret`) و در هدر امضا می‌فرستد.
- ربات باید امضا را با همان secret بازمحاسبه و **بررسی** کند تا جعلی نباشد.
- از این رویدادها برای منفی‌کردن کیف‌پول هنگام عبور از آستانه/overage استفاده کن.

## نکته‌ی امنیتی
- توکن پنل و secret‌های webhook را محرمانه نگه دار.
- پنل را پشت TLS قرار بده.
- `api_key` مشتری فقط یک‌بار در پاسخ ساخت اشتراک برمی‌گردد؛ آن را امن به کاربر برسان (دوباره قابل بازیابی نیست؛ در صورت گم‌شدن، اشتراک را حذف و دوباره بساز).
