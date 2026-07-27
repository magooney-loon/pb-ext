# Alerts

Operational notifications — crashes, failed cron jobs, recovered panics, traffic and error spikes, plus anything the app wants to report — delivered to a chat transport. Telegram is the only one built in; `Transport` exists so another is a file rather than a redesign. Design rationale and the full failure-mode table live in `TELEGRAM.md`.

**`core/alerts` imports nothing from pb-ext** — only the stdlib and PocketBase. That is forced: `core/server`, `core/jobs` and `core/logging` all emit alerts, and `logging → server → analytics/jobs` already exists, so any dependency the other way is a cycle. System metrics reach the rules through a `MetricsFunc` the server supplies (`Server.metricsSnapshot`), not through an import of `core/monitoring`.

**`Send` does no I/O and never blocks.** A cooldown check (one map lookup under a short mutex) then a non-blocking channel send; a single background worker owns every HTTP call and every database write. Alerts are emitted from request handlers and from panic recovery, so an unreachable Telegram must cost a request nothing. A full queue **drops and counts** rather than blocking — messages are already deduplicated, so 256 queued copies of "CPU high" carry no more information than one.

**Nothing here can break the server.** `Initialize` returns no error: a missing token, a revoked bot, a network partition, a missing `_alerts` table and a full queue are all ordinary states with defined behaviour. `alerts.Get()` never returns nil, so application code calls `alerts.Get().Send(...)` unguarded.

**Crash detection happens on the way back up, not on the way down** (`crash.go`). An OOM kill, a `log.Fatal` or a panic on an unrecovered goroutine ends the process in microseconds; a Telegram delivery needs hundreds of milliseconds, so sending from a dying process buys a hung exit and no message. Instead each run writes `pb_data/.pbext_lastrun.json` with `state: running`, refreshed on the evaluator's heartbeat and set to `stopped` by `OnTerminate` (including on a restart — a dev-mode reload is not an incident). A marker still reading `running` at boot means the previous process never reached its shutdown hook, and the heartbeat timestamp says roughly when it stopped. **A missing, unreadable or corrupt marker produces no alert** — a read-only data directory would otherwise claim a crash at every boot, and an integration that cries wolf gets muted within a day. Fail closed on false alarms.

**Resource saturation is watched out of the box; traffic thresholds are not.** The split is deliberate and the reasoning is not interchangeable. A request rate has no universal danger zone — 50/s is idle for one deployment and an incident for another — so a shipped default would be either silent or deafening. Saturation is not like that: a disk at 95% is about to stop SQLite writing on every machine there has ever been, and memory, swap and descriptor exhaustion are similarly absolute. Shipping those off by default means the common case is a server that looks monitored and says nothing while it fills up.

| Rule | Default | Notes |
|---|---|---|
| `disk_high` | **90%** | Earliest warning of the lot — a full disk stops writes and does not recover on its own. Uses `DiskUsagePercent` (`Used/(Used+Free)`), never `Used/Total` |
| `memory_high` | **90%** | `MemoryInfo.UsedPercent` excludes page cache, so 90% here is real pressure, not a warm cache |
| `swap_high` | **80%** | Skipped entirely when `SwapTotal == 0` — a host with no swap reports 0%, which is not a measurement |
| `cpu_high` | **90%** | Needs `SustainTicks` behind it; a batch job pinning the cores for one tick is working as intended |
| `open_files_high` | **80%** | Ratio against the soft `RLIMIT_NOFILE`. Skipped when the limit is 0 (unknown) — the raw count is meaningless, since the ceiling is 1024 on some hosts and 1048576 on others |
| `error_rate` | off | Opt-in via `WithErrorRateAlert` |
| `traffic_surge` | off | Opt-in via `WithTrafficSurgeAlert` |
| `goroutines_high` | off | Opt-in: a healthy busy server legitimately runs thousands, and a leak is unbounded growth rather than any particular number |

Tune one dimension with `WithDiskAlert(95)` etc., or drop the lot with `WithoutResourceAlerts()`. Passing 0 to any of them disables that rule.

**Rules are edge-triggered, not level-triggered** (`rules.go`). A rule that becomes true fires once and stays quiet until it becomes false, which can send a recovery notice. "CPU above 90%" evaluated every 30s without a state machine is 120 identical messages an hour. `Sustain` requires N consecutive breaches; a panicking rule is disabled after 3 panics rather than taking out the evaluator.

**Rates are differentiated from monotonic counters** inside the evaluator, never read from a pre-computed field. `monitoring.RequestStats.requestRate` is only recalculated when a request arrives, so after traffic stops it reports the last busy figure forever — precisely when a rate matters. `RequestStats.Totals()` supplies the monotonic counters; 4xx and 5xx are tracked separately because alerting on their sum fires on any bot sweeping for `/wp-admin`.

**Telegram specifics** (`telegram.go`, `format.go`):

- **HTML parse mode, never MarkdownV2.** MarkdownV2 requires escaping every one of ``_*[]()~`>#+-=|{}.!`` — which is exactly what alert bodies are made of (paths, stack traces, Go error strings, "1.2s"). One missed escape is a 400, so the message that silently fails is the interesting one. HTML needs three characters escaped.
- **The token is a URL path segment**, so every `net/http` error quotes it. Every error leaving the transport goes through `scrub()`; `TestTelegram_NeverLeaksTheTokenInErrors` and `TestNotifier_NeverLeaksTheTokenIntoStats` are what keep it out of logs, `Stats` and the delivery log.
- **The 4096 limit is in UTF-16 code units.** Counting runes or bytes misjudges it in opposite directions, and every message starts with an emoji (2 units). The header is rendered first and the body gets the remaining budget, so a 40KB stack trace can never push the title out.
- **429 is the one 4xx that means "later, not never"** — its `retry_after` is honoured. 401/403/400/404 are permanent: they are logged once, flag `Misconfigured`, and stop the retry loop rather than hot-looping against someone else's API.

**Bounds** — same discipline as analytics:

| Bound | Default | Effect when exceeded |
|---|---|---|
| `QueueSize` | 256 | Message dropped and counted; reported in the next digest |
| `Cooldown` (per `Key`) | 15m | Suppressed and counted; an empty `Key` is never suppressed |
| `MaxAlertsPerHour` | 20 | Overflow held back, summarised in an hourly digest |
| `maxCooldownKeys` | 1000 | Expired entries pruned, then the map is cleared |
| `MaxRetries` | 3 | Backoff 2s/8s/30s, then recorded as failed |
| `DrainTimeout` | 5s | Shutdown stops waiting; a wedged transport never holds the process open |
| stored body | 2000 runes | Truncated in the delivery log; the full trace is in the app log |

**Privacy**: alert bodies carry path, method, status, job id, error text and host. They do **not** carry client IP, user agent, request body or record contents — the same promise `core/analytics` makes. Don't widen the panic alert in `logging/error_handler.go`.

**Configuration**: `server.WithAlerts(alerts.With*...)`, falling back to `PBEXT_TELEGRAM_BOT_TOKEN` / `PBEXT_TELEGRAM_CHAT_ID`; `PBEXT_ALERTS_ENABLED=false` is a kill switch that outranks everything. **Disabled in developer mode by default** — pb-cli restarts on every file save, and each save would otherwise fire a shutdown and a startup notice.

**Storage**: `_alerts` in `auxiliary.db`, plain SQL, same reasoning as `_analytics` — an alert about a slow database must not queue behind the writes it is reporting on. Retention 30 days via `__pbExtAlertsClean__`. Delivery is low-volume, so one INSERT per delivery needs no batching.

**Dashboard**: a card in the Alerts & Access section (`components/alerts_status.tmpl`) plus `GET /api/alerts/status`, `GET /api/alerts/recent` and `POST /api/alerts/test`, all superuser-only. The test endpoint bypasses the queue deliberately — the point of a test button is to learn whether delivery works *right now* — and is rate-limited to one per minute so it cannot pump the bot into a flood limit. Note the dashboard renders with `text/template`, which escapes nothing: anything originating outside this repository must go through the `escapeHTML` template func.

**Testing**: `testutil.NewAlerts(t, opts...)` returns an app, a running notifier and an `AlertCapture` transport. `alerts.WithAPIBaseURL` points the real Telegram transport at an `httptest` server, so no test touches the network. Delivery is asynchronous — assert with `capture.Wait(t)` or poll `Stats()`, never immediately after `Send`. Tests needing `testutil` must live in package `alerts_test` (testutil → jobs → alerts would otherwise cycle).
