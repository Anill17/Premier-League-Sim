-- =====================================================================
-- 002_seed.sql
-- Fixed FC 25 team ratings.
-- Home advantage derived from real stadium capacities:
--   Arsenal  (Emirates,         60 704) → 8
--   Liverpool (Anfield,         61 276) → 8
--   Man City  (Etihad,          53 400) → 7
--   Chelsea   (Stamford Bridge, 40 343) → 6
-- =====================================================================

INSERT INTO teams (name, attack, defense, midfield, home_advantage, strength)
VALUES
    ('Manchester City', 90, 85, 88, 7, 88),
    ('Arsenal',         84, 80, 83, 8, 82),
    ('Liverpool',       86, 78, 80, 8, 80),
    ('Chelsea',         79, 76, 78, 6, 78)
ON CONFLICT (name) DO NOTHING;
