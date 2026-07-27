# Admin Access Auditing

`core/audit` records access to the administrative surfaces and raises alerts on what looks like an intrusion. **On by default.**

**This is the one place pb-ext stores personal data, and that is deliberate.** Analytics answers "how is the site used" and so keeps no identities at all; this answers "who tried to get into the admin panel", which is unanswerable without the client address, the user agent and the account an attempt targeted. The exception is scoped: `audit.WithPersonalData(ip, userAgent, identity bool)` switches each field off individually, `audit.WithEnabled(false)` turns the whole thing off, and everything is deleted after `RetentionDays` (90 by default) by `__pbExtAuditClean__`. Do not widen what is captured without a reason as specific as these.

**What PocketBase already does, so we don't.** PB's `activityLogger` writes every request into `_logs` (aux db): url, method, status, execTime, referer, userAgent, auth collection, and `userIP`/`remoteIP` when `Settings.Logs.LogIP` is on — it defaults to **true**. Duplicating that would be waste. Four gaps justify this package:

1. **A failed login never records which account was targeted.** The attempted identity is in the request body; PB logs a 400 and nothing more. This is the highest-value field in an intrusion investigation and it exists nowhere else. `OnRecordAuthWithPasswordRequest` is the only place it is observable.
2. `Settings.Logs.LogAuthId` defaults to **false**, so the built-in log says "a superuser" and not which one.
3. `Settings.Logs.MaxDays` defaults to **5**. Intrusions are usually noticed later than that.
4. Admin access in `_logs` is a JSON blob among every other request — no rollup, nothing to alert on.

**The submitted password is never read.** `RecordAuthWithPasswordRequestEvent` carries a `Password` field and nothing in the package touches it. `TestPackage_NeverReadsTheSubmittedPassword` parses the package's AST and fails on any selector named `Password` — a behavioural test cannot prove an absence, a source scan can. Don't defeat it.

**What is captured** (`collector.go`): `/_/_` (pb-ext dashboard), `/_/` (PB admin UI, minus its static assets — the SPA pulls dozens per page view), any `/api/` call carrying superuser auth, anything targeting `_superusers`, plus every superuser auth success and failure. Query strings are stored with `token`/`password`/`secret`/`access_token`/`api_key` values replaced — an audit log is a place people paste into tickets.

**Anonymous dashboard hits are `denied`, not `allowed`.** An unauthenticated GET of `/_/_` renders the login screen and returns 200 (`health.go`), so by status alone every probe would read as success. `outcomeOf` special-cases it; `TestOutcomeOf_AnonymousDashboardHitsAreDenials` pins it.

**Zero database work on the request path**, and it matters more here than elsewhere: this code runs while the server is being scanned and must not become the thing that falls over. Events fold into a bounded map under one short mutex; a worker writes batches every `FlushInterval` (5s). **All detection runs on that worker** — both questions it asks ("how many failures from this address recently", "has this address ever signed in before") are database reads, and asking them from the auth handler would put a query on the login path an attacker is hammering, so the check meant to detect a flood would amplify it.

**Repeats collapse.** Identical events inside one flush window become one row with `count` and `last_seen`. Ten thousand hits on `/_/` from one address in ten seconds is one row that says so, not ten thousand rows burying everything else.

**At the ceiling, only *new* distinct keys are refused.** A flood from one source against one path is a single key, so it keeps being counted exactly — that flood is the reason the ceiling was reached, so losing its count would lose the thing worth knowing. Dropped events are counted and raise their own alert: a security log with a hole in it must say so.

| Alert | Trigger | Level |
|---|---|---|
| Failed superuser login | Any rejected attempt | Warn, keyed per source |
| Repeated failed logins | `BruteForceThreshold` (5) within `BruteForceWindow` (10m), counting rows already on record | Critical |
| Sign-in from a new address | A successful auth from an IP with no prior success in the retained history | Warn |
| Non-superuser on an admin surface | An authenticated non-superuser reaching `/_/`, `/_/_` or a superuser API route | Warn |
| Auditing not recording | The `_admin_access` table is missing | Critical |

A missing table is loud rather than silent: **an empty security log reads as "nothing happened"**, so `Initialize` logs an error, sets `Recording() == false`, alerts, and the dashboard card says "Not recording" instead of showing an empty table.

`hasPriorSuccess` returns **true** on a query error — failing quiet, because a read failure must not manufacture a "new address" alert for somewhere the administrator signs in daily.

**Endpoints**: `GET /api/audit/status`, `/api/audit/recent`, `/api/audit/sources`, all superuser-only and capped at 500 rows, so a stolen superuser token cannot bulk-export the log in one call. Dashboard card lives in the Alerts & Access section (`#alerts`).
