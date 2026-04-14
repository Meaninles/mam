package storage

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/jackc/pgx/v5"

	"mare/services/center/internal/assets"
	apperrors "mare/services/center/internal/errors"
	"mare/services/center/internal/integration"
)

type MountScanPlan struct {
	Targets []MountScanTargetPlan
}

type MountScanTargetPlan struct {
	MountID     string
	MountName   string
	LibraryID   string
	LibraryName string
	NodeType    string
	SourcePath  string
}

func (s *LocalFolderService) PrepareMountScanPlan(ctx context.Context, ids []string) (MountScanPlan, error) {
	if len(ids) == 0 {
		return MountScanPlan{}, nil
	}

	items := make([]MountScanTargetPlan, 0, len(ids))
	for _, id := range ids {
		target, err := s.loadMountScanPlanTarget(ctx, id)
		if err != nil {
			return MountScanPlan{}, err
		}
		items = append(items, target)
	}
	return MountScanPlan{Targets: items}, nil
}

func (s *LocalFolderService) RunSingleMountScan(ctx context.Context, id string) error {
	_, err := s.runMountScan(ctx, id)
	return err
}

func (s *LocalFolderService) runMountScan(ctx context.Context, id string) (string, error) {
	now := s.now().UTC()
	mount, err := s.loadMountExecutionConfig(ctx, id)
	if err != nil {
		return "", err
	}

	finishedAt := s.now().UTC()
	scanStatus := "SUCCESS"
	summary := fmt.Sprintf("%s 扫描完成，目录可读取。", mount.Name)
	lastErrorCode := ""
	lastErrorMessage := ""
	healthStatus := "ONLINE"
	authStatus := initialMountAuthStatus(mount.NodeType)
	capacityBytes := int64(0)
	availableBytes := int64(0)
	cloudEntries := make([]assets.CloudMountEntry, 0)

	switch mount.NodeType {
	case "LOCAL":
		entries, readErr := os.ReadDir(mount.SourcePath)
		summary = fmt.Sprintf("%s 扫描完成，发现 %d 个直接子项。", mount.Name, len(entries))
		if readErr != nil {
			scanStatus = "FAILED"
			summary = fmt.Sprintf("%s 扫描失败：%s", mount.Name, readErr.Error())
			lastErrorCode = "scan_failed"
			lastErrorMessage = readErr.Error()
			healthStatus = "ERROR"
		}
		if total, free, usageErr := detectDiskUsage(mount.SourcePath); usageErr == nil {
			capacityBytes = total
			availableBytes = free
		}
	case "NAS":
		password, decryptErr := s.cipher.Decrypt(mount.SecretCiphertext)
		if decryptErr != nil {
			scanStatus = "FAILED"
			summary = fmt.Sprintf("%s 扫描失败：NAS 凭据无法读取，请重新保存账号和密码。", mount.Name)
			healthStatus = "ERROR"
			authStatus = "FAILED"
			lastErrorCode = "credential_unreadable"
			lastErrorMessage = decryptErr.Error()
			break
		}
		probe, probeErr := s.nas.Test(ctx, mount.SourcePath, mount.Username, password)
		if probeErr != nil {
			return "", probeErr
		}
		authStatus = probe.AuthStatus
		healthStatus = probe.HealthStatus
		if probe.OverallTone != "success" {
			scanStatus = "FAILED"
			summary = fmt.Sprintf("%s 扫描失败：%s", mount.Name, probe.Summary)
			lastErrorCode = probe.LastErrorCode
			lastErrorMessage = probe.LastErrorMessage
		} else {
			summary = fmt.Sprintf("%s 扫描完成，NAS 路径可读取。", mount.Name)
		}
	case "CLOUD":
		if s.integration == nil {
			scanStatus = "FAILED"
			summary = fmt.Sprintf("%s 扫描失败：云端集成服务尚未启用。", mount.Name)
			healthStatus = "ERROR"
			authStatus = "FAILED"
			lastErrorCode = "integration_unavailable"
			lastErrorMessage = "云端集成服务尚未启用"
			break
		}
		cloudService := NewCloudNodeService(s.pool, s.integration)
		profile, profileErr := cloudService.loadCloudProfile(ctx, mount.NodeID)
		if profileErr != nil {
			scanStatus = "FAILED"
			summary = fmt.Sprintf("%s 扫描失败：网盘配置读取失败。", mount.Name)
			healthStatus = "ERROR"
			authStatus = "FAILED"
			lastErrorCode = "cloud_profile_unavailable"
			lastErrorMessage = profileErr.Error()
			break
		}
		driver, driverErr := s.integration.Provider(profile.ProviderVendor)
		if driverErr != nil {
			scanStatus = "FAILED"
			summary = fmt.Sprintf("%s 扫描失败：云端驱动不可用。", mount.Name)
			healthStatus = "ERROR"
			authStatus = "FAILED"
			lastErrorCode = "cloud_driver_unavailable"
			lastErrorMessage = driverErr.Error()
			break
		}
		if ensureErr := driver.EnsureRemoteRoot(ctx, profile.Payload, mount.SourcePath); ensureErr != nil {
			scanStatus = "FAILED"
			summary = fmt.Sprintf("%s 扫描失败：挂载目录不可访问。", mount.Name)
			healthStatus = "ERROR"
			authStatus = "AUTHORIZED"
			lastErrorCode = "mount_path_unavailable"
			lastErrorMessage = ensureErr.Error()
			break
		}

		lister, ok := driver.(interface {
			ListRemoteEntries(ctx context.Context, payload integration.CloudProviderPayload, remoteRootPath string) ([]integration.CloudFileEntry, error)
		})
		if !ok {
			scanStatus = "FAILED"
			summary = fmt.Sprintf("%s 扫描失败：当前云端驱动暂不支持目录扫描。", mount.Name)
			healthStatus = "ERROR"
			authStatus = "AUTHORIZED"
			lastErrorCode = "cloud_list_not_supported"
			lastErrorMessage = "当前云端驱动暂不支持目录扫描"
			break
		}
		listResult, listErr := lister.ListRemoteEntries(ctx, profile.Payload, mount.SourcePath)
		if listErr != nil {
			scanStatus = "FAILED"
			summary = fmt.Sprintf("%s 扫描失败：读取网盘目录失败。", mount.Name)
			healthStatus = "ERROR"
			authStatus = "AUTHORIZED"
			lastErrorCode = "cloud_list_failed"
			lastErrorMessage = listErr.Error()
			break
		}
		cloudEntries = make([]assets.CloudMountEntry, 0, len(listResult))
		for _, item := range listResult {
			if strings.TrimSpace(item.Name) == "" {
				continue
			}
			cloudEntries = append(cloudEntries, assets.CloudMountEntry{
				Name:        item.Name,
				IsDirectory: item.IsDirectory,
				SizeBytes:   item.SizeBytes,
				ModifiedAt:  item.ModifiedAt,
			})
		}
		summary = fmt.Sprintf("%s 扫描完成，网盘挂载目录可读取，发现 %d 个直接子项。", mount.Name, len(cloudEntries))
		healthStatus = "ONLINE"
		authStatus = "AUTHORIZED"
	default:
		return "", fmt.Errorf("当前挂载类型暂不支持扫描: %s", mount.NodeType)
	}

	if scanStatus == "SUCCESS" && s.assetService != nil {
		if mount.NodeType == "CLOUD" {
			if syncErr := s.assetService.SyncCloudMountFirstLevel(ctx, id, cloudEntries); syncErr != nil {
				scanStatus = "FAILED"
				summary = fmt.Sprintf("%s 扫描失败：资产索引写入失败", mount.Name)
				lastErrorCode = "asset_index_failed"
				lastErrorMessage = syncErr.Error()
				healthStatus = "ERROR"
			}
		} else {
			if syncErr := s.assetService.SyncMount(ctx, id); syncErr != nil {
				scanStatus = "FAILED"
				summary = fmt.Sprintf("%s 扫描失败：资产索引写入失败", mount.Name)
				lastErrorCode = "asset_index_failed"
				lastErrorMessage = syncErr.Error()
				healthStatus = "ERROR"
			}
		}
	}

	_, err = s.pool.Exec(ctx, `
		UPDATE mount_runtime
		SET scan_status = $2,
		    last_scan_at = $3,
		    last_scan_summary = $4,
		    health_status = $5,
		    auth_status = $6,
		    last_error_code = NULLIF($7, ''),
		    last_error_message = NULLIF($8, ''),
		    capacity_bytes = CASE WHEN $9::bigint > 0 THEN $9::bigint ELSE NULL END,
		    available_bytes = CASE WHEN $10::bigint > 0 THEN $10::bigint ELSE NULL END,
		    updated_at = $3
		WHERE mount_id = $1
	`, id, scanStatus, finishedAt, summary, healthStatus, authStatus, lastErrorCode, lastErrorMessage, capacityBytes, availableBytes)
	if err != nil {
		return "", err
	}

	_, err = s.pool.Exec(ctx, `
		INSERT INTO mount_scan_histories (
			id, mount_id, started_at, finished_at, status, summary, trigger
		) VALUES ($1, $2, $3, $4, $5, $6, '手动扫描')
	`, buildCode("scan-history-id", finishedAt), id, now, finishedAt, scanStatus, summary)
	if err != nil {
		return "", err
	}
	return summary, nil
}

func (s *LocalFolderService) loadMountScanPlanTarget(ctx context.Context, id string) (MountScanTargetPlan, error) {
	var item MountScanTargetPlan
	err := s.pool.QueryRow(ctx, `
		SELECT
			m.id,
			m.name,
			m.library_id,
			m.library_name,
			sn.node_type,
			m.source_path
		FROM mounts m
		INNER JOIN storage_nodes sn ON sn.id = m.storage_node_id
		WHERE m.id = $1
		  AND m.deleted_at IS NULL
		  AND sn.deleted_at IS NULL
	`, id).Scan(&item.MountID, &item.MountName, &item.LibraryID, &item.LibraryName, &item.NodeType, &item.SourcePath)
	if err != nil {
		if err == pgx.ErrNoRows {
			return MountScanTargetPlan{}, apperrors.NotFound("挂载文件夹不存在")
		}
		return MountScanTargetPlan{}, err
	}
	return item, nil
}
