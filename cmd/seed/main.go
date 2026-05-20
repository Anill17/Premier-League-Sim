package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"

	"github.com/jackc/pgx/v5/pgxpool"
)

type teamAttributes struct {
	Name         string
	Attack       int
	Defense      int
	Midfield     int
	HomeAdvantage int
	Strength     int
}

// teams holds the fixed FC 25 ratings for all four clubs.
// Home advantage is derived from stadium capacity:
//   Arsenal (60 704) and Liverpool (61 276) → 8
//   Manchester City (53 400)               → 7
//   Chelsea (40 343)                       → 6
var teams = []teamAttributes{
	{Name: "Manchester City", Attack: 90, Defense: 85, Midfield: 88, HomeAdvantage: 7, Strength: 88},
	{Name: "Arsenal",         Attack: 84, Defense: 80, Midfield: 83, HomeAdvantage: 8, Strength: 82},
	{Name: "Liverpool",       Attack: 86, Defense: 78, Midfield: 80, HomeAdvantage: 8, Strength: 80},
	{Name: "Chelsea",         Attack: 79, Defense: 76, Midfield: 78, HomeAdvantage: 6, Strength: 78},
}

const upsertSQL = `
	INSERT INTO teams (name, attack, defense, midfield, home_advantage, strength)
	VALUES ($1, $2, $3, $4, $5, $6)
	ON CONFLICT (name)
	DO UPDATE SET
		attack         = EXCLUDED.attack,
		defense        = EXCLUDED.defense,
		midfield       = EXCLUDED.midfield,
		home_advantage = EXCLUDED.home_advantage,
		strength       = EXCLUDED.strength`

func printTable() {
	fmt.Printf("%-20s %7s %7s %8s %7s %9s\n",
		"Team", "Attack", "Defense", "Midfield", "HomeAdv", "Strength")
	fmt.Printf("%-20s %7s %7s %8s %7s %9s\n",
		"--------------------", "------", "-------", "--------", "-------", "--------")
	for _, t := range teams {
		fmt.Printf("%-20s %7d %7d %8d %7d %9d\n",
			t.Name, t.Attack, t.Defense, t.Midfield, t.HomeAdvantage, t.Strength)
	}
}

func main() {
	dryRun := flag.Bool("dry-run", false, "print ratings without inserting into DB")
	flag.Parse()

	printTable()

	if *dryRun {
		fmt.Println("\n[dry-run] skipped DB write.")
		return
	}

	dbURL := os.Getenv("DATABASE_URL")
	if dbURL == "" {
		log.Fatal("DATABASE_URL is required (set in .env or environment)")
	}

	ctx := context.Background()
	pool, err := pgxpool.New(ctx, dbURL)
	if err != nil {
		log.Fatalf("connect to DB: %v", err)
	}
	defer pool.Close()

	for _, t := range teams {
		if _, err := pool.Exec(ctx, upsertSQL,
			t.Name, t.Attack, t.Defense, t.Midfield, t.HomeAdvantage, t.Strength,
		); err != nil {
			log.Fatalf("upsert %s: %v", t.Name, err)
		}
		log.Printf("upserted: %s", t.Name)
	}

	fmt.Println("\nAll 4 teams upserted successfully.")
}
