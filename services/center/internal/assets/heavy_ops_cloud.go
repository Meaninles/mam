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
	source, err := s.openUploadSource(ctx, sourceReplica)
	if err != nil {
		return err
	}
	defer source.Close()

	asset, err := s.loadAssetByID(ctx, sourceReplica.AssetID)
	if err != nil {
		return err
	}
	_, err = s.uploadSourceToCloudTarget(
		ctx,
		jobID,
		itemID,
		uploadSourceDescriptor{
			ReferenceID:      sourceReplica.ID,
			PhysicalPath:     sourceReplica.PhysicalPath,
			NodeType:         sourceReplica.NodeType,
			Username:         sourceReplica.Username,
			SecretCiphertext: sourceReplica.SecretCiphertext,
		},
		source,
		targetMount,
		asset.RelativePath,
		func(physicalPath string) error {
			return s.upsertCloudReplica(ctx, asset.ID, targetMount, physicalPath, source.Size())
		},
	)
	return err
}

func (s *Service) uploadSourceToCloudTarget(
	ctx context.Context,
	jobID string,
	itemID string,
	sourceDescriptor uploadSourceDescriptor,
	source integration.UploadSource,
	targetMount operationMount,
	relativePath string,
	onComplete func(physicalPath string) error,
) (string, error) {
	if s.cloudResolver == nil {
		return "", fmt.Errorf("云传输服务尚未启用")
	}
	driver, err := s.cloudResolver.Provider(targetMount.ProviderVendor)
	if err != nil {
		return "", err
	}

	if err := driver.EnsureRemoteRoot(ctx, targetMount.ProviderPayload, targetMount.SourcePath); err != nil {
		return "", err
	}

	destinationPath := joinCloudReplicaPath(targetMount.SourcePath, relativePath)
	status, taskEngine, taskID, externalTaskStatus, _, err := s.loadExternalTaskState(ctx, itemID)
	if err == nil && taskEngine != nil && taskID != nil && *taskEngine == "CD2_REMOTE_UPLOAD" {
		if err := driver.AttachUpload(ctx, *taskID, buildCloudDriverPath(targetMount.ProviderPayload, destinationPath), source); err != nil {
			return "", err
		}
		if shouldResumeCD2Upload(status, externalTaskStatus) {
			appendProgressDebugLog(fmt.Sprintf("cd2-resume-existing job=%s item=%s task=%s externalStatus=%v", jobID, itemID, *taskID, externalTaskStatus))
			if resumeErr := driver.ResumeUpload(ctx, *taskID); resumeErr != nil {
				appendProgressDebugLog(fmt.Sprintf("cd2-resume-existing-error job=%s item=%s task=%s err=%v", jobID, itemID, *taskID, resumeErr))
			}
		}
		return s.waitForCD2UploadWithRecovery(
			ctx,
			jobID,
			itemID,
			driver,
			sourceDescriptor,
			source,
			targetMount,
			*taskID,
			buildCloudDriverPath(targetMount.ProviderPayload, destinationPath),
			func(currentPath string) error {
				if onComplete != nil {
					return onComplete(currentPath)
				}
				return nil
			},
		)
	}

	externalTaskID, fullPath, err := driver.StartUpload(ctx, targetMount.ProviderPayload, targetMount.SourcePath, relativePath, source)
	if err != nil {
		return "", err
	}
	_ = s.updateExternalTask(ctx, jobID, itemID, "CD2_REMOTE_UPLOAD", externalTaskID, "RUNNING", s.buildCloudUploadTaskPayload(sourceDescriptor, targetMount, source, fullPath, externalTaskID))
	return s.waitForCD2UploadWithRecovery(
		ctx,
		jobID,
		itemID,
		driver,
		sourceDescriptor,
		source,
		targetMount,
		externalTaskID,
		fullPath,
		func(currentPath string) error {
			if onComplete != nil {
				return onComplete(currentPath)
			}
			return nil
		},
	)
}

func (s *Service) waitForCD2UploadWithRecovery(
	ctx context.Context,
	jobID string,
	itemID string,
	driver integration.CloudProviderDriver,
	sourceDescriptor uploadSourceDescriptor,
	source integration.UploadSource,
	targetMount operationMount,
	initialTaskID string,
	initialPath string,
	onComplete func(physicalPath string) error,
) (string, error) {
	currentTaskID := initialTaskID
	currentPath := initialPath
	reboundCurrentTask := false

	for restartCount := 0; restartCount < 3; restartCount++ {
		err := driver.WaitUpload(ctx, currentTaskID, currentPath, s.progressNotifier(jobID, itemID))
		if err == nil {
			if onComplete != nil {
				if err := onComplete(currentPath); err != nil {
					return "", err
				}
			}
			_ = s.updateExternalTask(ctx, jobID, itemID, "CD2_REMOTE_UPLOAD", currentTaskID, "Finish", map[string]any{
				"destinationPath": currentPath,
			})
			return currentPath, nil
		}
		if errors.Is(err, context.Canceled) && ctx.Err() != nil {
			return "", err
		}
		if isCD2UploadConfirmationTimeoutError(err) {
			verifiedRelativePath := relativePathFromCloudDestination(targetMount.ProviderPayload, targetMount.SourcePath, currentPath)
			verified, verifyErr := s.confirmRemoteUploadPresent(ctx, driver, targetMount, verifiedRelativePath, source.Size())
			if verifyErr == nil && verified {
				if onComplete != nil {
					if err := onComplete(currentPath); err != nil {
						return "", err
					}
				}
				_ = s.updateExternalTask(ctx, jobID, itemID, "CD2_REMOTE_UPLOAD", currentTaskID, "Finish", map[string]any{
					"destinationPath": currentPath,
					"verifiedBy":      "remote-listing",
				})
				return currentPath, nil
			}
			_ = driver.CancelUpload(context.Background(), currentTaskID)
			_ = driver.ResetUploadSession(ctx)
		}
		if !errors.Is(err, context.Canceled) && !isRecoverableCD2InterruptionError(err) {
			return "", err
		}

		handled := s.handleCanceledCloudUpload(ctx, jobID, itemID, driver, currentTaskID)
		if handled {
			return "", err
		}
		if isCD2UploadSessionNotFoundError(err) && !reboundCurrentTask {
			appendProgressDebugLog(fmt.Sprintf("cd2-rebind-existing job=%s item=%s task=%s", jobID, itemID, currentTaskID))
			_ = driver.ResetUploadSession(ctx)
			attachErr := driver.AttachUpload(ctx, currentTaskID, currentPath, source)
			if attachErr == nil {
				if resumeErr := driver.ResumeUpload(ctx, currentTaskID); resumeErr != nil {
					appendProgressDebugLog(fmt.Sprintf("cd2-rebind-existing-resume-error job=%s item=%s task=%s err=%v", jobID, itemID, currentTaskID, resumeErr))
				}
				reboundCurrentTask = true
				continue
			}
			appendProgressDebugLog(fmt.Sprintf("cd2-rebind-existing-attach-error job=%s item=%s task=%s err=%v", jobID, itemID, currentTaskID, attachErr))
		}
		if restartCount == 2 {
			return "", err
		}
		if isCD2UploadSessionNotFoundError(err) {
			_ = driver.CancelUpload(context.Background(), currentTaskID)
			appendProgressDebugLog(fmt.Sprintf("cd2-reset-session-before-restart job=%s item=%s task=%s", jobID, itemID, currentTaskID))
			_ = driver.ResetUploadSession(ctx)
		}

		restartedTaskID, restartedPath, restartErr := driver.StartUpload(
			ctx,
			targetMount.ProviderPayload,
			targetMount.SourcePath,
			relativePathFromCloudDestination(targetMount.ProviderPayload, targetMount.SourcePath, currentPath),
			source,
		)
		if restartErr != nil {
			return "", err
		}
		appendProgressDebugLog(fmt.Sprintf("cd2-start-recreated job=%s item=%s oldTask=%s newTask=%s", jobID, itemID, currentTaskID, restartedTaskID))
		currentTaskID = restartedTaskID
		currentPath = restartedPath
		reboundCurrentTask = false
		_ = s.updateExternalTask(ctx, jobID, itemID, "CD2_REMOTE_UPLOAD", currentTaskID, "RUNNING", s.buildCloudUploadTaskPayload(sourceDescriptor, targetMount, source, currentPath, currentTaskID))
	}

	return "", fmt.Errorf("CloudDrive2 上传恢复已超过最大重建次数")
}

func (s *Service) buildCloudUploadTaskPayload(
	source uploadSourceDescriptor,
	targetMount operationMount,
	upload integration.UploadSource,
	destinationPath string,
	uploadID string,
) map[string]any {
	return map[string]any{
		"destinationPath":        destinationPath,
		"sourceReferenceId":      source.ReferenceID,
		"sourcePhysicalPath":     source.PhysicalPath,
		"sourceNodeType":         source.NodeType,
		"sourceUsername":         source.Username,
		"sourceSecretCiphertext": source.SecretCiphertext,
		"sourceSizeBytes":        upload.Size(),
		"providerVendor":         targetMount.ProviderVendor,
		"uploadId":               uploadID,
		"deviceId":               s.currentCD2DeviceID(context.Background()),
	}
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

func relativePathFromCloudDestination(payload integration.CloudProviderPayload, remoteRootPath string, fullPath string) string {
	root := buildCloudDestinationBase(payload, remoteRootPath)
	root = strings.TrimSuffix(strings.ReplaceAll(root, "\\", "/"), "/")
	full := strings.TrimSpace(strings.ReplaceAll(fullPath, "\\", "/"))
	if root == "" || root == "/" {
		return strings.TrimPrefix(full, "/")
	}
	return strings.TrimPrefix(strings.TrimPrefix(full, root), "/")
}

func buildCloudDestinationBase(payload integration.CloudProviderPayload, remoteRootPath string) string {
	root := strings.TrimSpace(strings.ReplaceAll(payload.CloudPath, "\\", "/"))
	if root == "" {
		root = "/"
	}
	if !strings.HasPrefix(root, "/") {
		root = "/" + root
	}
	root = strings.TrimSuffix(root, "/")

	remote := strings.Trim(strings.ReplaceAll(remoteRootPath, "\\", "/"), "/")
	if remote == "" {
		return root
	}
	return root + "/" + remote
}

func shouldResumeCD2Upload(itemStatus string, externalTaskStatus *string) bool {
	if itemStatus != "QUEUED" && itemStatus != "WAITING_RETRY" && itemStatus != "RUNNING" {
		return false
	}
	if externalTaskStatus == nil {
		return false
	}
	status := strings.TrimSpace(*externalTaskStatus)
	switch status {
	case "Pause", "Transfer", "WaitforPreprocessing", "Preprocessing", "Inqueue":
		return true
	default:
		return false
	}
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
	message := strings.ToLower(strings.TrimSpace(err.Error()))
	return strings.Contains(message, "interrupted upload cancelled") ||
		strings.Contains(message, "完成状态长时间未得到确认") ||
		strings.Contains(message, "upload session not found") ||
		strings.Contains(message, "session not found")
}

func isCD2UploadConfirmationTimeoutError(err error) bool {
	if err == nil {
		return false
	}
	return strings.Contains(strings.TrimSpace(err.Error()), "上传完成状态长时间未得到确认")
}

func (s *Service) confirmRemoteUploadPresent(
	ctx context.Context,
	driver integration.CloudProviderDriver,
	targetMount operationMount,
	relativePath string,
	expectedSize int64,
) (bool, error) {
	parentPath := parentLogicalPath(relativePath)
	entries, err := driver.ListRemoteEntries(ctx, targetMount.ProviderPayload, joinCloudReplicaPath(targetMount.SourcePath, parentPath))
	if err != nil {
		return false, err
	}
	name := filepath.Base(relativePath)
	for _, entry := range entries {
		if entry.IsDirectory {
			continue
		}
		if entry.Name == name && entry.SizeBytes == expectedSize {
			return true, nil
		}
	}
	return false, nil
}

func isCD2UploadSessionNotFoundError(err error) bool {
	if err == nil {
		return false
	}
	return strings.Contains(strings.ToLower(strings.TrimSpace(err.Error())), "upload session not found")
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
