// Package validator centralises input validation. Handlers must
// delegate to this package; they may not inspect request fields
// themselves beyond reading them off the wire.
package validator

import (
	"errors"
	"fmt"
	"strconv"
)

// MaxGoals is the upper bound on a single team's score in one match.
// Mirrors the cap applied in the Poisson simulator.
const MaxGoals = 6

// Standard error values — callers should errors.Is against these.
var (
	ErrInvalidID     = errors.New("id must be a positive integer")
	ErrInvalidWeek   = errors.New("week must be between 1 and 6")
	ErrInvalidGoals  = errors.New("goals must be between 0 and 6")
	ErrEmptyPayload  = errors.New("request body is empty")
)

// ParsePositiveInt converts a path/query parameter to a positive int.
// Returns ErrInvalidID for any non-positive or non-numeric value.
func ParsePositiveInt(raw string) (int, error) {
	n, err := strconv.Atoi(raw)
	if err != nil || n <= 0 {
		return 0, ErrInvalidID
	}
	return n, nil
}

// ParseWeek converts the :week URL parameter to an int in [1, 6].
func ParseWeek(raw string) (int, error) {
	n, err := strconv.Atoi(raw)
	if err != nil || n < 1 || n > 6 {
		return 0, ErrInvalidWeek
	}
	return n, nil
}

// EditFixturePayload is the validated body of PUT /api/fixtures/:id.
type EditFixturePayload struct {
	HomeGoals int `json:"home_goals"`
	AwayGoals int `json:"away_goals"`
}

// ValidateEditFixture enforces the score bounds and returns a stable
// error message on failure. Handlers map the error to a 400 response.
func ValidateEditFixture(p EditFixturePayload) error {
	if !inGoalsRange(p.HomeGoals) {
		return fmt.Errorf("home_goals: %w", ErrInvalidGoals)
	}
	if !inGoalsRange(p.AwayGoals) {
		return fmt.Errorf("away_goals: %w", ErrInvalidGoals)
	}
	return nil
}

func inGoalsRange(g int) bool {
	return g >= 0 && g <= MaxGoals
}
