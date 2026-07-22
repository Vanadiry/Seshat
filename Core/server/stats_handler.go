package server

import (
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"

	"github.com/vanadiry/seshat/Core/cache"
	"github.com/vanadiry/seshat/Core/config"
)

func handleStats(dd string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		type sizeCount struct{ Large, Grid, Small int }
		type imageStats struct {
			Total      int       `json:"total"`
			BySize     sizeCount `json:"by_size"`
			Subjects   int       `json:"subjects"`
			Characters int       `json:"characters"`
			Persons    int       `json:"persons"`
			UniqueSubj int       `json:"unique_subjects"`
			UniqueChar int       `json:"unique_characters"`
			UniquePers int       `json:"unique_persons"`
		}
		type entryCount struct {
			Subjects   int `json:"subjects"`
			Characters int `json:"characters"`
			Persons    int `json:"persons"`
		}
		type trackerInfo struct {
			Name  string `json:"name"`
			Count int    `json:"count"`
		}

		// Image stats
		var img imageStats
		for _, kind := range []string{"subject", "character", "person"} {
			m := loadImageIndex(dd, kind+"s_image.json")
			var domainTotal, uniqueCount int
			for _, e := range m {
				if e.Large != "" {
					img.BySize.Large++
					img.Total++
					domainTotal++
				}
				if e.Grid != "" {
					img.BySize.Grid++
					img.Total++
					domainTotal++
				}
				if e.Small != "" {
					img.BySize.Small++
					img.Total++
					domainTotal++
				}
				if e.Large != "" || e.Grid != "" || e.Small != "" {
					uniqueCount++
				}
			}
			switch kind {
			case "subject":
				img.Subjects = domainTotal
				img.UniqueSubj = uniqueCount
			case "character":
				img.Characters = domainTotal
				img.UniqueChar = uniqueCount
			case "person":
				img.Persons = domainTotal
				img.UniquePers = uniqueCount
			}
		}

		// Cache counts from index lists
		var entries entryCount
		for _, domain := range []string{"subjects", "characters", "persons"} {
			list := loadNameList(cache.IndexFile(dd, domain+".json"))
			switch domain {
			case "subjects":
				entries.Subjects = len(list)
			case "characters":
				entries.Characters = len(list)
			case "persons":
				entries.Persons = len(list)
			}
		}

		// Trackers
		var trackers []trackerInfo
		td := filepath.Join(config.Dir(), "tracker")
		files, _ := filepath.Glob(filepath.Join(td, "*.json"))
		files2, _ := filepath.Glob(filepath.Join(td, "*.toml"))
		for _, f := range append(files, files2...) {
			name := filepath.Base(f)
			name = name[:len(name)-len(filepath.Ext(name))]
			count := len(loadTrackerIDs(f))
			trackers = append(trackers, trackerInfo{Name: name, Count: count})
		}

		// Collections
		collCount := 0
		if data, err := os.ReadFile(filepath.Join(config.Dir(), "user", "info", "collections.json")); err == nil {
			var coll struct {
				Subjects map[string]int `json:"subjects"`
			}
			if json.Unmarshal(data, &coll) == nil {
				collCount = len(coll.Subjects)
			}
		}

		// ELO counts
		eloScores := 0
		if data, err := os.ReadFile(filepath.Join(config.Dir(), "user", "elo", "elo_data.json")); err == nil {
			var ed eloData
			if json.Unmarshal(data, &ed) == nil {
				eloScores = len(ed.Subjects)
			}
		}
		eloComparisons := len(loadELOHistory())

		// Tracker total entries
		trackerTotal := 0
		for _, t := range trackers {
			trackerTotal += t.Count
		}

		writeJSON(w, map[string]any{
			"images":          img,
			"cache_entries":   entries,
			"trackers":        trackers,
			"tracker_total":   trackerTotal,
			"collections":     collCount,
			"elo_scores":      eloScores,
			"elo_comparisons": eloComparisons,
		})
	}
}
