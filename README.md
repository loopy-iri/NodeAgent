# NodeAgent — PasarGuard multi-tenant node (fork)

نودِ **چند-مستأجری** بر پایه‌ی فورک [`PasarGuard/node`](https://github.com/PasarGuard/node). یک **Xray مشترک** با کانفیگ ثابت، چند مشتری (tenant) را با کاربرهایشان سرویس می‌دهد؛ سهمیه‌ی حجم و انقضا را به‌صورت **محلی** enforce می‌کند؛ و یک **API HTTP دو سطحی** (master برای پنل اصلی، tenant برای مشتری) دارد.

> Repo: `https://github.com/loopy-iri/NodeAgent`
> پنلِ کنترل در ریپوی جدا: `https://github.com/loopy-iri/NodePanel`

## ویژگی‌ها
- **Xray مشترک** (سبک) + جداسازی مستأجرها بر اساس کاربر؛ email‌ها per-tenant نِیم‌اسپیس می‌شوند.
- **یکتایی credential** (uuid/password) در کل نود enforce می‌شود (ایزولاسیون امن auth).
- **enforcement محلی**: اتمام حجم/انقضا → کاربرهای مشتری از هسته حذف و کلیدش رد می‌شود (suspend ≠ delete؛ تمدید کاربرها را برمی‌گرداند).
- **TLS سلف‌ساین** (auto-gen) + pin گواهی توسط پنل (یا TOFU خودکار).
- **gRPC سازگار با PasarGuard** (اختیاری، با core key): مدیریت **فقط کانفیگ هسته** از یک پنل بیرونی؛ عملیات کاربر/Stop آن نادیده گرفته می‌شود تا چند-مستأجری نشکند.

## اجرا

```bash
# نصب کامل (دانلود باینری per-arch + Xray + systemd؛ بدون build روی سرور):
sudo bash -c "$(curl -sL https://raw.githubusercontent.com/loopy-iri/NodeAgent/main/scripts/pg-node.sh)" @ install --core-key "$(openssl rand -hex 16)"

# یا از روی clone:
sudo bash scripts/pg-node.sh install
```

دستورهای CLI: `install, update [VER], uninstall, up, down, restart, status, logs, core-update [VER], renew-cert, edit, edit-env, install-script, completion`.

> نصب از **باینری از‌پیش‌ساخته** (GitHub Releases) انجام می‌شود؛ برای ساخت باینری‌ها کافی است یک تگ `v*` push کنی تا workflow ریلیز اجرا شود. اجرا با **systemd** (سرویس `pg-node`) است، بدون Docker و بدون کامپایل روی سرور.

### اجرای محلی (توسعه)
```bash
$env:PG_AGENT_MASTER_KEY="master"; $env:PG_AGENT_FIXED_CONFIG="configs/fixed-config.example.json"
$env:XRAY_EXECUTABLE_PATH="<path>/xray"; go run ./cmd/agent
```

## متغیرهای محیطی

| متغیر | پیش‌فرض | شرح |
|---|---|---|
| `PG_AGENT_HTTP_ADDR` | `:8090` | آدرس API کنترلی (HTTPS) |
| `PG_AGENT_MASTER_KEY` | — (الزامی) | کلید مستر برای پنل اصلی |
| `PG_AGENT_CORE_KEY` | خالی | فعال‌سازی gRPC سازگار PasarGuard (مدیریت هسته) |
| `PG_AGENT_GRPC_ADDR` | `:62050` | آدرس gRPC |
| `PG_AGENT_FORCE_INBOUNDS` | `vless-in` | کاربرهای هر مشتری روی این inbound tag(ها) اعمال شوند (باید با تگ کانفیگ ثابت بخورد) |
| `PG_AGENT_TENANT_DB` | `tenants.bolt` | مسیر store محلی |
| `PG_AGENT_FIXED_CONFIG` | — | کانفیگ ثابت Xray |
| `SSL_CERT_FILE`/`SSL_KEY_FILE` | `/var/lib/pg-node/certs/...` | گواهی (auto-gen اگر نبود) |

## اتصال مشتری از پنل PasarGuard

مشتری (که پنل PasarGuard خودش را دارد) نود تو را مثل یک **نود معمولی** اضافه می‌کند:
- **Address/Port:** آی‌پی نود + پورت gRPC (پیش‌فرض `62050`).
- **Protocol:** gRPC.
- **Certificate:** گواهی نود (همان PEM که موقع نصب چاپ شد / در پنل اصلی نمایش داده می‌شود) — برای pin.
- **API Key:** **کلید مشتری** که پنل اصلیِ تو هنگام ساخت اشتراک تولید کرده (نه master/core key).

سپس:
- کاربرهای پنل PasarGuard مشتری (vless/vmess/...) روی نود زیر همان tenant اضافه می‌شوند و در پنل خودش دیده می‌شوند.
- چون نود **حجمی** است، کانفیگِ هسته‌ای که PasarGuard می‌فرستد **نادیده** گرفته می‌شود (نود متصل می‌ماند و خطا نمی‌دهد) و فقط **کاربرها** اعمال می‌شوند.
- با `PG_AGENT_FORCE_INBOUNDS` کاربرها همیشه روی inbound واقعی نود می‌نشینند، فارغ از تگ inboundِ پنل مشتری.
- با اتمام حجم/انقضا، کاربرهای آن مشتری حذف و دسترسی کلیدش رد می‌شود.

> **استفاده‌ی شخصی + فروش هم‌زمان:** برای خودت یک tenant با سهمیه‌ی بزرگ بساز و پنل PasarGuard شخصی‌ات را با کلید همان tenant وصل کن — همان نود مشترک را مثل یک نود معمولی استفاده می‌کنی، در کنار فروش به بقیه. کانفیگ هسته را هم با core key (یا ویرایشگر پنل اصلی) کنترل می‌کنی.

### گزینه‌های نصب اضافه
- `--name NAME` نام نمونه (مسیر/سرویس/CLI)؛ پیش‌فرض `pg-node-agent` تا کنار نود رسمی نصب شود.
- `--core-key KEY` فعال‌سازی gRPC مدیریت هسته.
- `--force-inbounds CSV` تگ inboundها (پیش‌فرض `vless-in`).

## API (HTTP)
- **master** (`X-API-Key: <master>`): `POST /admin/config`, `POST/GET/DELETE /admin/tenants`, `PATCH .../quota`, `.../suspend|resume|reset`, `.../usage`.
- **tenant** (`X-API-Key: <customer>`): `PUT /tenant/users`, `GET /tenant/me`.

## ساختار
```
cmd/agent/            entrypoint چند-مستأجری
tenant/               Registry + auth دو سطحی + enforcement (bbolt)
shared/               مدیر هسته‌ی مشترک (یک Xray، add/remove per-tenant)
controller/agent/     API HTTP (master/tenant) + حلقه‌ی enforcement
controller/grpccompat/ gRPC سازگار با PasarGuard (مدیریت هسته)
backend/, common/, ... کد پایه‌ی upstream
```

این یک فورک است؛ مجوز اصلی در `LICENSE` حفظ شده است.
