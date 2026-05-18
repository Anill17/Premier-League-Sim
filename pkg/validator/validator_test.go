package validator

import (
	"errors"
	"testing"
)

func TestParsePositiveInt(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name   string
		input  string
		want   int
		wantOK bool
	}{
		{"positive", "42", 42, true},
		{"one", "1", 1, true},
		{"zero", "0", 0, false},
		{"negative", "-3", 0, false},
		{"non_numeric", "abc", 0, false},
		{"empty", "", 0, false},
	}

	for _, c := range cases {
		c := c
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			got, err := ParsePositiveInt(c.input)
			if c.wantOK {
				if err != nil {
					t.Fatalf("unexpected error: %v", err)
				}
				if got != c.want {
					t.Fatalf("got %d, want %d", got, c.want)
				}
				return
			}
			if !errors.Is(err, ErrInvalidID) {
				t.Fatalf("got err %v, want ErrInvalidID", err)
			}
		})
	}
}

func TestParseWeek(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name   string
		input  string
		want   int
		wantOK bool
	}{
		{"valid_1", "1", 1, true},
		{"valid_6", "6", 6, true},
		{"valid_3", "3", 3, true},
		{"zero", "0", 0, false},
		{"seven", "7", 0, false},
		{"negative", "-1", 0, false},
		{"non_numeric", "x", 0, false},
	}

	for _, c := range cases {
		c := c
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			got, err := ParseWeek(c.input)
			if c.wantOK {
				if err != nil {
					t.Fatalf("unexpected error: %v", err)
				}
				if got != c.want {
					t.Fatalf("got %d, want %d", got, c.want)
				}
				return
			}
			if !errors.Is(err, ErrInvalidWeek) {
				t.Fatalf("got err %v, want ErrInvalidWeek", err)
			}
		})
	}
}

func TestValidateEditFixture(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name    string
		payload EditFixturePayload
		wantErr bool
	}{
		{"both_zero", EditFixturePayload{0, 0}, false},
		{"max_max", EditFixturePayload{MaxGoals, MaxGoals}, false},
		{"normal", EditFixturePayload{3, 2}, false},
		{"home_too_high", EditFixturePayload{MaxGoals + 1, 0}, true},
		{"away_too_high", EditFixturePayload{0, MaxGoals + 1}, true},
		{"home_negative", EditFixturePayload{-1, 0}, true},
		{"away_negative", EditFixturePayload{0, -1}, true},
	}

	for _, c := range cases {
		c := c
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			err := ValidateEditFixture(c.payload)
			if c.wantErr {
				if err == nil {
					t.Fatalf("expected error for %+v", c.payload)
				}
				if !errors.Is(err, ErrInvalidGoals) {
					t.Fatalf("expected ErrInvalidGoals, got %v", err)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error for %+v: %v", c.payload, err)
			}
		})
	}
}
