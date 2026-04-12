package jobs

import (
	"context"
	"fmt"

	"mare/services/center/internal/assets"
	jobdto "mare/shared/contracts/dto/job"
)

func (s *Service) CreateReplicateJob(ctx context.Context, plan assets.ReplicatePlan) (jobdto.CreateResponse, error) {
	libraryID := plan.LibraryID
	routeType := deriveReplicateJobRouteType(plan)

	items := make([]CreateItemInput, 0, len(plan.Items))
	for _, item := range plan.Items {
		assetID := item.AssetID
		sourceReplicaID := item.SourceReplicaID
		sourceMountID := item.SourceMountID
		targetMountID := item.TargetMountID
		sourceStorageNodeID := item.SourceStorageNodeID
		targetStorageNodeID := item.TargetStorageNodeID

		items = append(items, CreateItemInput{
			ItemKey:    fmt.Sprintf("asset:%s:replicate:%s", item.AssetID, item.TargetMountID),
			ItemType:   ItemTypeAssetReplicaTransfer,
			RouteType:  ptr(item.RouteType),
			Title:      item.AssetName,
			Summary:    fmt.Sprintf("同步到 %s", item.TargetMountName),
			SourcePath: &item.SourcePhysicalPath,
			TargetPath: &item.TargetPhysicalPath,
			Links: []CreateObjectLinkInput{
				{LinkRole: LinkRoleSubjectAsset, ObjectType: ObjectTypeAsset, AssetID: &assetID},
				{LinkRole: LinkRoleSubjectReplica, ObjectType: ObjectTypeAssetReplica, AssetReplicaID: &sourceReplicaID},
				{LinkRole: LinkRoleSourceMount, ObjectType: ObjectTypeMount, MountID: &sourceMountID},
				{LinkRole: LinkRoleTargetMount, ObjectType: ObjectTypeMount, MountID: &targetMountID},
				{LinkRole: LinkRoleSourceStorageNode, ObjectType: ObjectTypeStorageNode, StorageNodeID: &sourceStorageNodeID},
				{LinkRole: LinkRoleTargetStorageNode, ObjectType: ObjectTypeStorageNode, StorageNodeID: &targetStorageNodeID},
			},
		})
	}

	title := fmt.Sprintf("同步到端点：%s", plan.EndpointName)
	summary := fmt.Sprintf("已纳入 %d 项同步任务", len(items))
	if plan.SkippedCount > 0 {
		summary = fmt.Sprintf("已纳入 %d 项同步任务，跳过 %d 项", len(items), plan.SkippedCount)
	}
	return s.CreateJob(ctx, CreateJobInput{
		LibraryID:     &libraryID,
		JobFamily:     JobFamilyTransfer,
		JobIntent:     JobIntentReplicate,
		RouteType:     &routeType,
		Title:         title,
		Summary:       summary,
		SourceDomain:  SourceDomainFileCenter,
		Priority:      PriorityNormal,
		CreatedByType: CreatedByUser,
		SourceSnapshot: map[string]any{
			"endpointName":   plan.EndpointName,
			"requestedCount": plan.RequestedCount,
			"plannedCount":   len(items),
			"skippedCount":   plan.SkippedCount,
			"libraryName":    plan.LibraryName,
		},
		Items: items,
		Links: []CreateObjectLinkInput{
			{LinkRole: LinkRoleTargetMount, ObjectType: ObjectTypeMount, MountID: &plan.TargetMountID},
		},
	})
}

func deriveReplicateJobRouteType(plan assets.ReplicatePlan) string {
	if len(plan.Items) == 0 {
		return "COPY"
	}
	current := plan.Items[0].RouteType
	for _, item := range plan.Items[1:] {
		if item.RouteType != current {
			return "COPY"
		}
	}
	if current == "UPLOAD" || current == "DOWNLOAD" {
		return current
	}
	return "COPY"
}

func (s *Service) CreateDeleteReplicaJob(ctx context.Context, plan assets.DeleteReplicaPlan) (jobdto.CreateResponse, error) {
	libraryID := plan.LibraryID

	items := make([]CreateItemInput, 0, len(plan.Items))
	for _, item := range plan.Items {
		assetID := item.AssetID
		replicaID := item.ReplicaID
		targetMountID := item.TargetMountID
		targetStorageNodeID := item.TargetStorageNodeID
		items = append(items, CreateItemInput{
			ItemKey:    fmt.Sprintf("asset:%s:delete-replica:%s", item.AssetID, item.TargetMountID),
			ItemType:   ItemTypeReplicaDelete,
			Title:      item.AssetName,
			Summary:    fmt.Sprintf("删除 %s 副本", item.TargetMountName),
			TargetPath: &item.TargetPhysicalPath,
			Links: []CreateObjectLinkInput{
				{LinkRole: LinkRoleSubjectAsset, ObjectType: ObjectTypeAsset, AssetID: &assetID},
				{LinkRole: LinkRoleSubjectReplica, ObjectType: ObjectTypeAssetReplica, AssetReplicaID: &replicaID},
				{LinkRole: LinkRoleTargetMount, ObjectType: ObjectTypeMount, MountID: &targetMountID},
				{LinkRole: LinkRoleTargetStorageNode, ObjectType: ObjectTypeStorageNode, StorageNodeID: &targetStorageNodeID},
			},
		})
	}

	title := fmt.Sprintf("删除端点副本：%s", plan.EndpointName)
	summary := fmt.Sprintf("已纳入 %d 项副本删除任务", len(items))
	if plan.SkippedCount > 0 {
		summary = fmt.Sprintf("已纳入 %d 项副本删除任务，跳过 %d 项", len(items), plan.SkippedCount)
	}
	return s.CreateJob(ctx, CreateJobInput{
		LibraryID:     &libraryID,
		JobFamily:     JobFamilyMaintenance,
		JobIntent:     JobIntentDeleteReplica,
		Title:         title,
		Summary:       summary,
		SourceDomain:  SourceDomainFileCenter,
		Priority:      PriorityNormal,
		CreatedByType: CreatedByUser,
		SourceSnapshot: map[string]any{
			"endpointName":   plan.EndpointName,
			"requestedCount": plan.RequestedCount,
			"plannedCount":   len(items),
			"skippedCount":   plan.SkippedCount,
			"libraryName":    plan.LibraryName,
		},
		Items: items,
		Links: []CreateObjectLinkInput{
			{LinkRole: LinkRoleTargetMount, ObjectType: ObjectTypeMount, MountID: &plan.TargetMountID},
		},
	})
}

func (s *Service) CreateDeleteAssetJob(ctx context.Context, plan assets.DeleteAssetPlan) (jobdto.CreateResponse, error) {
	libraryID := plan.LibraryID

	items := make([]CreateItemInput, 0, len(plan.Items))
	for _, item := range plan.Items {
		var links []CreateObjectLinkInput
		itemKey := ""
		title := item.EntryName
		summary := "删除资产"
		if item.AssetID != nil {
			assetID := *item.AssetID
			itemKey = fmt.Sprintf("asset:%s:delete", assetID)
			links = append(links, CreateObjectLinkInput{
				LinkRole:   LinkRoleSubjectAsset,
				ObjectType: ObjectTypeAsset,
				AssetID:    &assetID,
			})
		}
		if item.DirectoryID != nil {
			directoryID := *item.DirectoryID
			itemKey = fmt.Sprintf("directory:%s:delete", directoryID)
			summary = "删除目录"
			links = append(links, CreateObjectLinkInput{
				LinkRole:    LinkRoleTargetDirectory,
				ObjectType:  ObjectTypeDirectory,
				DirectoryID: &directoryID,
			})
		}
		items = append(items, CreateItemInput{
			ItemKey:  itemKey,
			ItemType: ItemTypeAssetDeleteStep,
			Title:    title,
			Summary:  summary,
			Links:    links,
		})
	}

	title := "删除条目"
	summary := fmt.Sprintf("已纳入 %d 项条目删除任务", len(items))
	if plan.SkippedCount > 0 {
		summary = fmt.Sprintf("已纳入 %d 项条目删除任务，跳过 %d 项", len(items), plan.SkippedCount)
	}
	return s.CreateJob(ctx, CreateJobInput{
		LibraryID:     &libraryID,
		JobFamily:     JobFamilyMaintenance,
		JobIntent:     JobIntentDeleteAsset,
		Title:         title,
		Summary:       summary,
		SourceDomain:  SourceDomainFileCenter,
		Priority:      PriorityNormal,
		CreatedByType: CreatedByUser,
		SourceSnapshot: map[string]any{
			"requestedCount": plan.RequestedCount,
			"plannedCount":   len(items),
			"skippedCount":   plan.SkippedCount,
			"libraryName":    plan.LibraryName,
		},
		Items: items,
	})
}
