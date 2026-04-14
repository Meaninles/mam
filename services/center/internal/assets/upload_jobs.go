package assets

import (
	"context"
	"io"
	"os"
	"path/filepath"

	apperrors "mare/services/center/internal/errors"
)

func (s *Service) ExecuteFileCenterUploadJobItem(
	ctx context.Context,
	jobID string,
	itemID string,
	libraryID string,
	logicalPath string,
	sourcePath string,
	targetPath *string,
	targetMountID string,
) error {
	sourcePath = filepath.Clean(sourcePath)
	info, err := os.Stat(sourcePath)
	if err != nil {
		return err
	}
	cleanupSource := false
	defer func() {
		if cleanupSource {
			_ = os.Remove(sourcePath)
		}
	}()

	targetMount, err := s.loadOperationMountByID(ctx, targetMountID)
	if err != nil {
		return err
	}
	if targetMount.LibraryID != libraryID {
		return apperrors.BadRequest("上传目标挂载与资产库不匹配")
	}

	if targetMount.NodeType == "CLOUD" {
		source, err := openLocalUploadSource(sourcePath)
		if err != nil {
			return err
		}
		defer source.Close()

		physicalPath, err := s.uploadSourceToCloudTarget(
			ctx,
			jobID,
			itemID,
			uploadSourceDescriptor{
				ReferenceID:  logicalPath,
				PhysicalPath: sourcePath,
				NodeType:     "LOCAL",
			},
			source,
			targetMount,
			logicalPath,
			nil,
		)
		if err != nil {
			return err
		}
		if err := s.IngestImportedReplica(ctx, ImportedReplicaInput{
			LibraryID:    libraryID,
			MountID:      targetMount.ID,
			LogicalPath:  logicalPath,
			PhysicalPath: physicalPath,
			SizeBytes:    info.Size(),
			ModifiedAt:   info.ModTime().UTC(),
			FileKind:     mapDetectedFileKind(normalizeExtension(filepath.Base(logicalPath))),
		}); err != nil {
			return err
		}
		cleanupSource = true
		return nil
	}

	targetExecutor, err := s.resolveExecutor(targetMount.NodeType)
	if err != nil {
		return err
	}
	physicalPath := resolveUploadPhysicalFilePath(targetMount.SourcePath, logicalPath)
	if targetPath != nil && *targetPath != "" {
		physicalPath = *targetPath
	}
	if err := streamLocalFile(sourcePath, func(reader io.Reader) error {
		return targetExecutor.WriteStream(ctx, pathExecutionContext{
			PhysicalPath:     physicalPath,
			Username:         targetMount.Username,
			SecretCiphertext: targetMount.SecretCiphertext,
		}, reader)
	}); err != nil {
		return err
	}
	if err := targetExecutor.SetFileModifiedTime(ctx, pathExecutionContext{
		PhysicalPath:     physicalPath,
		Username:         targetMount.Username,
		SecretCiphertext: targetMount.SecretCiphertext,
	}, info.ModTime().UTC()); err != nil {
		return err
	}
	if err := s.IngestImportedReplica(ctx, ImportedReplicaInput{
		LibraryID:    libraryID,
		MountID:      targetMount.ID,
		LogicalPath:  logicalPath,
		PhysicalPath: physicalPath,
		SizeBytes:    info.Size(),
		ModifiedAt:   info.ModTime().UTC(),
		FileKind:     mapDetectedFileKind(normalizeExtension(filepath.Base(logicalPath))),
	}); err != nil {
		return err
	}
	cleanupSource = true
	return nil
}
