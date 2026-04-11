package assets

import (
	"context"

	"github.com/jackc/pgx/v5"

	apperrors "mare/services/center/internal/errors"
	assetdto "mare/shared/contracts/dto/asset"
)

func (s *Service) UpdateAnnotations(
	ctx context.Context,
	id string,
	request assetdto.UpdateAnnotationsRequest,
) (assetdto.UpdateAnnotationsResponse, error) {
	if request.Rating < 0 || request.Rating > 5 {
		return assetdto.UpdateAnnotationsResponse{}, apperrors.BadRequest("星级范围无效")
	}

	tx, err := s.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return assetdto.UpdateAnnotationsResponse{}, err
	}
	defer tx.Rollback(ctx)

	var libraryID string
	err = tx.QueryRow(ctx, `
		SELECT library_id
		FROM assets
		WHERE id = $1
		  AND lifecycle_state <> 'DELETED'
	`, id).Scan(&libraryID)
	if err != nil {
		if err == pgx.ErrNoRows {
			var directoryLibraryID string
			directoryErr := tx.QueryRow(ctx, `
				SELECT library_id
				FROM library_directories
				WHERE id = $1
				  AND status <> 'DELETED'
			`, id).Scan(&directoryLibraryID)
			if directoryErr != nil {
				if directoryErr == pgx.ErrNoRows {
					return assetdto.UpdateAnnotationsResponse{}, apperrors.NotFound("未找到指定条目")
				}
				return assetdto.UpdateAnnotationsResponse{}, directoryErr
			}
			if _, err := tx.Exec(ctx, `
				UPDATE library_directories
				SET updated_at = $2
				WHERE id = $1
			`, id, s.now().UTC()); err != nil {
				return assetdto.UpdateAnnotationsResponse{}, err
			}
			if err := s.syncDirectoryTags(ctx, tx, id, directoryLibraryID, request.Tags); err != nil {
				return assetdto.UpdateAnnotationsResponse{}, err
			}
			if err := tx.Commit(ctx); err != nil {
				return assetdto.UpdateAnnotationsResponse{}, err
			}
			return assetdto.UpdateAnnotationsResponse{Message: "资产标记已更新"}, nil
		}
		return assetdto.UpdateAnnotationsResponse{}, err
	}

	_, err = tx.Exec(ctx, `
		UPDATE assets
		SET rating = $2,
		    color_label = $3,
		    updated_at = $4
		WHERE id = $1
		  AND lifecycle_state <> 'DELETED'
	`, id, request.Rating, dbColorLabel(request.ColorLabel), s.now().UTC())
	if err != nil {
		return assetdto.UpdateAnnotationsResponse{}, err
	}
	if err := s.syncAssetTags(ctx, tx, id, libraryID, request.Tags); err != nil {
		return assetdto.UpdateAnnotationsResponse{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return assetdto.UpdateAnnotationsResponse{}, err
	}

	return assetdto.UpdateAnnotationsResponse{Message: "资产标记已更新"}, nil
}
