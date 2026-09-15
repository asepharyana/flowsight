-- FlowSight schema v6: cached LLM narration for briefings.
-- narasi = 3 kalimat santai Indonesia hasil poles LLM (OmniRoute),
-- best-effort: kosong berarti FE pakai ringkasan rules.
ALTER TABLE briefings ADD COLUMN narasi TEXT NOT NULL DEFAULT '';
