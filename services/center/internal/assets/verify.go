package assets

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"strings"

	"github.com/jackc/pgx/v5"

	apperrors "mare/services/center/internal/errors"
)

type replicaVerificationTarget struct {
	ReplicaID         string
	AssetID           string
	MountID           string
	NodeType          string
	PhysicalPath      string
	Username          string
	SecretCiphertext  string
	ExistingQuickHash *string
}

func (s *Service) ExecuteReplicaVerification(ctx context.Context, replicaID string) error {
	target, err := s.loadReplicaVerificationTarget(ctx, replicaID)
	if err != nil {
		return err
	}
	if target.NodeType == "CLOUD" {
		return apperrors.BadRequest("当前暂不支持云端副本深度校验")
	}

	executor, err := s.executorResolver(target.NodeType)
	if err != nil {
		return err
	}

	hasher := sha256.New()
	streamErr := executor.StreamFile(ctx, pathExecutionContext{
		PhysicalPath:     target.PhysicalPath,
		Username:         target.Username,
		SecretCiphertext: target.SecretCiphertext,
	}, func(reader io.Reader) error {
		_, err := io.Copy(hasher, reader)
		return err
	})
	now := s.now().UTC()
	if streamErr != nil {
		_, _ = s.pool.Exec(ctx, `
			UPDATE asset_replicas
			SET verification_state = 'FAILED',
			    hash_verified_at = $2,
			    last_error_code = 'verification_failed',
			    last_error_message = $3,
			    updated_at = $2
			WHERE id = $1
		`, replicaID, now, streamErr.Error())
		return streamErr
	}

	computedHash := strings.ToLower(hex.EncodeToString(hasher.Sum(nil)))
	if target.ExistingQuickHash != nil && strings.TrimSpace(*target.ExistingQuickHash) != "" && !strings.EqualFold(strings.TrimSpace(*target.ExistingQuickHash), computedHash) {
		_, err = s.pool.Exec(ctx, `
			UPDATE asset_replicas
			SET verification_state = 'MISMATCH',
			    quick_hash = $2,
			    quick_hash_algorithm = 'SHA256',
			    quick_hash_at = $3,
			    hash_verified_at = $3,
			    last_error_code = 'verification_mismatch',
			    last_error_message = '副本内容与最近一次校验哈希不一致',
			    updated_at = $3
			WHERE id = $1
		`, replicaID, computedHash, now)
		if err != nil {
			return err
		}
		return fmt.Errorf("副本校验失败：内容哈希不一致")
	}

	_, err = s.pool.Exec(ctx, `
		UPDATE asset_replicas
		SET verification_state = 'PASSED',
		    quick_hash = $2,
		    quick_hash_algorithm = 'SHA256',
		    quick_hash_at = $3,
		    hash_verified_at = $3,
		    last_error_code = NULL,
		    last_error_message = NULL,
		    updated_at = $3
		WHERE id = $1
	`, replicaID, computedHash, now)
	return err
}

func (s *Service) loadReplicaVerificationTarget(ctx context.Context, replicaID string) (replicaVerificationTarget, error) {
	row := s.pool.QueryRow(ctx, `
		SELECT
			ar.id,
			ar.asset_id,
			ar.mount_id,
			sn.node_type,
			ar.physical_path,
			COALESCE(snc.username, ''),
			COALESCE(snc.secret_ciphertext, ''),
			ar.quick_hash
		FROM asset_replicas ar
		INNER JOIN mounts m ON m.id = ar.mount_id
		INNER JOIN storage_nodes sn ON sn.id = m.storage_node_id
		LEFT JOIN storage_node_credentials snc ON snc.storage_node_id = sn.id
		WHERE ar.id = $1
	`, replicaID)

	var item replicaVerificationTarget
	if err := row.Scan(
		&item.ReplicaID,
		&item.AssetID,
		&item.MountID,
		&item.NodeType,
		&item.PhysicalPath,
		&item.Username,
		&item.SecretCiphertext,
		&item.ExistingQuickHash,
	); err != nil {
		if err == pgx.ErrNoRows {
			return replicaVerificationTarget{}, apperrors.NotFound("副本不存在")
		}
		return replicaVerificationTarget{}, err
	}
	return item, nil
}
