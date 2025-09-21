CREATE TABLE IF NOT EXISTS user_data (
    id SERIAL PRIMARY KEY,
    username TEXT NOT NULL UNIQUE,
    user_slug TEXT NOT NULL,
    user_avatar TEXT,
    country_code CHAR(2),
    country_name TEXT,
    real_name TEXT,
    typename TEXT,
    total_problems_solved INT NOT NULL DEFAULT 0,
    total_submissions INT NOT NULL DEFAULT 0,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS staging_user_data (
    username TEXT NOT NULL UNIQUE,
    user_slug TEXT NOT NULL,
    user_avatar TEXT,
    country_code CHAR(2),
    country_name TEXT,
    real_name TEXT,
    typename TEXT,
    total_problems_solved INT NOT NULL DEFAULT 0,
    total_submissions INT NOT NULL DEFAULT 0
);

-- keep updated_at fresh automatically
CREATE OR REPLACE FUNCTION set_updated_at()
RETURNS TRIGGER AS $$
BEGIN
   NEW.updated_at = NOW();
   RETURN NEW;
END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER trg_user_data_updated
BEFORE UPDATE ON user_data
FOR EACH ROW
EXECUTE FUNCTION set_updated_at();

-- helpful indexes
CREATE INDEX idx_user_data_country ON user_data(country_code);
CREATE INDEX idx_user_data_solved ON user_data(total_problems_solved DESC);
