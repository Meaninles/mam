package assets

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"mare/services/center/internal/integration"
)

func (s *Service) ExecuteReplicaSyncTask(ctx context.Context, jobID string, itemID string, sourceReplicaID string, targetMountID string) error {
	sourceReplica, err := s.loadOperationReplicaByID(ctx, sourceReplicaID)
	if err != nil {
		return err
	}
	targetMount, err := s.loadOperationMountByID(ctx, targetMountID)
	if err != nil {
		return err
	}

	switch determineReplicateRouteType(sourceReplica.NodeType, targetMount.NodeType) {
	case "UPLOAD":
		return s.executeCloudUploadTask(ctx, jobID, itemID, sourceReplica, targetMount)
	case "DOWNLOAD":
		return s.executeCloudDownloadTask(ctx, jobID, itemID, sourceReplica, targetMount)
	default:
		return s.ExecuteReplicaSync(ctx, sourceReplicaID, targetMountID)
	}
}

func (s *Service) ExecuteReplicaDeletionTask(ctx context.Context, jobID string, itemID string, replicaID string) error {
	replica, err := s.loadOperationReplicaByID(ctx, replicaID)
	if err != nil {
		return err
	}
	if replica.NodeType != "CLOUD" {
		return s.ExecuteReplicaDeletion(ctx, replicaID)
	}
	return s.executeCloudReplicaDeletion(ctx, replica)
}

func (s *Service) executeCloudUploadTask(ctx context.Context, jobID string, itemID string, sourceReplica operationReplica, targetMount operationMount) error {
	if s.cloudResolver == nil {
		return fmt.Errorf("云传输服务尚未启用")
	}
	driver, err := s.cloudResolver.Provider(targetMount.ProviderVendor)
	if err != nil {
		return err
	}
	source, err := s.openUploadSource(ctx, sourceReplica)
	if err != nil {
		return err
	}
	defer source.Close()

	asset, err := s.loadAssetByID(ctx, sourceReplica.AssetID)
	if err != nil {
		return err
	}

	destinationPath := joinCloudReplicaPath(targetMount.SourcePath, asset.RelativePath)
	status, taskEngine, taskID, externalTaskStatus, _, err := s.loadExternalTaskState(ctx, itemID)
	if err == nil && taskEngine != nil && taskID != nil && *taskEngine == "CD2_REMOTE_UPLOAD" {
		if err := driver.AttachUpload(ctx, *taskID, buildCloudDriverPath(targetMount.ProviderPayload, destinationPath), source); err != nil {
			return err
		}
		if shouldResumeCD2Upload(status, externalTaskStatus) {
			_ = driver.ResumeUpload(ctx, *taskID)
		}
	err = driver.WaitUpload(ctx, *taskID, buildCloudDriverPath(targetMount.ProviderPayload, destinationPath), s.progressNotifier(jobID, itemID))
	if err != nil {
		if errors.Is(err, context.Canceled) || isRecoverableCD2InterruptionError(err) {
			handled := s.handleCanceledCloudUpload(ctx, jobID, itemID, driver, *taskID)
			if !handled {
				restartedTaskID, restartedPath, restartErr := driver.StartUpload(ctx, targetMount.ProviderPayload, targetMount.SourcePath, asset.RelativePath, source)
				if restartErr == nil {
					_ = s.updateExternalTask(ctx, jobID, itemID, "CD2_REMOTE_UPLOAD", restartedTaskID, "RUNNING", map[string]any{
						"destinationPath":        restartedPath,
						"sourceReplicaId":        sourceReplica.ID,
						"sourcePhysicalPath":     sourceReplica.PhysicalPath,
						"sourceNodeType":         sourceReplica.NodeType,
						"sourceUsername":         sourceReplica.Username,
						"sourceSecretCiphertext": sourceReplica.SecretCiphertext,
						"sourceSizeBytes":        source.Size(),
						"providerVendor":         targetMount.ProviderVendor,
						"uploadId":               restartedTaskID,
						"deviceId":               s.currentCD2DeviceID(ctx),
					})
					err = driver.WaitUpload(ctx, restartedTaskID, restartedPath, s.progressNotifier(jobID, itemID))
					if err == nil {
						return s.upsertCloudReplica(ctx, asset.ID, targetMount, destinationPath, source.Size())
					}
					if errors.Is(err, context.Canceled) || isRecoverableCD2InterruptionError(err) {
						s.handleCanceledCloudUpload(ctx, jobID, itemID, driver, restartedTaskID)
					}
				}
			}
		}
		return err
	}
		return s.upsertCloudReplica(ctx, asset.ID, targetMount, destinationPath, source.Size())
	}

	externalTaskID, fullPath, err := driver.StartUpload(ctx, targetMount.ProviderPayload, targetMount.SourcePath, asset.RelativePath, source)
	if err != nil {
		return err
	}
	_ = s.updateExternalTask(ctx, jobID, itemID, "CD2_REMOTE_UPLOAD", externalTaskID, "RUNNING", map[string]any{
		"destinationPath":        fullPath,
		"sourceReplicaId":        sourceReplica.ID,
		"sourcePhysicalPath":     sourceReplica.PhysicalPath,
		"sourceNodeType":         sourceReplica.NodeType,
		"sourceUsername":         sourceReplica.Username,
		"sourceSecretCiphertext": sourceReplica.SecretCiphertext,
		"sourceSizeBytes":        source.Size(),
		"providerVendor":         targetMount.ProviderVendor,
		"uploadId":               externalTaskID,
		"deviceId":               s.currentCD2DeviceID(ctx),
	})
	err = driver.WaitUpload(ctx, externalTaskID, fullPath, s.progressNotifier(jobID, itemID))
	if err != nil {
		if errors.Is(err, context.Canceled) || isRecoverableCD2InterruptionError(err) {
			handled := s.handleCanceledCloudUpload(ctx, jobID, itemID, driver, externalTaskID)
			if !handled {
				restartedTaskID, restartedPath, restartErr := driver.StartUpload(ctx, targetMount.ProviderPayload, targetMount.SourcePath, asset.RelativePath, source)
				if restartErr == nil {
					_ = s.updateExternalTask(ctx, jobID, itemID, "CD2_REMOTE_UPLOAD", restartedTaskID, "RUNNING", map[string]any{
						"destinationPath":        restartedPath,
						"sourceReplicaId":        sourceReplica.ID,
						"sourcePhysicalPath":     sourceReplica.PhysicalPath,
						"sourceNodeType":         sourceReplica.NodeType,
						"sourceUsername":         sourceReplica.Username,
						"sourceSecretCiphertext": sourceReplica.SecretCiphertext,
						"sourceSizeBytes":        source.Size(),
						"providerVendor":         targetMount.ProviderVendor,
						"uploadId":               restartedTaskID,
						"deviceId":               s.currentCD2DeviceID(ctx),
					})
					err = driver.WaitUpload(ctx, restartedTaskID, restartedPath, s.progressNotifier(jobID, itemID))
					if err == nil {
						return s.upsertCloudReplica(ctx, asset.ID, targetMount, destinationPath, source.Size())
					}
					if errors.Is(err, context.Canceled) || isRecoverableCD2InterruptionError(err) {
						s.handleCanceledCloudUpload(ctx, jobID, itemID, driver, restartedTaskID)
					}
				}
			}
		}
		return err
	}
	return s.upsertCloudReplica(ctx, asset.ID, targetMount, destinationPath, source.Size())
}

func (s *Service) currentCD2DeviceID(ctx context.Context) string {
	if s.cloudResolver == nil {
		return ""
	}
	deviceID, err := s.cloudResolver.EnsureCD2ClientDeviceID(ctx)
	if err != nil {
		return ""
	}
	return deviceID
}

func (s *Service) executeCloudDownloadTask(ctx context.Context, jobID string, itemID string, sourceReplica operationReplica, targetMount operationMount) error {
	if s.cloudResolver == nil {
		return fmt.Errorf("云传输服务尚未启用")
	}
	driver, err := s.cloudResolver.Provider(sourceReplica.ProviderVendor)
	if err != nil {
		return err
	}
	downloader, err := s.cloudResolver.Downloader("ARIA2")
	if err != nil {
		return err
	}
	asset, err := s.loadAssetByID(ctx, sourceReplica.AssetID)
	if err != nil {
		return err
	}
	sourceInfo, err := driver.ResolveDownloadSource(ctx, sourceReplica.ProviderPayload, "", sourceReplica.PhysicalPath)
	if err != nil {
		return err
	}

	targetPath := resolveUploadPhysicalFilePath(targetMount.SourcePath, asset.RelativePath)
	status, taskEngine, taskID, externalTaskStatus, _, err := s.loadExternalTaskState(ctx, itemID)
	if err == nil && taskEngine != nil && taskID != nil && *taskEngine == "ARIA2" {
		downloadRequest := integration.DownloadRequest{
			URL:               sourceInfo.URL,
			DestinationPath:   targetPath,
			UserAgent:         sourceInfo.UserAgent,
			AdditionalHeaders: sourceInfo.AdditionalHeaders,
		}
		recoveredTaskID, recoverErr := downloader.Recover(ctx, *taskID, downloadRequest)
		if recoverErr != nil {
			return recoverErr
		}
		if recoveredTaskID != *taskID {
			_ = s.updateExternalTask(ctx, jobID, itemID, "ARIA2", recoveredTaskID, "RUNNING", map[string]any{
				"destinationPath": targetPath,
				"sourceUrl":       sourceInfo.URL,
			})
			taskID = &recoveredTaskID
		}
		if shouldResumeAria2Download(status, externalTaskStatus) {
			_ = downloader.Resume(ctx, *taskID)
		}
		err = downloader.Wait(ctx, *taskID, s.progressNotifier(jobID, itemID))
		if err != nil {
			if errors.Is(err, context.Canceled) {
				s.handleCanceledDownload(ctx, jobID, itemID, downloader, *taskID)
			}
			return err
		}
		return s.upsertDownloadedReplica(ctx, asset, targetMount, targetPath, sourceReplica.SizeBytes, sourceReplica.ModifiedAt)
	}

	gid, err := downloader.Enqueue(ctx, integration.DownloadRequest{
		URL:               sourceInfo.URL,
		DestinationPath:   targetPath,
		UserAgent:         sourceInfo.UserAgent,
		AdditionalHeaders: sourceInfo.AdditionalHeaders,
	})
	if err != nil {
		return err
	}
	_ = s.updateExternalTask(ctx, jobID, itemID, "ARIA2", gid, "RUNNING", map[string]any{
		"destinationPath": targetPath,
		"sourceUrl":       sourceInfo.URL,
	})
	err = downloader.Wait(ctx, gid, s.progressNotifier(jobID, itemID))
	if err != nil {
		if errors.Is(err, context.Canceled) {
			s.handleCanceledDownload(ctx, jobID, itemID, downloader, gid)
		}
		return err
	}
	return s.upsertDownloadedReplica(ctx, asset, targetMount, targetPath, sourceReplica.SizeBytes, sourceReplica.ModifiedAt)
}

func (s *Service) executeCloudReplicaDeletion(ctx context.Context, replica operationReplica) error {
	if s.cloudResolver == nil {
		return fmt.Errorf("云传输服务尚未启用")
	}
	driver, err := s.cloudResolver.Provider(replica.ProviderVendor)
	if err != nil {
		return err
	}
	if err := driver.DeleteFile(ctx, replica.ProviderPayload, "", replica.PhysicalPath); err != nil {
		return err
	}
	_, err = s.pool.Exec(ctx, `DELETE FROM asset_replicas WHERE id = $1`, replica.ID)
	return err
}

func (s *Service) upsertCloudReplica(ctx context.Context, assetID string, targetMount operationMount, physicalPath string, sizeBytes int64) error {
	now := s.now().UTC()
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	if err := s.upsertReplica(ctx, tx, assetID, targetMount.ID, physicalPath, uploadFileInfo{
		name:    filepath.Base(physicalPath),
		size:    sizeBytes,
		modTime: now,
	}, now); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func (s *Service) upsertDownloadedReplica(ctx context.Context, asset assetModel, targetMount operationMount, targetPath string, sizeBytes int64, modifiedAt *time.Time) error {
	now := s.now().UTC()
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	if _, err := s.ensureDirectoryChain(ctx, tx, asset.LibraryID, parentLogicalPath(asset.RelativePath), now); err != nil {
		return err
	}
	if err := s.upsertReplica(ctx, tx, asset.ID, targetMount.ID, targetPath, uploadFileInfo{
		name:    filepath.Base(asset.RelativePath),
		size:    sizeBytes,
		modTime: coalesceModifiedTime(modifiedAt, now),
	}, now); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func coalesceModifiedTime(value *time.Time, fallback time.Time) time.Time {
	if value == nil {
		return fallback
	}
	return value.UTC()
}

func joinCloudReplicaPath(rootPath string, relativePath string) string {
	root := strings.TrimSuffix(strings.ReplaceAll(rootPath, "\\", "/"), "/")
	rel := strings.TrimPrefix(strings.ReplaceAll(relativePath, "\\", "/"), "/")
	if root == "" || root == "/" {
		return "/" + rel
	}
	if rel == "" {
		return root
	}
	return root + "/" + rel
}

func buildCloudDriverPath(payload integration.CloudProviderPayload, replicaPath string) string {
	root := strings.TrimSuffix(strings.ReplaceAll(payload.CloudPath, "\\", "/"), "/")
	rel := strings.TrimPrefix(strings.ReplaceAll(replicaPath, "\\", "/"), "/")
	if rel == "" {
		return root
	}
	if root == "" || root == "/" {
		return "/" + rel
	}
	return root + "/" + rel
}

func shouldResumeCD2Upload(itemStatus string, externalTaskStatus *string) bool {
	if itemStatus != "QUEUED" && itemStatus != "WAITING_RETRY" && itemStatus != "RUNNING" {
		return false
	}
	if externalTaskStatus == nil {
		return false
	}
	status := strings.TrimSpace(*externalTaskStatus)
	return status == "Pause"
}

func shouldResumeAria2Download(itemStatus string, externalTaskStatus *string) bool {
	if itemStatus != "QUEUED" && itemStatus != "WAITING_RETRY" && itemStatus != "RUNNING" {
		return false
	}
	if externalTaskStatus == nil {
		return false
	}
	status := strings.TrimSpace(*externalTaskStatus)
	return status == "paused"
}

func isRecoverableCD2InterruptionError(err error) bool {
	if err == nil {
		return false
	}
	return strings.Contains(strings.ToLower(strings.TrimSpace(err.Error())), "interrupted upload cancelled")
}

func (s *Service) progressNotifier(jobID string, itemID string) func(integration.TransferProgress) {
	return func(progress integration.TransferProgress) {
		appendProgressDebugLog(fmt.Sprintf("notify job=%s item=%s done=%d total=%d speed=%d message=%q", jobID, itemID, progress.BytesDone, progress.BytesTotal, progress.SpeedBPS, progress.Message))
		if s.jobRuntime == nil || jobID == "" || itemID == "" {
			appendProgressDebugLog(fmt.Sprintf("skip-missing-runtime job=%s item=%s", jobID, itemID))
			return
		}
		if !s.shouldPersistTransferProgress(itemID, progress) {
			appendProgressDebugLog(fmt.Sprintf("throttled job=%s item=%s done=%d total=%d", jobID, itemID, progress.BytesDone, progress.BytesTotal))
			return
		}
		appendProgressDebugLog(fmt.Sprintf("persist job=%s item=%s done=%d total=%d", jobID, itemID, progress.BytesDone, progress.BytesTotal))
		if err := s.jobRuntime.UpdateItemTransferProgress(context.Background(), jobID, itemID, "RUNNING", progress.BytesDone, progress.BytesTotal, progress.SpeedBPS, progress.Message); err != nil {
			appendProgressDebugLog(fmt.Sprintf("job=%s item=%s done=%d total=%d speed=%d message=%q err=%v", jobID, itemID, progress.BytesDone, progress.BytesTotal, progress.SpeedBPS, progress.Message, err))
		}
	}
}

func appendProgressDebugLog(line string) {
	path := filepath.Join(".tmp", "progress-notifier-debug.log")
	_ = os.MkdirAll(filepath.Dir(path), 0o755)
	file, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return
	}
	defer file.Close()
	_, _ = file.WriteString(time.Now().UTC().Format(time.RFC3339) + " " + line + "\n")
}

func (s *Service) shouldPersistTransferProgress(itemID string, progress integration.TransferProgress) bool {
	now := time.Now()
	if s.now != nil {
		now = s.now()
	}

	s.progressMu.Lock()
	defer s.progressMu.Unlock()
	if s.progressState == nil {
		s.progressState = make(map[string]transferProgressState)
	}

	current := s.progressState[itemID]
	persist := current.LastPersistAt.IsZero()
	if !persist && progress.BytesTotal > 0 && current.BytesTotal > 0 {
		progressDelta := float64(progress.BytesDone-current.BytesDone) / float64(progress.BytesTotal)
		if progressDelta >= 0.05 {
			persist = true
		}
	}
	if !persist && now.Sub(current.LastPersistAt) >= 5*time.Second {
		persist = true
	}
	if persist {
		s.progressState[itemID] = transferProgressState{
			LastPersistAt: now,
			BytesDone:     progress.BytesDone,
			BytesTotal:    progress.BytesTotal,
		}
	}
	return persist
}

func (s *Service) updateExternalTask(ctx context.Context, jobID string, itemID string, engine string, taskID string, status string, payload map[string]any) error {
	if s.jobRuntime == nil || jobID == "" || itemID == "" {
		return nil
	}
	return s.jobRuntime.UpdateExternalTask(ctx, jobID, itemID, engine, taskID, status, payload, nil)
}

func (s *Service) loadExternalTaskState(ctx context.Context, itemID string) (string, *string, *string, *string, *string, error) {
	if s.jobRuntime == nil || itemID == "" {
		return "", nil, nil, nil, nil, fmt.Errorf("job runtime unavailable")
	}
	return s.jobRuntime.LoadExternalTaskState(ctx, itemID)
}

func (s *Service) handleCanceledCloudUpload(ctx context.Context, jobID string, itemID string, driver integration.CloudProviderDriver, taskID string) bool {
	status, _, _, _, _, err := s.loadExternalTaskState(ctx, itemID)
	if err != nil {
		return false
	}
	if status == "PAUSED" {
		_ = driver.PauseUpload(context.Background(), taskID)
		_ = s.updateExternalTask(context.Background(), jobID, itemID, "CD2_REMOTE_UPLOAD", taskID, "Pause", nil)
		return true
	}
	if status == "CANCELED" {
		_ = driver.CancelUpload(context.Background(), taskID)
		_ = s.updateExternalTask(context.Background(), jobID, itemID, "CD2_REMOTE_UPLOAD", taskID, "Cancelled", nil)
		return true
	}
	return false
}

func (s *Service) handleCanceledDownload(ctx context.Context, jobID string, itemID string, downloader integration.DownloadEngine, taskID string) {
	status, _, _, _, _, err := s.loadExternalTaskState(ctx, itemID)
	if err != nil {
		return
	}
	if status == "PAUSED" {
		_ = downloader.Pause(context.Background(), taskID)
		_ = s.updateExternalTask(context.Background(), jobID, itemID, "ARIA2", taskID, "paused", nil)
		return
	}
	if status == "CANCELED" {
		_ = downloader.Cancel(context.Background(), taskID)
		_ = s.updateExternalTask(context.Background(), jobID, itemID, "ARIA2", taskID, "removed", nil)
	}
}
