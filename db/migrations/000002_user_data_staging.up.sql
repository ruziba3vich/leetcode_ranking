
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

