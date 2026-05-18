-- =====================================================================
-- 002_seed.sql
-- The four-team Premier League seed data.
-- Team attributes are sourced once here; no Go code may hardcode them.
-- =====================================================================

INSERT INTO teams (name, attack, defense, midfield, home_advantage, strength)
VALUES
    ('Manchester City', 90, 85, 88, 8, 88),
    ('Arsenal',         84, 80, 83, 7, 82),
    ('Liverpool',       86, 78, 80, 7, 80),
    ('Chelsea',         79, 76, 78, 6, 78)
ON CONFLICT (name) DO NOTHING;
