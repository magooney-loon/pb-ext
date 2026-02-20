# Analytics System

## Current State vs. Redesign

The current implementation stores one SQLite row per page view (raw event log). This
document describes its problems and the planned redesign to aggregated counters.

---

## Problems With the Current Design

### 1. Unbounded storage growth

One row per request, forever. At 10k req/day:

| Time | Rows | Approx file size |
|---|---|---|
| 1 month | ~300k | ~150MB |
| 1 year | ~3.6M | ~1.8GB |
| 3 years | ~10M | ~5GB |

The 90-day pruning job (`__pbExtAnalyticsClean__`) bounds this, but the table still
holds ~900k rows at steady state. Backups include all of it.

### 2. `GetData()` is expensive

Every dashboard load: fetch up to 50k full records into Go memory, iterate them all in
Go to compute aggregates. ~25MB of allocations per dashboard open. The `COUNT(*)` is a
separate full-table scan regardless of row count.

### 3. Not GDPR-compliant

- `ip` stored in plaintext — personal data under GDPR
- `user_agent` stored in full — fingerprinting vector
- `visitor_id` is a truncated SHA-256 of `ip+ua` — still a pseudonymous identifier
  that must be disclosed

---

## Redesign: Aggregated Counters + Session Ring

Two tables replace the single raw-events table:

### Table 1: `_analytics` — daily aggregated counters

One row per unique `(path, date, device_type, browser)` combination per day.
Updated via upsert on every tracked request. No personal data stored.

**Schema:**

| Field | Type | Notes |
|---|---|---|
| `path` | text, required | URL path (`/about`, `/blog/post-1`) |
| `date` | text, required | ISO date string `YYYY-MM-DD` |
| `device_type` | text | `desktop` / `mobile` / `tablet` |
| `browser` | text | `chrome` / `firefox` / `safari` / `edge` / `opera` / `unknown` |
| `views` | number | Incremented on every hit |
| `unique_sessions` | number | Incremented only on new session (visitor first hit in session window) |
| `created` | autodate | |
| `updated` | autodate | |

**Unique index on `(path, date, device_type, browser)`** — enforces one row per combination,
makes upsert correct.

**What this enables:**
- `SELECT SUM(views), SUM(unique_sessions) FROM _analytics` → total views + sessions
- `SELECT SUM(views) FROM _analytics WHERE date = today` → today's views
- `SELECT path, SUM(views) FROM _analytics GROUP BY path ORDER BY 2 DESC LIMIT 10` → top pages
- `SELECT device_type, SUM(views) FROM _analytics GROUP BY device_type` → device breakdown
- `SELECT browser, SUM(views) FROM _analytics GROUP BY browser` → browser breakdown
- All computed in SQLite, zero Go-side aggregation, tiny result sets

**Row count at steady state:**
Realistic sites have <200 unique paths, 3 device types, 6 browsers. That's
200 × 3 × 6 = 3,600 rows per day max. With 90-day retention: **~324k rows absolute
maximum**, likely far less. Compare to 900k rows in the current design for the same
period at 10k req/day.

### Table 2: `_analytics_sessions` — recent visit ring buffer

Fixed-size table: keeps the last N visits (default: 50). Stores no personal data.
Used only for the "Recent Activity" section of the dashboard.

**Schema:**

| Field | Type | Notes |
|---|---|---|
| `path` | text, required | URL path |
| `device_type` | text | `desktop` / `mobile` / `tablet` |
| `browser` | text | Browser name |
| `os` | text | OS name |
| `timestamp` | date, required | Request timestamp |
| `is_new_session` | bool | True if first hit in session window |
| `created` | autodate | |

**Ring buffer enforcement:** after each insert, delete all rows where `rowid NOT IN
(SELECT rowid FROM _analytics_sessions ORDER BY created DESC LIMIT 50)`. This keeps the
table permanently bounded at 50 rows with no background worker needed.

**No IP, no user_agent, no visitor_id stored.** Device type, browser, and OS are
already-parsed categorical values — not personal data.

---

## GDPR Compliance

The redesign is **privacy-first by design**:

| Data point | Current | Redesign |
|---|---|---|
| IP address | Stored in plaintext | Never stored |
| User agent string | Stored in full | Never stored |
| Visitor ID (hash of ip+ua) | Stored | Never stored |
| Device type / browser / OS | Stored | Stored (categorical, not personal) |
| UTM parameters | Stored | Not stored (removed from scope) |
| Query params | Stored | Not stored (removed from scope) |
| Referrer | Stored | Not stored (removed from scope) |

**What remains:** path, date, device type, browser, OS, timestamp. None of these are
personal data under GDPR. No consent requirement, no data subject rights to handle, no
retention obligation beyond what makes operational sense.

**Caveat:** `path` can contain personal data if your routes embed user IDs
(e.g. `/users/12345/profile`). Sanitize paths before tracking if this applies.

---

## Data Flow (Redesigned)

```
HTTP request
  → middleware
      → shouldExclude(path)?  → skip
      → isBot(ua)?            → skip
      → e.Next()              → handler runs
      → track(request)
          → date = today's date string (YYYY-MM-DD)
          → device_type, browser, os = parseUA(ua)
          → isNew = isNewSession(sessionKey)  ← sessionKey = hash(ip+ua), never stored
          → upsertDailyCounter(path, date, device_type, browser, isNew)
          → insertSessionEntry(path, device_type, browser, os, isNew)
              → insert row
              → DELETE ring overflow (keep only last 50)

GetData():
  → 6 lightweight SQL queries (all aggregate, no row scan in Go)
  → returns Data struct — same shape as today, template unchanged
```

**The in-memory session map (`knownVisitors`) is kept** — it's used only to determine
`isNew` before writing to DB, and its key (hash of ip+ua) is **never persisted**. This
is purely an ephemeral rate-limit / dedup mechanism, not storage of personal data.

---

## Dashboard Data Mapping

All fields currently shown in `visitor_analytics.tmpl` remain available:

| Template field | Source (redesign) |
|---|---|
| `UniqueVisitors` | `SELECT SUM(unique_sessions) FROM _analytics WHERE date >= 90d ago` |
| `NewVisitors` | Tracked via `is_new_session=true` in sessions ring |
| `ReturningVisitors` | `UniqueVisitors - NewVisitors` |
| `TotalPageViews` | `SELECT SUM(views) FROM _analytics` |
| `TodayPageViews` | `SELECT SUM(views) FROM _analytics WHERE date = today` |
| `YesterdayPageViews` | `SELECT SUM(views) FROM _analytics WHERE date = yesterday` |
| `ViewsPerVisitor` | `TotalPageViews / UniqueVisitors` |
| `TopDeviceType` | `SELECT device_type, SUM(views) ... GROUP BY device_type ORDER BY 2 DESC LIMIT 1` |
| `Desktop/Mobile/TabletPercentage` | Same query, all three rows |
| `TopBrowser` + `BrowserBreakdown` | `SELECT browser, SUM(views) ... GROUP BY browser` |
| `TopPages` | `SELECT path, SUM(views) ... GROUP BY path ORDER BY 2 DESC LIMIT 10` |
| `RecentVisits` | `SELECT * FROM _analytics_sessions ORDER BY created DESC LIMIT 3` |
| `RecentVisitCount` | `SELECT COUNT(*) FROM _analytics_sessions WHERE timestamp >= 1h ago` |
| `HourlyActivityPercentage` | `RecentVisitCount / MaxExpectedHourlyVisits * 100` |

**Template requires zero changes.** `analytics.Data` keeps the same struct shape.

---

## Implementation Plan

### Files to change

| File | Change |
|---|---|
| `core/analytics/types.go` | Remove raw `PageView` struct, add `DailyCounter` and `SessionEntry` internal types; keep `Data`, `PageStat`, `RecentVisit`, constants |
| `core/analytics/collection.go` | Replace single collection setup with two: `_analytics` (aggregated) + `_analytics_sessions` (ring) |
| `core/analytics/collector.go` | Replace `track()` → upsert counter + ring insert; remove IP/UA/referrer/UTM/query_params fields; keep `isNewSession()`, `parseUA()`, `shouldExclude()`, `isBot()`, `clientIP()` (still needed for session dedup, just not stored) |
| `core/analytics/storage.go` | Replace `aggregate()` + raw record fetching with 6 SQL queries; remove buffer/flush system entirely — writes are now synchronous upserts |
| `core/analytics/analytics.go` | Remove buffer fields, flush goroutines, `flushChan`; keep `knownVisitors` session map; simplify `Initialize()` |
| `core/analytics/analytics_test.go` | Update tests for new schema and data flow |
| `core/jobs/manager.go` | Update `__pbExtAnalyticsClean__` to delete from `_analytics WHERE date < 90d ago` and truncate `_analytics_sessions` (already bounded, but good hygiene) |
| `DOCS/ANALYTICS.md` | This file — mark implemented when done |

### No buffer, no flush goroutines

The current design buffers writes to avoid per-request DB writes. With aggregated
counters, each request does one `UPDATE _analytics SET views=views+1 WHERE ...` (plus
ring insert) — two writes per request, both tiny. SQLite handles hundreds of writes/sec
easily for this workload. The async flush complexity is eliminated entirely.

### Upsert strategy

SQLite `INSERT OR REPLACE` on the unique index `(path, date, device_type, browser)`:

```sql
INSERT INTO _analytics (path, date, device_type, browser, views, unique_sessions)
VALUES (?, ?, ?, ?, 1, ?)
ON CONFLICT (path, date, device_type, browser)
DO UPDATE SET
  views = views + 1,
  unique_sessions = unique_sessions + excluded.unique_sessions
```

PocketBase exposes `app.DB().NewQuery(sql).Execute()` for raw SQL — use this since
PocketBase's record API doesn't support upserts.

### Ring buffer enforcement

After inserting into `_analytics_sessions`:

```sql
DELETE FROM _analytics_sessions
WHERE rowid NOT IN (
  SELECT rowid FROM _analytics_sessions ORDER BY created DESC LIMIT 50
)
```

---

## What We Drop

| Removed | Reason |
|---|---|
| `ip` field | Personal data — eliminated for GDPR |
| `user_agent` field | Personal data — eliminated for GDPR |
| `visitor_id` field | Pseudonymous identifier — eliminated for GDPR |
| `referrer` field | Not shown in dashboard — not worth storing |
| `utm_*` fields | Not shown in dashboard — not worth storing |
| `query_params` field | Not shown in dashboard — not worth storing |
| `method` field | Not shown in dashboard — not worth storing |
| `duration_ms` field | Not shown in dashboard — not worth storing |
| Async buffer + flush workers | Replaced by synchronous upserts |
| `ForceFlush()` / `flushChan` | No longer needed |
| `GetData()` `time.Sleep(100ms)` | No longer needed |

---

## Success Criteria

- [ ] Template renders identically — no changes to `visitor_analytics.tmpl`
- [ ] `analytics.Data` struct shape unchanged — no changes to `health.go`
- [ ] No IP, UA, visitor ID, referrer, UTM, or query params stored anywhere in DB
- [ ] `_analytics` row count bounded: `unique_paths × 3 devices × 6 browsers × retention_days`
- [ ] `_analytics_sessions` permanently capped at 50 rows
- [ ] `GetData()` uses only SQL aggregation — no Go-side record iteration
- [ ] `go build ./...` and `go test ./...` clean

---

**Last Updated**: 2026-02-20
