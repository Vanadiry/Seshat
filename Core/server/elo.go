package server

import (
	"net/http"
	"encoding/json"
	"math"
	"math/rand"
	"os"
	"path/filepath"
	"sort"
	"strconv"

	"github.com/vanadiry/seshat/Core/cache"
)

const eloK = 32
const eloDefault = 1500

type eloEntry struct {
	ID     int     `json:"id"`
	Name   string  `json:"name"`
	NameCN string  `json:"name_cn"`
	Score  float64 `json:"score"`
	Rating float64 `json:"rating"`
}

func eloPath(dd string) string {
	return filepath.Join(dd, "elo.json")
}

func loadELO(dd string) map[int]float64 {
	data, err := os.ReadFile(eloPath(dd))
	if err != nil {
		return map[int]float64{}
	}
	var m map[int]float64
	json.Unmarshal(data, &m)
	if m == nil {
		m = map[int]float64{}
	}
	return m
}

func saveELO(dd string, m map[int]float64) {
	data, _ := json.Marshal(m)
	os.WriteFile(eloPath(dd), data, 0o644)
}

// getELOPair returns two random cached subjects for comparison.
func getELOPair(dd string) []eloEntry {
	keys, err := cache.List(dd, "subjects")
	if err != nil || len(keys) < 2 {
		return nil
	}
	// Pick two random indices
	i1 := rand.Intn(len(keys))
	i2 := rand.Intn(len(keys))
	for i2 == i1 {
		i2 = rand.Intn(len(keys))
	}
	return []eloEntry{subjectSummary(dd, keys[i1]), subjectSummary(dd, keys[i2])}
}

// updateELO updates ratings after a comparison.
func updateELO(dd string, winnerID, loserID int) {
	scores := loadELO(dd)
	wa := scores[winnerID]
	if wa == 0 {
		wa = eloDefault
	}
	wb := scores[loserID]
	if wb == 0 {
		wb = eloDefault
	}

	ea := 1.0 / (1.0 + math.Pow(10, (wb-wa)/400))
	eb := 1.0 / (1.0 + math.Pow(10, (wa-wb)/400))

	scores[winnerID] = wa + eloK*(1.0-ea)
	scores[loserID] = wb + eloK*(0.0-eb)

	saveELO(dd, scores)
}

// getELORanking returns subjects sorted by ELO rating descending.
func getELORanking(dd string) []eloEntry {
	scores := loadELO(dd)
	keys, _ := cache.List(dd, "subjects")
	var list []eloEntry
	for _, k := range keys {
		id, err := strconv.Atoi(k)
		if err != nil || id == 0 {
			continue
		}
		s := subjectSummary(dd, k)
		s.Rating = scores[id]
		if s.Rating == 0 {
			s.Rating = eloDefault
		}
		list = append(list, s)
	}
	// Sort by rating desc
	sort.Slice(list, func(i, j int) bool { return list[i].Rating > list[j].Rating })
	return list
}

// subjectSummary returns a lightweight subject entry.
func subjectSummary(dd, key string) eloEntry {
	data, err := cache.Get(dd, "subjects/"+key+".json")
	if err != nil {
		return eloEntry{}
	}
	var s struct {
		ID     int     `json:"id"`
		Name   string  `json:"name"`
		NameCN string  `json:"name_cn"`
		Rating struct {
			Score float64 `json:"score"`
		} `json:"rating"`
	}
	json.Unmarshal(data, &s)
	return eloEntry{ID: s.ID, Name: s.Name, NameCN: s.NameCN, Score: s.Rating.Score}
}

func handleELOPair(dd string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		pair := getELOPair(dd)
		if pair == nil { writeJSON(w, map[string]string{"error": "need at least 2 cached subjects"}); return }
		writeJSON(w, pair)
	}
}

func handleELOCompare(dd string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req struct{ Winner int `json:"winner"`; Loser int `json:"loser"` }
		json.NewDecoder(r.Body).Decode(&req)
		if req.Winner == 0 || req.Loser == 0 { http.Error(w, `{"error":"winner and loser required"}`, 400); return }
		updateELO(dd, req.Winner, req.Loser)
		writeJSON(w, map[string]string{"status": "ok"})
	}
}

func handleELORanking(dd string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) { writeJSON(w, getELORanking(dd)) }
}
