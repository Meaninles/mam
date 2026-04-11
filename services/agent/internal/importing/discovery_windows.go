//go:build windows

package importing

import (
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"strings"
	"time"
)

type windowsVolumeRecord struct {
	DriveLetter     string `json:"DriveLetter"`
	Path            string `json:"Path"`
	FileSystemLabel string `json:"FileSystemLabel"`
	FileSystem      string `json:"FileSystem"`
	DriveType       string `json:"DriveType"`
	BusType         string `json:"BusType"`
	FriendlyName    string `json:"FriendlyName"`
	SerialNumber    string `json:"SerialNumber"`
	Size            int64  `json:"Size"`
	SizeRemaining   int64  `json:"SizeRemaining"`
}

func discoverAttachedSources(ctx context.Context) ([]SourceDescriptor, error) {
	records, err := queryWindowsVolumes(ctx)
	if err != nil {
		return nil, err
	}
	now := time.Now().UTC().Format(time.RFC3339)
	items := make([]SourceDescriptor, 0, len(records))
	for _, record := range records {
		if !isImportableWindowsVolume(record) {
			continue
		}
		deviceLabel := strings.TrimSpace(record.FileSystemLabel)
		if deviceLabel == "" {
			deviceLabel = strings.TrimSpace(record.FriendlyName)
		}
		if deviceLabel == "" {
			deviceLabel = fmt.Sprintf("%s 盘", strings.ToUpper(record.DriveLetter))
		}
		deviceType := "移动硬盘"
		if strings.EqualFold(strings.TrimSpace(record.DriveType), "Removable") {
			deviceType = "U 盘"
		}
		path := strings.TrimSpace(record.Path)
		if path == "" {
			path = fmt.Sprintf("%s:\\", strings.TrimSuffix(strings.ToUpper(record.DriveLetter), ":"))
		}
		deviceKey := normalizeDeviceKey(path)
		items = append(items, SourceDescriptor{
			DeviceKey:      deviceKey,
			SourceType:     "REMOVABLE_VOLUME",
			DeviceLabel:    deviceLabel,
			DeviceType:     deviceType,
			SourcePath:     path,
			MountPath:      path,
			VolumeName:     optionalString(record.FileSystemLabel),
			FileSystem:     optionalString(record.FileSystem),
			CapacityBytes:  optionalInt64(record.Size),
			AvailableBytes: optionalInt64(record.SizeRemaining),
			ConnectedAt:    now,
			LastSeenAt:     now,
			SourceSnapshot: map[string]any{
				"driveLetter":   record.DriveLetter,
				"driveType":     record.DriveType,
				"busType":       record.BusType,
				"friendlyName":  record.FriendlyName,
				"serialNumber":  record.SerialNumber,
				"sourceType":    "REMOVABLE_VOLUME",
			},
		})
	}
	return items, nil
}

func queryWindowsVolumes(ctx context.Context) ([]windowsVolumeRecord, error) {
	script := "$records = Get-Volume | Where-Object { $_.DriveLetter } | ForEach-Object { $vol = $_; $partition = Get-Partition -DriveLetter $vol.DriveLetter -ErrorAction SilentlyContinue | Select-Object -First 1; $disk = $null; if ($partition) { $disk = $partition | Get-Disk -ErrorAction SilentlyContinue }; [PSCustomObject]@{ DriveLetter = [string]$vol.DriveLetter; Path = ([string]$vol.DriveLetter + ':\\\\'); FileSystemLabel = [string]$vol.FileSystemLabel; FileSystem = [string]$vol.FileSystem; DriveType = [string]$vol.DriveType; BusType = if ($disk) { [string]$disk.BusType } else { '' }; FriendlyName = if ($disk) { [string]$disk.FriendlyName } else { '' }; SerialNumber = if ($disk) { [string]$disk.SerialNumber } else { '' }; Size = [int64]$vol.Size; SizeRemaining = [int64]$vol.SizeRemaining } }; if (-not $records) { '[]' } else { $records | ConvertTo-Json -Compress }"
	command := exec.CommandContext(ctx, "powershell", "-NoLogo", "-NoProfile", "-Command", script)
	output, err := command.Output()
	if err != nil {
		return nil, err
	}
	return parseWindowsVolumesJSON(output)
}

func parseWindowsVolumesJSON(raw []byte) ([]windowsVolumeRecord, error) {
	trimmed := strings.TrimSpace(string(raw))
	if trimmed == "" || trimmed == "null" || trimmed == "[]" {
		return []windowsVolumeRecord{}, nil
	}
	if strings.HasPrefix(trimmed, "{") {
		var item windowsVolumeRecord
		if err := json.Unmarshal([]byte(trimmed), &item); err != nil {
			return nil, err
		}
		return []windowsVolumeRecord{item}, nil
	}
	var items []windowsVolumeRecord
	if err := json.Unmarshal([]byte(trimmed), &items); err != nil {
		return nil, err
	}
	return items, nil
}

func isImportableWindowsVolume(record windowsVolumeRecord) bool {
	busType := strings.ToUpper(strings.TrimSpace(record.BusType))
	driveType := strings.ToUpper(strings.TrimSpace(record.DriveType))
	return busType == "USB" || driveType == "REMOVABLE"
}

func optionalString(value string) *string {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return nil
	}
	return &trimmed
}

func optionalInt64(value int64) *int64 {
	if value <= 0 {
		return nil
	}
	return &value
}
