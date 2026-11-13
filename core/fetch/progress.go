package fetch

import (
	"context"
	"errors"
)

// Progress 拉取过程需要的进度上报能力
type Progress interface {
	Send(step string, done, total int, status string)
	SendPhaseProgress(done, total int, status string)
	SendPhaseStatus(status string)
	BeginPhase(index int)
	SetError(msg string)
	HasPhaseRange() bool
	Cancelled() bool
	Context() context.Context
}

func taskCanceled(p Progress) bool {
	return p != nil && p.Cancelled()
}

func isCanceled(err error) bool {
	return errors.Is(err, context.Canceled)
}
