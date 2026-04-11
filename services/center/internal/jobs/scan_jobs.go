package jobs

import (
	"context"
	"fmt"

	"mare/services/center/internal/assets"
	"mare/services/center/internal/storage"
	jobdto "mare/shared/contracts/dto/job"
)

func (s *Service) CreateDirectoryScanJob(ctx context.Context, plan assets.DirectoryScanPlan) (jobdto.CreateResponse, error) {
	libraryID := plan.LibraryID
	items := make([]CreateItemInput, 0, len(plan.Targets))
	for _, target := range plan.Targets {
		targetMountID := target.MountID
		targetDirectoryID := target.DirectoryID
		items = append(items, CreateItemInput{
			ItemKey:    fmt.Sprintf("directory:%s:mount:%s", target.DirectoryID, target.MountID),
			ItemType:   ItemTypeDirectoryScan,
			Title:      fmt.Sprintf("扫描端点：%s", target.MountName),
			Summary:    fmt.Sprintf("扫描目录 %s", plan.DirectoryRelativePath),
			TargetPath: &target.PhysicalPath,
			Links: []CreateObjectLinkInput{
				{LinkRole: LinkRoleTargetDirectory, ObjectType: ObjectTypeDirectory, DirectoryID: &targetDirectoryID},
				{LinkRole: LinkRoleTargetMount, ObjectType: ObjectTypeMount, MountID: &targetMountID},
			},
		})
	}

	return s.CreateJob(ctx, CreateJobInput{
		LibraryID:     &libraryID,
		JobFamily:     JobFamilyMaintenance,
		JobIntent:     JobIntentScanDirectory,
		Title:         fmt.Sprintf("扫描目录：%s", plan.DirectoryRelativePath),
		Summary:       fmt.Sprintf("已创建 %d 个扫描子项", len(items)),
		SourceDomain:  SourceDomainFileCenter,
		Priority:      PriorityNormal,
		CreatedByType: CreatedByUser,
		SourceSnapshot: map[string]any{
			"directoryId":  plan.DirectoryID,
			"relativePath": plan.DirectoryRelativePath,
			"libraryName":  plan.LibraryName,
			"targetCount":  len(items),
		},
		Items: items,
	})
}

func (s *Service) CreateMountScanJob(ctx context.Context, plan storage.MountScanPlan) (jobdto.CreateResponse, error) {
	items := make([]CreateItemInput, 0, len(plan.Targets))
	for _, target := range plan.Targets {
		targetMountID := target.MountID
		items = append(items, CreateItemInput{
			ItemKey:    fmt.Sprintf("mount:%s", target.MountID),
			ItemType:   ItemTypeDirectoryScan,
			Title:      fmt.Sprintf("扫描挂载：%s", target.MountName),
			Summary:    fmt.Sprintf("扫描挂载路径 %s", target.SourcePath),
			TargetPath: &target.SourcePath,
			Links: []CreateObjectLinkInput{
				{LinkRole: LinkRoleTargetMount, ObjectType: ObjectTypeMount, MountID: &targetMountID},
			},
		})
	}

	var libraryID *string
	if len(plan.Targets) == 1 {
		libraryID = &plan.Targets[0].LibraryID
	}
	return s.CreateJob(ctx, CreateJobInput{
		LibraryID:     libraryID,
		JobFamily:     JobFamilyMaintenance,
		JobIntent:     JobIntentScanDirectory,
		Title:         fmt.Sprintf("挂载扫描：%d 个目标", len(plan.Targets)),
		Summary:       fmt.Sprintf("已创建 %d 个扫描子项", len(items)),
		SourceDomain:  SourceDomainStorageNodes,
		Priority:      PriorityNormal,
		CreatedByType: CreatedByUser,
		SourceSnapshot: map[string]any{
			"targetCount": len(items),
		},
		Items: items,
	})
}
