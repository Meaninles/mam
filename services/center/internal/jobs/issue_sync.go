package jobs

import "context"

func (s *Service) syncJobIssues(ctx context.Context, jobID string) {
	if s.issueSync == nil || jobID == "" {
		return
	}
	_ = s.issueSync.SyncJobIssues(ctx, jobID)
}

func (s *Service) syncJobNotifications(ctx context.Context, jobID string) {
	if s.notificationSync == nil || jobID == "" {
		return
	}
	_ = s.notificationSync.SyncJobNotifications(ctx, jobID)
}
