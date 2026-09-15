-- 0009_universe.sql: persisted full IDX ticker universe from the close sweep.
CREATE TABLE IF NOT EXISTS universe(
  ticker TEXT PRIMARY KEY,
  close INTEGER NOT NULL DEFAULT 0,
  date TEXT NOT NULL DEFAULT ''
);