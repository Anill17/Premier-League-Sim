package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"math"
	"net/http"
	"os"
	"sort"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

const dropAPIBase = "https://drop-api.ea.com/rating/fc-25"

var teamEAIDs = map[string]int{
	"Manchester City": 11,
	"Arsenal":         1,
	"Liverpool":       10,
	"Chelsea":         5,
}

var stadiumCapacity = map[string]int{
	"Manchester City": 53400,
	"Arsenal":         60704,
	"Liverpool":       61276,
	"Chelsea":         40343,
}

type apiResponse struct {
	Items []apiPlayer `json:"items"`
}

// apiPlayer maps the EA FC 25 drop-API player object.
// Field names match the JSON keys returned by drop-api.ea.com/rating/fc-25.
type apiPlayer struct {
	OverallRating int `json:"overallRating"`
	Pace          int `json:"pac"`
	Shooting      int `json:"sho"`
	Passing       int `json:"pas"`
	Dribbling     int `json:"dri"`
	Defending     int `json:"def"`
	Physical      int `json:"phy"`
}

type rawRatings struct {
	Name        string
	RawAttack   float64
	RawDefense  float64
	RawMidfield float64
	HomeAdv     int
}

type teamAttributes struct {
	Name     string
	Attack   int
	Defense  int
	Midfield int
	HomeAdv  int
	Strength int
}

func homeAdvFromCapacity(team string) int {
	cap := stadiumCapacity[team]
	switch {
	case cap > 70000:
		return 9
	case cap > 60000:
		return 8
	case cap > 50000:
		return 7
	default:
		return 6
	}
}

func normalize(value, min, max float64) int {
	if max == min {
		return 50
	}
	return int(math.Round(((value-min)/(max-min))*99 + 1))
}

func fetchRaw(client *http.Client, name string, teamID int) (rawRatings, error) {
	url := fmt.Sprintf("%s?limit=30&teamId=%d", dropAPIBase, teamID)
	resp, err := client.Get(url)
	if err != nil {
		return rawRatings{}, fmt.Errorf("GET %s: %w", name, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return rawRatings{}, fmt.Errorf("GET %s: unexpected status %d", name, resp.StatusCode)
	}

	var apiResp apiResponse
	if err := json.NewDecoder(resp.Body).Decode(&apiResp); err != nil {
		return rawRatings{}, fmt.Errorf("decode %s: %w", name, err)
	}

	players := apiResp.Items
	sort.Slice(players, func(i, j int) bool {
		return players[i].OverallRating > players[j].OverallRating
	})
	if len(players) > 11 {
		players = players[:11]
	}
	if len(players) == 0 {
		return rawRatings{}, fmt.Errorf("%s: no players returned by API", name)
	}

	var sumAtk, sumDef, sumMid float64
	for _, p := range players {
		sumAtk += float64(p.Pace+p.Shooting) / 2.0
		sumDef += float64(p.Defending+p.Physical) / 2.0
		sumMid += float64(p.Passing+p.Dribbling) / 2.0
	}
	n := float64(len(players))

	return rawRatings{
		Name:        name,
		RawAttack:   sumAtk / n,
		RawDefense:  sumDef / n,
		RawMidfield: sumMid / n,
		HomeAdv:     homeAdvFromCapacity(name),
	}, nil
}

func computeAttributes(raws []rawRatings) []teamAttributes {
	minAtk, maxAtk := raws[0].RawAttack, raws[0].RawAttack
	minDef, maxDef := raws[0].RawDefense, raws[0].RawDefense
	minMid, maxMid := raws[0].RawMidfield, raws[0].RawMidfield

	for _, r := range raws[1:] {
		if r.RawAttack < minAtk {
			minAtk = r.RawAttack
		}
		if r.RawAttack > maxAtk {
			maxAtk = r.RawAttack
		}
		if r.RawDefense < minDef {
			minDef = r.RawDefense
		}
		if r.RawDefense > maxDef {
			maxDef = r.RawDefense
		}
		if r.RawMidfield < minMid {
			minMid = r.RawMidfield
		}
		if r.RawMidfield > maxMid {
			maxMid = r.RawMidfield
		}
	}

	attrs := make([]teamAttributes, len(raws))
	for i, r := range raws {
		atk := normalize(r.RawAttack, minAtk, maxAtk)
		def := normalize(r.RawDefense, minDef, maxDef)
		mid := normalize(r.RawMidfield, minMid, maxMid)
		str := int(math.Round(float64(atk)*0.35 + float64(mid)*0.35 + float64(def)*0.30))
		attrs[i] = teamAttributes{
			Name:     r.Name,
			Attack:   atk,
			Defense:  def,
			Midfield: mid,
			HomeAdv:  r.HomeAdv,
			Strength: str,
		}
	}
	return attrs
}

func printTable(attrs []teamAttributes) {
	fmt.Printf("\n%-20s %7s %7s %8s %7s %9s\n",
		"Team", "Attack", "Defense", "Midfield", "HomeAdv", "Strength")
	fmt.Printf("%-20s %7s %7s %8s %7s %9s\n",
		"--------------------", "------", "-------", "--------", "-------", "--------")
	for _, a := range attrs {
		fmt.Printf("%-20s %7d %7d %8d %7d %9d\n",
			a.Name, a.Attack, a.Defense, a.Midfield, a.HomeAdv, a.Strength)
	}
}

func main() {
	dryRun := flag.Bool("dry-run", false, "print computed ratings without inserting into DB")
	flag.Parse()

	dbURL := os.Getenv("DATABASE_URL")
	if !*dryRun && dbURL == "" {
		log.Fatal("DATABASE_URL is required (set in .env or environment)")
	}

	client := &http.Client{Timeout: 15 * time.Second}

	teamOrder := []string{"Manchester City", "Arsenal", "Liverpool", "Chelsea"}
	raws := make([]rawRatings, 0, 4)
	for _, name := range teamOrder {
		log.Printf("fetching %s (teamId=%d)...", name, teamEAIDs[name])
		raw, err := fetchRaw(client, name, teamEAIDs[name])
		if err != nil {
			log.Fatalf("error: %v", err)
		}
		raws = append(raws, raw)
	}

	attrs := computeAttributes(raws)
	printTable(attrs)

	if *dryRun {
		fmt.Println("\n[dry-run] skipped DB write.")
		return
	}

	ctx := context.Background()
	pool, err := pgxpool.New(ctx, dbURL)
	if err != nil {
		log.Fatalf("connect to DB: %v", err)
	}
	defer pool.Close()

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

	for _, a := range attrs {
		if _, err := pool.Exec(ctx, upsertSQL, a.Name, a.Attack, a.Defense, a.Midfield, a.HomeAdv, a.Strength); err != nil {
			log.Fatalf("upsert %s: %v", a.Name, err)
		}
		log.Printf("upserted: %s", a.Name)
	}

	fmt.Println("\nAll 4 teams upserted successfully.")
}
