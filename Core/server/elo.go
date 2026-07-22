package server

import (
	"encoding/json"
	"math"
	"math/rand"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"time"

	"github.com/vanadiry/seshat/Core/cache"
	"github.com/vanadiry/seshat/Core/config"
	"github.com/vanadiry/seshat/Core/events"
	"github.com/vanadiry/seshat/Core/log"
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
	RankPct float64  `json:"rank_pct"`
}

type eloOrphan struct {
	ID     int     `json:"id"`
	Rating float64 `json:"rating"`
}

type eloRankingResult struct {
	Entries       []eloEntry  `json:"entries"`
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
		os.MkdirAll(filepath.Dir(path), 0o755)
		os.WriteFile(path, []byte(excludeTemplate), 0o644)
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
	os.MkdirAll(filepath.Join(config.Dir(), "user", "elo"), 0o755)
	data, err := json.Marshal(d)
	if err != nil {
		log.Error("saveEloData marshal", "err", err)
		events.Bus.Error("ELO 数据保存失败")
		return
	}
	os.WriteFile(eloDataPath(), data, 0o644)
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
	os.MkdirAll(filepath.Join(config.Dir(), "user", "elo"), 0o755)
	data, err := json.Marshal(h)
	if err != nil {
		log.Error("saveELOHistory marshal", "err", err)
		events.Bus.Error("ELO 历史保存失败")
		return
	}
	os.WriteFile(eloHistoryPath(), data, 0o644)
}

// getELOPair 混合策略返回两个评比条目：
// Phase 1（有人 <10 场）：40% 低频加权，30% 相近分数，30% 随机
// Phase 2（全员 ≥10 场）：10% 低频加权，60% 相近分数，20% 保持覆盖，10% 随机
// Staleness 劫持：staleness > 2N 强制配对
func getELOPair(dd string) []eloPairEntry {
	keys, err := cache.List(dd, "subjects")
	if err != nil || len(keys) < 2 {
		return nil
	}
	excluded := loadExcludeIDs()
	var eligible []int
	for _, k := range keys {
		id, _ := strconv.Atoi(k)
		if !excluded[id] {
			eligible = append(eligible, id)
		}
	}
	if len(eligible) < 2 {
		return nil
	}

	data := loadEloData()

	// Staleness 兜底：超过 2N 次全局比较未出场的条目强制配对
	staleThreshold := len(eligible) * 2
	for _, id := range eligible {
		lastAt := data.Subjects[id].LastCompareAt
		if data.GlobalCompareCount > 0 && data.GlobalCompareCount-lastAt > staleThreshold {
			var opponent int
			for {
				opponent = eligible[rand.Intn(len(eligible))]
				if opponent != id {
					break
				}
			}
			s1 := subjectSummary(dd, id)
			s2 := subjectSummary(dd, opponent)
			return []eloPairEntry{
				{ID: id, Name: s1.Name, NameCN: s1.NameCN},
				{ID: opponent, Name: s2.Name, NameCN: s2.NameCN},
			}
		}
	}

	// Phase: 全部条目 ≥10 场
	allSeen := true
	for _, id := range eligible {
		if data.Subjects[id].Count < 10 {
			allSeen = false
			break
		}
	}

	r := rand.Float64()
	var id1, id2 int
	if allSeen {
		// Phase 2: lowFreq 10%, simScore 60%, maintain 20%, random 10%
		if r < 0.1 {
			id1, id2 = pickLowFreq(eligible, &data)
		} else if r < 0.7 {
			id1, id2 = pickSimilarScore(eligible, &data)
		} else if r < 0.9 {
			id1, id2 = pickMaintain(eligible, &data)
		} else {
			id1, id2 = pickRandomPair(eligible)
		}
	} else {
		// Phase 1: lowFreq 40%, simScore 30%, random 30%
		if r < 0.4 {
			id1, id2 = pickLowFreq(eligible, &data)
		} else if r < 0.7 {
			id1, id2 = pickSimilarScore(eligible, &data)
		} else {
			id1, id2 = pickRandomPair(eligible)
		}
	}

	info1 := subjectSummary(dd, id1)
	info2 := subjectSummary(dd, id2)
	return []eloPairEntry{
		{ID: id1, Name: info1.Name, NameCN: info1.NameCN},
		{ID: id2, Name: info2.Name, NameCN: info2.NameCN},
	}
}

// pickLowFreq 按 1/(count+1) 加权采样选两个不同条目
func pickLowFreq(eligible []int, data *eloData) (int, int) {
	weights := make([]float64, len(eligible))
	total := 0.0
	for i, id := range eligible {
		w := 1.0 / float64(data.Subjects[id].Count+1)
		weights[i] = w
		total += w
	}
	id1 := weightedPick(eligible, weights, total, 0)
	id2 := weightedPick(eligible, weights, total, id1)
	return id1, id2
}

func weightedPick(eligible []int, weights []float64, totalWeight float64, exclude int) int {
	effTotal := totalWeight
	exclIdx := -1
	for i, id := range eligible {
		if id == exclude {
			effTotal -= weights[i]
			exclIdx = i
			break
		}
	}
	if effTotal <= 0 {
		for _, id := range eligible {
			if id != exclude {
				return id
			}
		}
		return 0
	}
	r := rand.Float64() * effTotal
	cumulative := 0.0
	for i, id := range eligible {
		if i == exclIdx {
			continue
		}
		cumulative += weights[i]
		if r < cumulative {
			return id
		}
	}
	for _, id := range eligible {
		if id != exclude {
			return id
		}
	}
	return 0
}

// pickMaintain 从战斗次数最少的 30% 条目中随机配对
func pickMaintain(eligible []int, data *eloData) (int, int) {
	sorted := make([]int, len(eligible))
	copy(sorted, eligible)
	sort.Slice(sorted, func(i, j int) bool {
		return data.Subjects[sorted[i]].Count < data.Subjects[sorted[j]].Count
	})
	n := len(sorted) * 30 / 100
	if n < 2 {
		n = 2
	}
	pool := sorted[:n]
	i1 := rand.Intn(len(pool))
	i2 := rand.Intn(len(pool))
	for i2 == i1 {
		i2 = rand.Intn(len(pool))
	}
	return pool[i1], pool[i2]
}

// pickSimilarScore 随机选一个，再在相对阈值 ±15%（至少 ±100）内选另一个
func pickSimilarScore(eligible []int, data *eloData) (int, int) {
	id1 := eligible[rand.Intn(len(eligible))]
	s1 := data.rating(id1)
	threshold := math.Max(s1*0.15, 100.0)
	var close []int
	for _, id := range eligible {
		if id == id1 {
			continue
		}
		s := data.rating(id)
		if math.Abs(s-s1) <= threshold {
			close = append(close, id)
		}
	}
	if len(close) == 0 {
		return pickRandomPair(eligible)
	}
	return id1, close[rand.Intn(len(close))]
}

// pickRandomPair 随机选两个不同条目
func pickRandomPair(eligible []int) (int, int) {
	if len(eligible) < 2 {
		return 0, 0
	}
	i1 := rand.Intn(len(eligible))
	i2 := rand.Intn(len(eligible))
	for i2 == i1 {
		i2 = rand.Intn(len(eligible))
	}
	return eligible[i1], eligible[i2]
}

// updateELO 更新 ELO 评分并记录对战历史
func updateELO(dd string, winnerID, loserID int) {
	data := loadEloData()
	wb := data.Subjects[winnerID].Count
	lb := data.Subjects[loserID].Count
	k := (kFactor(wb) + kFactor(lb)) / 2.0

	wa := data.rating(winnerID)
	wbRating := data.rating(loserID)

	ea := 1.0 / (1.0 + math.Pow(10, (wbRating-wa)/400))
	eb := 1.0 / (1.0 + math.Pow(10, (wa-wbRating)/400))

	data.Subjects[winnerID] = eloSubjectData{
		Rating:        math.Round((wa+k*(1.0-ea))*10) / 10,
		Count:         wb + 1,
		LastCompareAt: data.GlobalCompareCount,
	}
	data.Subjects[loserID] = eloSubjectData{
		Rating:        math.Round((wbRating+k*(0.0-eb))*10) / 10,
		Count:         lb + 1,
		LastCompareAt: data.GlobalCompareCount,
	}
	data.GlobalCompareCount++

	saveEloData(data)

	history := loadELOHistory()
	history = append(history, eloHistory{Winner: winnerID, Loser: loserID, Time: time.Now().Format(time.RFC3339)})
	saveELOHistory(history)
}

// loadSubjectIndex 读取条目索引
func loadSubjectIndex(dd string) []eloEntry {
	list, _ := loadCachedIndex[[]cache.SubjectSummary](cache.IndexFile(dd, "subjects.json"))
	var result []eloEntry
	for _, s := range list {
		result = append(result, eloEntry{ID: s.ID, Name: s.Name, NameCN: s.NameCN, Score: s.Score})
	}
	return result
}

// getELORanking 返回有 ELO、无 ELO 及孤立评分
func getELORanking(dd string) eloRankingResult {
	data := loadEloData()
	all := loadSubjectIndex(dd)
	seen := map[int]bool{}

	var entries []eloEntry
	var noRating []eloEntry
	for i := range all {
		if v, ok := data.Subjects[all[i].ID]; ok && v.Rating != 0 {
			r := v.Rating
			all[i].Rating = &r
			entries = append(entries, all[i])
		} else {
			noRating = append(noRating, all[i])
		}
		seen[all[i].ID] = true
	}
	sort.Slice(entries, func(i, j int) bool { return *entries[i].Rating > *entries[j].Rating })
	if n := len(entries); n > 1 {
		for i := range entries {
			entries[i].RankPct = math.Round(float64(n-1-i)/float64(n-1)*100*10) / 10
		}
	}
	sort.Slice(noRating, func(i, j int) bool { return noRating[i].Score > noRating[j].Score })

	var orphans []eloOrphan
	for id, v := range data.Subjects {
		if !seen[id] && v.Rating != 0 {
			orphans = append(orphans, eloOrphan{ID: id, Rating: v.Rating})
		}
	}
	sort.Slice(orphans, func(i, j int) bool { return orphans[i].Rating > orphans[j].Rating })

	return eloRankingResult{
		Entries:       entries,
		NoRating:      noRating,
		Orphans:       orphans,
		TotalCompares: data.GlobalCompareCount,
	}
}

// subjectSummary 返回轻量条目信息
type subjectInfo struct {
	Name   string
	NameCN string
	Score  float64
}

func subjectSummary(dd string, id int) subjectInfo {
	data, err := cache.Get(dd, cache.Key("subjects", id, "info.json"))
	if err != nil {
		return subjectInfo{}
	}
	var s struct {
		Name   string `json:"name"`
		NameCN string `json:"name_cn"`
		Rating struct {
			Score float64 `json:"score"`
		} `json:"rating"`
	}
	json.Unmarshal(data, &s)
	return subjectInfo{Name: s.Name, NameCN: s.NameCN, Score: s.Rating.Score}
}

func handleELOPair(dd string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		pair := getELOPair(dd)
		if pair == nil {
			writeError(w, http.StatusInternalServerError, "need at least 2 cached subjects")
			return
		}
		// pair 类型为 []eloPairEntry
		writeJSON(w, pair)
	}
}

func handleELOCompare(dd string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			Winner int `json:"winner"`
			Loser  int `json:"loser"`
		}
		if limitBody(w, r); json.NewDecoder(r.Body).Decode(&req) != nil {
			writeError(w, http.StatusBadRequest, "invalid request body")
			return
		}
		if req.Winner == 0 || req.Loser == 0 {
			writeError(w, 400, "winner and loser required")
			return
		}
		updateELO(dd, req.Winner, req.Loser)
		writeJSON(w, map[string]string{"status": "ok"})
	}
}

func handleELORanking(dd string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) { writeJSON(w, getELORanking(dd)) }
}

func handleELOHistory(dd string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) { writeJSON(w, loadELOHistory()) }
}

func rebuildELO() {
	data := eloData{Subjects: map[int]eloSubjectData{}}
	history := loadELOHistory()
	for _, h := range history {
		wa := data.rating(h.Winner)
		wbRating := data.rating(h.Loser)
		wb := data.Subjects[h.Winner].Count
		lb := data.Subjects[h.Loser].Count
		k := (kFactor(wb) + kFactor(lb)) / 2.0

		ea := 1.0 / (1.0 + math.Pow(10, (wbRating-wa)/400))
		eb := 1.0 / (1.0 + math.Pow(10, (wa-wbRating)/400))

		data.Subjects[h.Winner] = eloSubjectData{
			Rating:        math.Round((wa+k*(1.0-ea))*10) / 10,
			Count:         wb + 1,
			LastCompareAt: data.GlobalCompareCount,
		}
		data.Subjects[h.Loser] = eloSubjectData{
			Rating:        math.Round((wbRating+k*(0.0-eb))*10) / 10,
			Count:         lb + 1,
			LastCompareAt: data.GlobalCompareCount,
		}
		data.GlobalCompareCount++
	}
	saveEloData(data)
}

func handleELORebuild(dd string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		rebuildELO()
		writeJSON(w, map[string]any{"status": "ok", "count": len(loadEloData().Subjects)})
	}
}
