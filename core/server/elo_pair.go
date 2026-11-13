package server

import (
	"math"
	"math/rand"
	"sort"
	"strconv"

	"github.com/vanadiry/seshat/core/store"
)

// getELOPair 混合策略返回两个评比条目
// 阶段 1（有条目 <10 场）：40% 低频加权，30% 相近分数，30% 随机
// 阶段 2（全员 ≥10 场）：10% 低频加权，60% 相近分数，20% 保持覆盖，10% 随机
// Staleness 强制配对：staleness > 2N
func getELOPair(dd string) []eloPairEntry {
	keys, err := store.List(dd, "subjects")
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

	// 全部条目 ≥10 场进入阶段 2
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
		// 第二阶段：低频 10%，相似度 60%，保持 20%，随机 10%
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
		// 第一阶段：低频 40%，相似度 30%，随机 30%
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
