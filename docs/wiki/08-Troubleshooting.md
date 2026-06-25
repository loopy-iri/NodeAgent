<div dir="rtl">

[🇬🇧 English](08-Troubleshooting-EN) · **🇮🇷 فارسی**

# عیب‌یابی

## نصب: `cannot create regular file '/opt/.../...': No such file or directory`
باگ نسخه‌ی قدیمی اسکریپت بود (پوشه‌ی نصب قبل از کپی باینری ساخته نمی‌شد). در v0.2.2 رفع شد. کافی است دوباره همان دستور `install` را اجرا کنی (اسکریپت از `main` گرفته می‌شود).

## لاگ `Failed to load API Key` یا `Failed to load env file`
بی‌خطر است. این پیام از کد پایه‌ی upstream می‌آید؛ اسکریپت یک `API_KEY` ساختگی می‌نویسد تا ساکت شود. عملکرد چند-مستأجری مستقل از آن است.

## سرویس کرش‌لوپ می‌کند / `bind: address already in use`
اگر `status` نشان داد `activating (auto-restart)` و شمارنده‌ی restart بالا می‌رود، و در `logs` دیدی:
```
server error: listen tcp :8080: bind: address already in use
```
یعنی پورت قبلاً اشغال شده — معمولاً توسط یک **کانتینر Docker** (مثلاً نود/پنل رسمی PasarGuard) یا یک نسخه‌ی سرگردان.
```bash
sudo ss -ltnp 'sport = :8080'      # ببین چه چیزی پورت را گرفته (پنل)
sudo ss -ltnp 'sport = :62050'     # نود: gRPC
sudo ss -ltnp 'sport = :8090'      # نود: HTTP کنترل
```
- اگر `docker-proxy` بود: یک کانتینر آن پورت را publish کرده. **پورت سرویس ما را عوض کن** (تداخل ندارد):
  - پنل: `sudo pg-panel edit-env` → `PANEL_HTTP_ADDR=:2095`
  - نود: `sudo pg-node-agent edit-env` → `PG_AGENT_HTTP_ADDR=:8092` و `PG_AGENT_GRPC_ADDR=:62052` (و در پنل، آدرس و «پورت gRPC» نود را مطابقش بگذار).
- اگر یک `pg-panel`/`pg-node-agent` سرگردان بود: `sudo pkill -f /opt/pg-panel/pg-panel` (بعد `systemctl reset-failed` و `restart`).
- نکته: نود رسمی PasarGuard هم gRPC را روی **62050** می‌گذارد؛ برای نصب کنار آن حتماً پورت‌های نود ما را تغییر بده.

## نود در پنل اصلی offline است
- `sudo pg-node-agent status` و `logs` را چک کن.
- آدرس را درست وارد کرده‌ای؟ باید `https://NODE_IP:8090` باشد (نه gRPC).
- فایروال پورت‌های `8090` (کنترل) و `62050` (gRPC) را باز کرده؟
- گواهی pin‌شده با گواهی فعلی نود می‌خورد؟ اگر `renew-cert` زده‌ای، دوباره pin کن.

## پنل گواهی را نمی‌گیرد / خطای TLS موقع افزودن نود
- `cert_pem` را کامل (با خطوط BEGIN/END) بچسبان.
- یا فیلد گواهی را خالی بگذار تا TOFU خودکار انجام شود.
- اگر گواهی نود عوض شده، نود را حذف و دوباره اضافه کن.

## مشتری در پنل PasarGuard وصل شد ولی لینک کاربر کار نمی‌کند
- inbound پنل مشتری باید با inbound واقعی نود **یکی** باشد: پورت، پروتکل، شبکه، SNI/host، TLS. (نگاه کنید: [اتصال مشتری](05-Customer-Connect)).
- `--force-inbounds` نود باید تگ inbound واقعی را داشته باشد (پیش‌فرض `vless-in`).
- کانفیگ نود را با `sudo pg-node-agent edit` ببین و پارامترهای دقیق را به مشتری بده.

## کاربرهای مشتری قطع شده‌اند
- احتمالاً حجم/انقضای اشتراک تمام شده. مصرف: `GET /api/v1/customers/{id}/usage`.
- با `topup-quota` یا `renew` در پنل برگردان. داده‌ها با تمدید برمی‌گردند (suspend ≠ delete).

## مصرف اضافه (overage)
چون Xray مشترک است، ممکن است کمی مصرف بعد از اتمام حجم تا لحظه‌ی enforce ثبت شود. این overage در `GET /customers/{id}/usage` (`overage_bytes`) گزارش می‌شود تا ربات فروش کیف‌پول را تنظیم کند. بازه‌ی enforce پیش‌فرض ۱۰ ثانیه است (`PG_AGENT_ENFORCE_INTERVAL`).

</div>
