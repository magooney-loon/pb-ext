package alerts

// _alerts lives in auxiliary.db as a plain table rather than in data.db as a
// PocketBase collection, for the same reasons _analytics and PocketBase's own
// _logs do: it is machine-written, append-only and disposable, and nothing here
// needs the records API, realtime or collection rules — every read and write in
// this package is raw SQL.
//
// The practical effect is that writing a delivery record takes the auxiliary
// writer lock, never the application's. An alert about the database being slow
// must not have to queue behind the thing it is reporting on.

// createTableSQL is the delivery log: one row per attempted delivery, carrying
// the outcome.
//
// The text columns are NOT NULL defaulting to the empty string, matching
// _analytics — a NULL here would mean every read had to handle a null string
// for no benefit.
//
// The column is alert_key rather than key: KEY is a keyword in enough SQL
// dialects that the bracket-quoted form would be the only way to reference it,
// and a column you cannot mention unquoted is a trap for the next query.
const createTableSQL = `
	CREATE TABLE IF NOT EXISTS {{_alerts}} (
		[[id]]        INTEGER PRIMARY KEY,
		[[created]]   TEXT    DEFAULT (strftime('%Y-%m-%d %H:%M:%fZ')) NOT NULL,
		[[level]]     TEXT    DEFAULT '' NOT NULL,
		[[alert_key]] TEXT    DEFAULT '' NOT NULL,
		[[title]]     TEXT    DEFAULT '' NOT NULL,
		[[text]]      TEXT    DEFAULT '' NOT NULL,
		[[transport]] TEXT    DEFAULT '' NOT NULL,
		[[status]]    TEXT    DEFAULT '' NOT NULL,
		[[attempts]]  INTEGER DEFAULT 0  NOT NULL,
		[[error]]     TEXT    DEFAULT '' NOT NULL
	);

	CREATE INDEX IF NOT EXISTS idx_alerts_created ON {{_alerts}} ([[created]]);
`

// dropTableSQL reverts createTableSQL. The index goes with the table.
const dropTableSQL = `DROP TABLE IF EXISTS {{_alerts}}`
