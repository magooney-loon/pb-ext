# Analytics

Request middleware counts page views as daily aggregates. **No personal data is persisted** — no IP, user agent, or visitor ID ever reaches the database.

**The request path does zero database work.** A tracked view folds into an in-memory counter map, a recent-visit ring, and a minute bucket under one short mutex (~500ns, 1 alloc). A background goroutine writes accumulated deltas every `FlushInterval` (default 10s) in a single batched transaction. Sustained: ~1.8M views/sec with zero loss.

Never add per-request database work to `Track` — that is the exact bottleneck this design exists to remove.

**Counters live in `auxiliary.db`, not `data.db`** — the same split PocketBase uses for `_logs`, and for the same reason. Each database has its own writer connection (`NonconcurrentDB()` and `AuxNonconcurrentDB()` are both capped at 1) and its own WAL, so a flush can neither block nor be blocked by an application write, however many rows it carries. Use `AuxRunInTransaction` + `AuxNonconcurrentDB()` to write and `AuxDB()` to read; `TestFlush_DoesNotWaitOnTheDataDBWriter` holds the data.db writer open and fails if a flush waits on it. The daily retention DELETE (`__pbExtAnalyticsClean__` in `core/jobs/manager.go`) runs on the aux writer for the same reason. Backups still cover it — PocketBase archives the whole data dir and checkpoints the aux WAL first.

**`_analytics` is a plain SQLite table, not a PocketBase collection.** That follows from living in `auxiliary.db`, and costs nothing: every read and write in the package is raw SQL, and nothing needs the records API, realtime or collection rules. The DDL is in `schema.go`; `migrations.go` applies it. One row per `(path, date, device_type, browser)` with `views`, `unique_sessions` (sessions started by a new visitor) and `returning_sessions` counters, deleted after 90 days.

The text columns are `NOT NULL DEFAULT ''`. SQLite treats NULLs as distinct inside a unique index, so one NULL `device_type` would let the same key insert twice and silently split its counters instead of upserting — `idx_analytics_upsert` is what the `ON CONFLICT` target in `buildUpsert` resolves against, so it is required for correctness, not just speed.

**What is filtered out** (`collector.go`): non-GET methods, non-2xx/3xx responses, bot user agents, static assets, and the `/api/`, `/_/`, `/_app/immutable/`, `/.well-known/` prefixes.

**Everything is bounded** — these limits are the defense against junk-URL floods and forged client identities:

| Bound | Default | Effect when exceeded |
|---|---|---|
| `MaxDistinctPaths` | 5000/day | Extra paths collapse into the `/*` bucket; views are still counted |
| `MaxPathLength` | 255 | Longer paths go straight to `/*` |
| `VisitorGenerations` × `MaxVisitorsPerGeneration` | 4 × 25000 | Oldest generation is dropped wholesale |
| `MaxPendingCounters` | 10000 | Triggers an early flush (never drops a view) |

Configure with the `With*` options passed to `analytics.Initialize`.

**Visitor tracking** (`visitors.go`) is memory-only, keyed by a non-reversible FNV-1a hash of `(RealIP, user agent)`. Entries live in rotating generations, giving a hard memory ceiling and O(1) amortized cost — no unbounded map, no full scan under lock. A visitor is remembered for up to `VisitorGenerations × SessionWindow` (default 2h); returning after that reads as new. `RealIP()` honours PocketBase's admin-configured `TrustedProxy` settings, so `X-Forwarded-For` is only trusted when a proxy is configured.

**Dashboard** (`storage.go`): four `GROUP BY` aggregates bounded to `LookbackDays`, each served by a covering index (`idx_analytics_totals`, `_pages`, `_devices`, `_browsers`) so SQLite never does per-row table lookups. Results are memoized for `CacheTTL` (5s); recent visits and hourly activity come live from memory. ~29ms at 50k rows, ~300ns cached. If you change a dashboard query, keep it covered — `TestGetData_QueriesUseCoveringIndexes` asserts this.

**Dashboard templates** (`core/server/templates/`) render into a buffer before anything is written, and the `/_/_` route is wrapped in `recoverDashboardPanic`, so a bad metric degrades to a 500 for that one request instead of unwinding the handler — this does not depend on the embedding app calling `logging.SetupRecovery`. Three traps, all covered by `core/server/dashboard_render_test.go`, which renders the whole page against healthy, sensorless, nil-slice and all-zero data and asserts no panic and no `NaN`/`+Inf`:

- **`isset` on a struct.** It handles slices, maps and (now) structs; the original had no struct case and silently returned `false`, which is why the CPU Details card read `N/C` while System Metrics showed a real temperature from the same data. A guard that always fails closed is worse than no guard — prefer testing the value (`gt $x.Temperature 0.0`) over probing for field existence.
- **`{{with}}` rebinds the dot.** `{{with $t := getSystemTemp .SystemStats}}` makes `.` a `float64` inside the block, so `.SystemStats` there is a template execution error — and `with` skips the block entirely when the value is zero. Use a plain assignment plus `{{if}}`. This one only fired on hosts that actually have a board sensor.
- **Zero denominators.** `divide`/`divideFloat64`/`percentOf`/`errorRate`/`avgCPUUsage`/`requestRate` all return 0 rather than `NaN`/`+Inf`; keep it that way, and note `divide` only accepts `float64`/`uint64` — passing an `int` silently yields 0.

**Analytics template** (`core/server/templates/components/visitor_analytics.tmpl`): the only consumer of `analytics.Data` — there is no JSON endpoint for it, so display limits belong in the template, not in `GetData`. `RecentVisits` carries the full 50-entry ring but the card renders 8; `TopPages` carries 10 and renders 5. Template funcs live in `templateFuncs` (`health.go`); `pathLabel` renders the `/*` overflow bucket as "other pages". `core/server/templates_test.go` parses every template and renders this one against populated and empty data, which is the only place a missing template func gets caught (at runtime it is merely logged).

**Testing**: `testutil.NewTestApp(t)` already has both pb-ext schemas (migrations run automatically). `testutil.NewAnalytics(t, opts...)` returns an app plus a running collector (closed on cleanup); `testutil.AnalyticsTotals(t, app)` reads persisted counters — from `AuxDB()`, like everything else that touches the table. Pass `WithFlushInterval(time.Hour)` to observe buffering and flush by hand. Internals (`aggregator`, `visitorTracker`) are tested white-box in package `analytics`; anything needing `testutil` must live in package `analytics_test` to avoid an import cycle.
