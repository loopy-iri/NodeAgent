**🇬🇧 English** · [🇮🇷 فارسی](06-Personal-Use)

# Personal use alongside selling (operator)

Want to sell to others and also use the same node yourself like a normal PasarGuard node? Two ways.

## Way 1 (recommended): a personal tenant for yourself
Create a customer/subscription with a **large quota** for yourself and connect your personal PasarGuard panel with that tenant's key. Exactly like a normal customer, just with a high quota.

1. Create a plan (e.g. 10 TB, long duration).
2. Create a "self" customer and a subscription → get the customer key.
3. Connect your personal panel with that key + certificate + `NODE_IP:62050`.

Benefit: fully isolated from the core config, zero risk, works today.

## Way 2: manage the core with the core key
If you want to manage the **Xray core config itself** from a PasarGuard panel:
1. The node generates a core key by default (or install with `--core-key`).
2. In PasarGuard, add the node using the **core key** (instead of a customer key).
3. Now you can push core config (Start applies config), while user/Stop ops from it are ignored so multi-tenancy doesn't break.

> Warning: the core key changes the shared config for all customers. Use carefully. For pure personal usage, **Way 1 is safer**.

## Do my users collide with customers' users?
No. Each tenant has its own namespace (emails are namespaced) and credential uniqueness (uuid/password) is enforced node-wide. Each tenant's data and stats are separate.
