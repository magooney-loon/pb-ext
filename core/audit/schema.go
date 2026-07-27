package audit

// _admin_access lives in auxiliary.db as a plain table, for the same reasons
// _analytics and _alerts do: it is machine-written, append-heavy and has no use
// for the records API, realtime or collection rules.
//
// There is a sharper reason here than for the others. This table is written
// precisely when the server is under attention it did not ask for — a scanner
// sweeping the admin panel, someone working through a password list. Sharing
// data.db's single writer connection would mean that traffic contending with
// the application's own writes at exactly the wrong moment. On the auxiliary
// writer it cannot.
//
// The rows are also, unlike anything else pb-ext stores, personal data: client
// addresses, user agents, and the account names authentication attempts
// targeted. That is the point — an intrusion question is unanswerable without
// them — but it means retention is a feature, not an afterthought, and the
// created index exists to make the daily delete cheap.

// createTableSQL is the admin access log.
//
// One row per distinct event, except that identical events observed inside the
// same flush window collapse into one row with a count and a last_seen. That
// keeps a flood — ten thousand hits on /_/ from one address in ten seconds —
// to a single row that says so, rather than ten thousand rows that bury
// everything else in the table.
//
// The text columns are NOT NULL defaulting to the empty string, matching the
// other pb-ext tables: a NULL would mean every read had to handle a null string
// for no benefit.
const createTableSQL = `
	CREATE TABLE IF NOT EXISTS {{_admin_access}} (
		[[id]]          INTEGER PRIMARY KEY,
		[[created]]     TEXT    DEFAULT (strftime('%Y-%m-%d %H:%M:%fZ')) NOT NULL,
		[[last_seen]]   TEXT    DEFAULT '' NOT NULL,
		[[kind]]        TEXT    DEFAULT '' NOT NULL,
		[[method]]      TEXT    DEFAULT '' NOT NULL,
		[[path]]        TEXT    DEFAULT '' NOT NULL,
		[[query]]       TEXT    DEFAULT '' NOT NULL,
		[[status]]      INTEGER DEFAULT 0  NOT NULL,
		[[outcome]]     TEXT    DEFAULT '' NOT NULL,
		[[auth_state]]  TEXT    DEFAULT '' NOT NULL,
		[[identity]]    TEXT    DEFAULT '' NOT NULL,
		[[ip]]          TEXT    DEFAULT '' NOT NULL,
		[[user_agent]]  TEXT    DEFAULT '' NOT NULL,
		[[referer]]     TEXT    DEFAULT '' NOT NULL,
		[[trace_id]]    TEXT    DEFAULT '' NOT NULL,
		[[duration_ms]] REAL    DEFAULT 0  NOT NULL,
		[[error]]       TEXT    DEFAULT '' NOT NULL,
		[[count]]       INTEGER DEFAULT 1  NOT NULL
	);

	CREATE INDEX IF NOT EXISTS idx_admin_access_created ON {{_admin_access}} ([[created]]);
	CREATE INDEX IF NOT EXISTS idx_admin_access_ip ON {{_admin_access}} ([[ip]], [[kind]], [[created]]);
	CREATE INDEX IF NOT EXISTS idx_admin_access_kind ON {{_admin_access}} ([[kind]], [[created]]);
`

// dropTableSQL reverts createTableSQL. The indexes go with the table.
const dropTableSQL = `DROP TABLE IF EXISTS {{_admin_access}}`
