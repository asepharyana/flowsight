-- 0010_universe_names.sql: company names for screener display.
ALTER TABLE universe ADD COLUMN company_name TEXT NOT NULL DEFAULT '';