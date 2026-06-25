# ۲) نصب نود (اپراتور)

روی هر سروری که می‌خواهی بفروشی، یک نود نصب کن.

## نصب یک‌خطی
```bash
sudo bash -c "$(curl -sL https://raw.githubusercontent.com/loopy-iri/NodeAgent/main/scripts/pg-node.sh)" @ install
```

اگر می‌خواهی هسته را هم از یک پنل PasarGuard مدیریت کنی (مثلاً برای استفاده‌ی شخصی)، با core key نصب کن:
```bash
sudo bash -c "$(curl -sL https://raw.githubusercontent.com/loopy-iri/NodeAgent/main/scripts/pg-node.sh)" @ install --core-key "$(openssl rand -hex 32)"
```

در پایان این‌ها چاپ می‌شود — **یادداشت کن** (مخصوصاً master key و گواهی):
```
Address (HTTPS control):  https://SERVER_IP:8090
gRPC (PasarGuard-compat): SERVER_IP:62050
Master key (register in main panel):
    <MASTER_KEY>
Core key (PasarGuard core management):   # فقط اگر --core-key دادی
    <CORE_KEY>
Node certificate (paste into the main panel when adding the node):
-----BEGIN CERTIFICATE-----
... (گواهی سلف‌ساین نود) ...
-----END CERTIFICATE-----
```

## گزینه‌ها
| گزینه | پیش‌فرض | توضیح |
|------|---------|-------|
| `--master-key KEY` | تصادفی | کلید مدیریت کامل (برای ثبت در پنل اصلی) |
| `--core-key KEY` | خالی | فعال‌سازی gRPC سازگار PasarGuard برای مدیریت هسته |
| `--http-port PORT` | `8090` | پورت API کنترلی (HTTPS) |
| `--grpc-port PORT` | `62050` | پورت gRPC |
| `--force-inbounds CSV` | `vless-in` | کاربرهای هر مشتری روی این inbound tag(ها) اعمال شوند (باید با تگ کانفیگ ثابت بخورد) |
| `--san-entries CSV` | — | SANهای اضافی گواهی، مثلاً `DNS:node.example.com` |
| `--version vX.Y.Z` | latest | نصب نسخه‌ی مشخص |
| `--name NAME` | `pg-node-agent` | نام نمونه — برای نصب **کنار نود رسمی PasarGuard** |

## کانفیگ هسته (Xray)
یک کانفیگ ثابتِ پیش‌فرض ساخته می‌شود: `/var/lib/pg-node-agent/fixed-config.json` با یک inbound به نام `vless-in` روی پورت `443`. این کانفیگ برای همه‌ی مشتری‌ها مشترک است.
- ویرایش محلی: `sudo pg-node-agent edit` (سپس `restart`).
- یا از پنل اصلی push کن: `PUT /api/v1/nodes/{id}/config`.

> 📌 **راهنمای کامل تنظیم هسته (نمونه‌ی Reality/WS، تولید کلید، چه چیزی به مشتری بده):** [تنظیم هسته‌ی نود](10-Configure-Core).

> **مهم:** هر inbound در این کانفیگ که می‌خواهی کاربرهای مشتری رویش بنشینند باید تگش در `--force-inbounds` باشد و تنظیماتش (port/SNI/شبکه) را به مشتری بدهی تا لینک‌های کاربر نهایی کار کنند.

## دستورهای مدیریتی
```bash
sudo pg-node-agent status
sudo pg-node-agent logs
sudo pg-node-agent restart
sudo pg-node-agent edit         # ویرایش کانفیگ هسته
sudo pg-node-agent edit-env     # ویرایش متغیرها
sudo pg-node-agent renew-cert   # ساخت دوباره‌ی گواهی (باید در پنل دوباره pin شود)
sudo pg-node-agent core-update  # آپدیت Xray-core
sudo pg-node-agent update       # آپدیت باینری نود
```

## مرحله‌ی بعد
[افزودن نود به پنل و فروش](04-Add-Node-And-Sell).
