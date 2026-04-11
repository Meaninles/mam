package assets

import (
	"context"
	"time"

	"github.com/jackc/pgx/v5"

	assetdto "mare/shared/contracts/dto/asset"
)

type DirectoryScanPlan struct {
	LibraryID             string
	LibraryName           string
	DirectoryID           string
	DirectoryRelativePath string
	ParentID              *string
	Targets               []DirectoryScanTargetPlan
}

type DirectoryScanTargetPlan struct {
	MountID      string
	MountName    string
	LibraryID    string
	LibraryName  string
	DirectoryID  string
	RelativePath string
	PhysicalPath string
}

func (s *Service) PrepareDirectoryScanPlan(
	ctx context.Context,
	libraryID string,
	request assetdto.ScanDirectoryRequest,
) (DirectoryScanPlan, error) {
	library, err := s.loadLibrary(ctx, libraryID)
	if err != nil {
		return DirectoryScanPlan{}, err
	}

	currentDir, _, err := s.resolveCurrentDirectory(ctx, libraryID, request.ParentID)
	if err != nil {
		return DirectoryScanPlan{}, err
	}

	targets, err := s.loadDirectorySyncTargets(ctx, libraryID, currentDir)
	if err != nil {
		return DirectoryScanPlan{}, err
	}

	items := make([]DirectoryScanTargetPlan, 0, len(targets))
	for _, target := range targets {
		items = append(items, DirectoryScanTargetPlan{
			MountID:      target.MountID,
			MountName:    target.MountName,
			LibraryID:    target.LibraryID,
			LibraryName:  target.LibraryName,
			DirectoryID:  target.DirectoryID,
			RelativePath: target.RelativePath,
			PhysicalPath: target.PhysicalPath,
		})
	}

	return DirectoryScanPlan{
		LibraryID:             library.ID,
		LibraryName:           library.Name,
		DirectoryID:           currentDir.ID,
		DirectoryRelativePath: currentDir.RelativePath,
		ParentID:              request.ParentID,
		Targets:               items,
	}, nil
}

func (s *Service) ExecuteDirectoryScanTarget(ctx context.Context, target DirectoryScanTargetPlan) error {
	now := s.now().UTC()
	tx, err := s.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	if err := s.upsertLibrary(ctx, tx, target.LibraryID, target.LibraryName, now); err != nil {
		return err
	}
	if _, err := s.ensureDirectoryChain(ctx, tx, target.LibraryID, target.RelativePath, now); err != nil {
		return err
	}
	if err := s.syncDirectoryTarget(ctx, tx, directorySyncTarget(target), now); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func (s *Service) ExecuteDirectoryScanPlan(ctx context.Context, plan DirectoryScanPlan) error {
	for _, target := range plan.Targets {
		if err := s.ExecuteDirectoryScanTarget(ctx, target); err != nil {
			return err
		}
	}
	return nil
}

func (s *Service) SyncDirectoryTargetInTx(ctx context.Context, tx pgx.Tx, target DirectoryScanTargetPlan, now time.Time) error {
	return s.syncDirectoryTarget(ctx, tx, directorySyncTarget(target), now)
}
