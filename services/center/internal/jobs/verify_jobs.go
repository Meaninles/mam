package jobs

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5"

	apperrors "mare/services/center/internal/errors"
	jobdto "mare/shared/contracts/dto/job"
)

func (s *Service) CreateVerifyReplicaJob(ctx context.Context, replicaID string) (jobdto.CreateResponse, error) {
	replica, err := s.loadVerifyReplicaTarget(ctx, replicaID)
	if err != nil {
		return jobdto.CreateResponse{}, err
	}

	return s.CreateJob(ctx, CreateJobInput{
		LibraryID:     replica.LibraryID,
		JobFamily:     JobFamilyMaintenance,
		JobIntent:     JobIntentVerifyReplica,
		Title:         fmt.Sprintf("校验副本：%s", replica.AssetName),
		Summary:       fmt.Sprintf("校验副本文件 %s", replica.PhysicalPath),
		SourceDomain:  SourceDomainFileCenter,
		Priority:      PriorityNormal,
		CreatedByType: CreatedByUser,
		SourceSnapshot: map[string]any{
			"assetId":      replica.AssetID,
			"entryId":      replica.AssetID,
			"endpointId":   replica.MountID,
			"endpointLabel": replica.MountName,
			"path":         replica.PhysicalPath,
		},
		Items: []CreateItemInput{
			{
				ItemKey:  "verify:replica:" + replica.ReplicaID,
				ItemType: ItemTypeReplicaVerify,
				Title:    fmt.Sprintf("校验副本：%s", replica.AssetName),
				Summary:  "执行副本内容校验",
				Links: []CreateObjectLinkInput{
					{LinkRole: LinkRoleSubjectReplica, ObjectType: ObjectTypeAssetReplica, AssetReplicaID: &replica.ReplicaID},
					{LinkRole: LinkRoleSubjectAsset, ObjectType: ObjectTypeAsset, AssetID: &replica.AssetID},
					{LinkRole: LinkRoleTargetMount, ObjectType: ObjectTypeMount, MountID: &replica.MountID},
				},
			},
		},
		Links: []CreateObjectLinkInput{
			{LinkRole: LinkRoleSubjectReplica, ObjectType: ObjectTypeAssetReplica, AssetReplicaID: &replica.ReplicaID},
			{LinkRole: LinkRoleSubjectAsset, ObjectType: ObjectTypeAsset, AssetID: &replica.AssetID},
			{LinkRole: LinkRoleTargetMount, ObjectType: ObjectTypeMount, MountID: &replica.MountID},
		},
	})
}

type verifyReplicaTarget struct {
	ReplicaID    string
	AssetID      string
	LibraryID    *string
	AssetName    string
	MountID      string
	MountName    string
	PhysicalPath string
}

func (s *Service) loadVerifyReplicaTarget(ctx context.Context, replicaID string) (verifyReplicaTarget, error) {
	row := s.pool.QueryRow(ctx, `
		SELECT
			ar.id,
			a.id,
			a.library_id,
			a.name,
			m.id,
			m.name,
			ar.physical_path
		FROM asset_replicas ar
		INNER JOIN assets a ON a.id = ar.asset_id
		INNER JOIN mounts m ON m.id = ar.mount_id
		WHERE ar.id = $1
	`, replicaID)

	var item verifyReplicaTarget
	if err := row.Scan(&item.ReplicaID, &item.AssetID, &item.LibraryID, &item.AssetName, &item.MountID, &item.MountName, &item.PhysicalPath); err != nil {
		if err == pgx.ErrNoRows {
			return verifyReplicaTarget{}, apperrors.NotFound("副本不存在")
		}
		return verifyReplicaTarget{}, err
	}
	return item, nil
}
