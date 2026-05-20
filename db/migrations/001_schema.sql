-- =====================================================================
-- 001_schema.sql
-- Football League Simulation API — initial schema
-- =====================================================================

CREATE TABLE IF NOT EXISTS teams (
    id              SERIAL PRIMARY KEY,
    name            VARCHAR(100) NOT NULL UNIQUE,
    attack          INT NOT NULL CHECK (attack BETWEEN 1 AND 100),
    defense         INT NOT NULL CHECK (defense BETWEEN 1 AND 100),
    midfield        INT NOT NULL CHECK (midfield BETWEEN 1 AND 100),
    home_advantage  INT NOT NULL CHECK (home_advantage BETWEEN 1 AND 10),
    strength        INT NOT NULL CHECK (strength BETWEEN 1 AND 100)
);

CREATE TABLE IF NOT EXISTS seasons (
    id              SERIAL PRIMARY KEY,
    current_week    INT NOT NULL DEFAULT 0,
    is_complete     BOOLEAN NOT NULL DEFAULT FALSE,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS fixtures (
    id              SERIAL PRIMARY KEY,
    season_id       INT NOT NULL REFERENCES seasons(id) ON DELETE CASCADE,
    week            INT NOT NULL CHECK (week BETWEEN 1 AND 6),
    home_team_id    INT NOT NULL REFERENCES teams(id),
    away_team_id    INT NOT NULL REFERENCES teams(id),
    home_goals      INT,
    away_goals      INT,
    played          BOOLEAN NOT NULL DEFAULT FALSE,
    played_at       TIMESTAMPTZ,
    CONSTRAINT no_self_play CHECK (home_team_id <> away_team_id)
);

CREATE TABLE IF NOT EXISTS standings (
    id              SERIAL PRIMARY KEY,
    season_id       INT NOT NULL REFERENCES seasons(id) ON DELETE CASCADE,
    team_id         INT NOT NULL REFERENCES teams(id),
    played          INT NOT NULL DEFAULT 0,
    won             INT NOT NULL DEFAULT 0,
    drawn           INT NOT NULL DEFAULT 0,
    lost            INT NOT NULL DEFAULT 0,
    goals_for       INT NOT NULL DEFAULT 0,
    goals_against   INT NOT NULL DEFAULT 0,
    goal_difference INT GENERATED ALWAYS AS (goals_for - goals_against) STORED,
    points          INT NOT NULL DEFAULT 0,
    UNIQUE (season_id, team_id)
);

CREATE TABLE IF NOT EXISTS team_form (
    id              SERIAL PRIMARY KEY,
    season_id       INT NOT NULL REFERENCES seasons(id) ON DELETE CASCADE,
    team_id         INT NOT NULL REFERENCES teams(id),
    recent_results  JSONB NOT NULL DEFAULT '[]',
    UNIQUE (season_id, team_id)
);

CREATE TABLE IF NOT EXISTS predictions (
    id                       SERIAL PRIMARY KEY,
    season_id                INT NOT NULL REFERENCES seasons(id) ON DELETE CASCADE,
    week                     INT NOT NULL,
    team_id                  INT NOT NULL REFERENCES teams(id),
    championship_probability FLOAT NOT NULL,
    created_at               TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_fixtures_season_week    ON fixtures(season_id, week);
CREATE INDEX IF NOT EXISTS idx_standings_season        ON standings(season_id);
CREATE INDEX IF NOT EXISTS idx_predictions_season_week ON predictions(season_id, week);

-- =====================================================================
-- Fixed FC 25 team ratings. Home advantage derived from stadium capacity:
--   Arsenal (60 704) and Liverpool (61 276) → 8
--   Manchester City (53 400)               → 7
--   Chelsea (40 343)                       → 6
-- =====================================================================
INSERT INTO teams (name, attack, defense, midfield, home_advantage, strength)
VALUES
    ('Manchester City', 90, 85, 88, 7, 88),
    ('Arsenal',         84, 80, 83, 8, 82),
    ('Liverpool',       86, 78, 80, 8, 80),
    ('Chelsea',         79, 76, 78, 6, 78)
ON CONFLICT (name) DO NOTHING;
