package assets

import (
	"context"

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

	tag, err := s.pool.Exec(ctx, `
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
	if tag.RowsAffected() == 0 {
		if _, dirErr := s.loadDirectoryByID(ctx, id); dirErr == nil {
			return assetdto.UpdateAnnotationsResponse{}, apperrors.BadRequest("当前仅支持为文件设置星级和色标")
		}
		return assetdto.UpdateAnnotationsResponse{}, apperrors.NotFound("未找到指定文件")
	}

	return assetdto.UpdateAnnotationsResponse{Message: "资产标记已更新"}, nil
}
