package server

import (
	"encoding/json"
	"math"
	"os"
	"path/filepath"

	"github.com/vanadiry/seshat/core/config"
	"github.com/vanadiry/seshat/core/events"
	"github.com/vanadiry/seshat/core/log"
	"github.com/vanadiry/seshat/core/store"
)

const eloDefault = 1500

type eloSubjectData struct {
	Rating        float64 `json:"r"`
	Count         int     `json:"c"`
	LastCompareAt int     `json:"t"`
}

type eloData struct {
	GlobalCompareCount int                    `json:"n"`
	Subjects           map[int]eloSubjectData `json:"s"`
}

func (d *eloData) rating(id int) float64 {
	if s, ok := d.Subjects[id]; ok && s.Rating != 0 {
		return s.Rating
	}
	return eloDefault
}

func kFactor(battles int) float64 {
	k := 32.0 / (1.0 + math.Sqrt(float64(battles)/8.0))
	if k < 6.0 {
		k = 6.0
	}
	return k
}

type eloEntry struct {
	ID      int      `json:"id"`
	Name    string   `json:"name"`
	NameCN  string   `json:"name_cn"`
	Score   float64  `json:"score"`
	Rating  *float64 `json:"rating,omitempty"`
	RankPct *float64 `json:"rank_pct,omitempty"`
}

type eloOrphan struct {
	ID     int     `json:"id"`
	Rating float64 `json:"rating"`
}

type eloRankingResult struct {
	Total         int         `json:"total"`
	Limit         int         `json:"limit"`
	Offset        int         `json:"offset"`
	Data          []eloEntry  `json:"data"`
	NoRating      []eloEntry  `json:"no_rating"`
	Orphans       []eloOrphan `json:"orphans"`
	TotalCompares int         `json:"total_compares"`
}

type eloPairEntry struct {
	ID     int    `json:"id"`
	Name   string `json:"name"`
	NameCN string `json:"name_cn"`
}

type eloHistory struct {
	Winner int    `json:"winner"`
	Loser  int    `json:"loser"`
	Time   string `json:"time"`
}

func eloDataPath() string {
	return filepath.Join(config.Dir(), "user", "elo", "elo_data.json")
}

func eloHistoryPath() string {
	return filepath.Join(config.Dir(), "user", "elo", "history.json")
}

func eloExcludePath() string {
	return filepath.Join(config.Dir(), "user", "elo", "exclude.toml")
}

const excludeTemplate = `# ELO 排除名单，此文件中的条目不会被列入评分配对
ids = []
`

// EnsureExcludeFile 确保 ELO 排除文件存在
func EnsureExcludeFile() {
	path := eloExcludePath()
	if _, err := os.Stat(path); os.IsNotExist(err) {
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			log.Error("elo mkdir", "err", err)
			events.Bus.Error("ELO 排除列表初始化失败")
			return
		}
		if err := store.WriteAtomic(path, []byte(excludeTemplate), 0o644); err != nil {
			log.Error("elo exclude write", "err", err)
			events.Bus.Error("ELO 排除列表初始化失败")
		}
	}
}

func loadExcludeIDs() map[int]bool {
	data, err := os.ReadFile(eloExcludePath())
	if err != nil {
		return nil
	}
	ids := parseTOMLIDArray(data)
	if len(ids) == 0 {
		return nil
	}
	m := make(map[int]bool, len(ids))
	for _, id := range ids {
		m[id] = true
	}
	return m
}

func loadEloData() eloData {
	data, err := os.ReadFile(eloDataPath())
	if err != nil {
		return eloData{Subjects: map[int]eloSubjectData{}}
	}
	var d eloData
	json.Unmarshal(data, &d)
	if d.Subjects == nil {
		d.Subjects = map[int]eloSubjectData{}
	}
	return d
}

func saveEloData(d eloData) {
	if err := os.MkdirAll(filepath.Join(config.Dir(), "user", "elo"), 0o755); err != nil {
		log.Error("saveEloData mkdir", "err", err)
		events.Bus.Error("ELO 数据保存失败")
		return
	}
	data, err := json.Marshal(d)
	if err != nil {
		log.Error("saveEloData marshal", "err", err)
		events.Bus.Error("ELO 数据保存失败")
		return
	}
	if err := store.WriteAtomic(eloDataPath(), data, 0o644); err != nil {
		log.Error("saveEloData write", "err", err)
		events.Bus.Error("ELO 数据保存失败")
	}
}

func loadELOHistory() []eloHistory {
	data, err := os.ReadFile(eloHistoryPath())
	if err != nil {
		return []eloHistory{}
	}
	var h []eloHistory
	json.Unmarshal(data, &h)
	if h == nil {
		h = []eloHistory{}
	}
	return h
}

func saveELOHistory(h []eloHistory) {
	if err := os.MkdirAll(filepath.Join(config.Dir(), "user", "elo"), 0o755); err != nil {
		log.Error("saveELOHistory mkdir", "err", err)
		events.Bus.Error("ELO 历史保存失败")
		return
	}
	data, err := json.Marshal(h)
	if err != nil {
		log.Error("saveELOHistory marshal", "err", err)
		events.Bus.Error("ELO 历史保存失败")
		return
	}
	if err := store.WriteAtomic(eloHistoryPath(), data, 0o644); err != nil {
		log.Error("saveELOHistory write", "err", err)
		events.Bus.Error("ELO 历史保存失败")
	}
}
