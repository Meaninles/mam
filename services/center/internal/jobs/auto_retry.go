package jobs

import (
	"strings"
	"time"
)

func (s *Service) shouldAutoRetry(job jobRow, item itemRow, message string) bool {
	if item.AttemptCount >= s.retryMaxAttempts {
		return false
	}
	switch job.JobIntent {
	case JobIntentConnectionTest, JobIntentReplicate, JobIntentImport:
	default:
		return false
	}
	lowered := strings.ToLower(strings.TrimSpace(message))
	if lowered == "" {
		return false
	}
	return strings.Contains(lowered, "失败") ||
		strings.Contains(lowered, "timeout") ||
		strings.Contains(lowered, "不可达") ||
		strings.Contains(lowered, "unavailable") ||
		strings.Contains(lowered, "interrupted")
}

func (s *Service) computeRetryDelay(attemptCount int) time.Duration {
	if attemptCount <= 0 {
		attemptCount = 1
	}
	delay := s.retryBaseDelay
	for i := 1; i < attemptCount; i++ {
		delay *= 2
		if delay > 5*time.Minute {
			return 5 * time.Minute
		}
	}
	if delay <= 0 {
		return 30 * time.Second
	}
	return delay
}
