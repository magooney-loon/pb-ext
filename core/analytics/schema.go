package analytics

// _analytics lives in auxiliary.db as a plain table rather than in data.db as a
// PocketBase collection. It is the same choice PocketBase makes for _logs: an
// append-heavy, machine-written table has no business sharing the app's single
// writer connection, and nothing here needs the records API, realtime or
// collection rules — every read and write in this package is raw SQL.
//
// The practical effect is that a flush never queues behind an application write
// (and vice versa), and the daily retention DELETE never takes the data.db write
// lock at all.

// createTableSQL is the full _analytics schema: one row per
// (path, date, device_type, browser) with the counters folded into it.
//
// The text columns are NOT NULL, defaulting to the empty string, rather than
// nullable. SQLite treats NULLs as distinct within a unique index, so a single
// NULL device_type would let the same key insert twice and silently split its
// counters instead of upserting onto the existing row.
//
// Beyond idx_analytics_upsert — which the ON CONFLICT target in buildUpsert
// resolves against, and is therefore required for correctness — each dashboard
// aggregate gets a covering index whose leading column is its GROUP BY target
// and which carries every column the query reads. That lets SQLite answer them
// from the index alone: no per-row table lookups, and no temp B-tree for the
// grouping. Writes happen once per flush rather than once per request, so the
// extra index maintenance is a good trade.
const createTableSQL = `
	CREATE TABLE IF NOT EXISTS {{_analytics}} (
		[[path]]               TEXT    DEFAULT '' NOT NULL,
		[[date]]               TEXT    DEFAULT '' NOT NULL,
		[[device_type]]        TEXT    DEFAULT '' NOT NULL,
		[[browser]]            TEXT    DEFAULT '' NOT NULL,
		[[views]]              INTEGER DEFAULT 0  NOT NULL,
		[[unique_sessions]]    INTEGER DEFAULT 0  NOT NULL,
		[[returning_sessions]] INTEGER DEFAULT 0  NOT NULL,
		[[created]]            TEXT    DEFAULT (strftime('%Y-%m-%d %H:%M:%fZ')) NOT NULL,
		[[updated]]            TEXT    DEFAULT (strftime('%Y-%m-%d %H:%M:%fZ')) NOT NULL
	);

	CREATE UNIQUE INDEX IF NOT EXISTS idx_analytics_upsert ON {{_analytics}} ([[path]], [[date]], [[device_type]], [[browser]]);
	CREATE INDEX IF NOT EXISTS idx_analytics_totals ON {{_analytics}} ([[date]], [[views]], [[unique_sessions]], [[returning_sessions]]);
	CREATE INDEX IF NOT EXISTS idx_analytics_pages ON {{_analytics}} ([[path]], [[date]], [[views]]);
	CREATE INDEX IF NOT EXISTS idx_analytics_devices ON {{_analytics}} ([[device_type]], [[date]], [[views]]);
	CREATE INDEX IF NOT EXISTS idx_analytics_browsers ON {{_analytics}} ([[browser]], [[date]], [[views]]);
`

// dropTableSQL reverts createTableSQL. The indexes go with the table.
const dropTableSQL = `DROP TABLE IF EXISTS {{_analytics}}`
