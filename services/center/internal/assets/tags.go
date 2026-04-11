package assets

import (
	"context"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"

	apperrors "mare/services/center/internal/errors"
)

const defaultTagGroupID = "tag-group-ungrouped"

type assetTagRow struct {
	ID             string
	Name           string
	NormalizedName string
	GroupID        string
	IsPinned       bool
	OrderIndex     int
}

func (s *Service) loadAssetTags(ctx context.Context, assetID string) ([]string, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT t.name
		FROM asset_tag_links atl
		INNER JOIN tags t ON t.id = atl.tag_id
		WHERE atl.asset_id = $1
		ORDER BY atl.order_index ASC, atl.created_at ASC, t.name ASC
	`, assetID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	items := make([]string, 0)
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			return nil, err
		}
		items = append(items, name)
	}
	return items, rows.Err()
}

func (s *Service) loadDirectoryTags(ctx context.Context, directoryID string) ([]string, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT t.name
		FROM directory_tag_links dtl
		INNER JOIN tags t ON t.id = dtl.tag_id
		WHERE dtl.directory_id = $1
		ORDER BY dtl.order_index ASC, dtl.created_at ASC, t.name ASC
	`, directoryID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	items := make([]string, 0)
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			return nil, err
		}
		items = append(items, name)
	}
	return items, rows.Err()
}

func (s *Service) syncAssetTags(ctx context.Context, tx pgx.Tx, assetID string, libraryID string, names []string) error {
	if names == nil {
		return nil
	}
	if err := ensureDefaultAssetTagGroup(ctx, tx, s.now); err != nil {
		return err
	}

	currentTagIDs, err := loadAssetTagIDs(ctx, tx, assetID)
	if err != nil {
		return err
	}
	targetTagIDs := make([]string, 0, len(names))
	affectedTagIDs := append([]string{}, currentTagIDs...)

	normalizedNames := normalizeRequestedTagNames(names)
	for _, candidate := range normalizedNames {
		row, err := loadTagByNormalizedName(ctx, tx, normalizeAssetTagName(candidate))
		if err != nil {
			if err == pgx.ErrNoRows {
				tagID, createErr := createDefaultScopedTag(ctx, tx, candidate, libraryID, s.now)
				if createErr != nil {
					return createErr
				}
				targetTagIDs = append(targetTagIDs, tagID)
				affectedTagIDs = append(affectedTagIDs, tagID)
				continue
			}
			return err
		}
		if err := ensureTagScopeForLibrary(ctx, tx, row.ID, libraryID); err != nil {
			return err
		}
		targetTagIDs = append(targetTagIDs, row.ID)
		affectedTagIDs = append(affectedTagIDs, row.ID)
	}

	if len(targetTagIDs) == 0 {
		if _, err := tx.Exec(ctx, `DELETE FROM asset_tag_links WHERE asset_id = $1`, assetID); err != nil {
			return err
		}
		return refreshAssetTagUsageCounts(ctx, tx, affectedTagIDs, s.now)
	}

	if _, err := tx.Exec(ctx, `
		DELETE FROM asset_tag_links
		WHERE asset_id = $1
		  AND tag_id <> ALL($2)
	`, assetID, targetTagIDs); err != nil {
		return err
	}

	now := s.now().UTC()
	for index, tagID := range targetTagIDs {
		if _, err := tx.Exec(ctx, `
			INSERT INTO asset_tag_links (asset_id, tag_id, order_index, created_at)
			VALUES ($1, $2, $3, $4)
			ON CONFLICT (asset_id, tag_id)
			DO UPDATE SET order_index = EXCLUDED.order_index
		`, assetID, tagID, index, now); err != nil {
			return err
		}
	}

	return refreshAssetTagUsageCounts(ctx, tx, affectedTagIDs, s.now)
}

func (s *Service) syncDirectoryTags(ctx context.Context, tx pgx.Tx, directoryID string, libraryID string, names []string) error {
	if names == nil {
		return nil
	}
	if err := ensureDefaultAssetTagGroup(ctx, tx, s.now); err != nil {
		return err
	}

	currentTagIDs, err := loadDirectoryTagIDs(ctx, tx, directoryID)
	if err != nil {
		return err
	}
	targetTagIDs := make([]string, 0, len(names))
	affectedTagIDs := append([]string{}, currentTagIDs...)

	normalizedNames := normalizeRequestedTagNames(names)
	for _, candidate := range normalizedNames {
		row, err := loadTagByNormalizedName(ctx, tx, normalizeAssetTagName(candidate))
		if err != nil {
			if err == pgx.ErrNoRows {
				tagID, createErr := createDefaultScopedTag(ctx, tx, candidate, libraryID, s.now)
				if createErr != nil {
					return createErr
				}
				targetTagIDs = append(targetTagIDs, tagID)
				affectedTagIDs = append(affectedTagIDs, tagID)
				continue
			}
			return err
		}
		if err := ensureTagScopeForLibrary(ctx, tx, row.ID, libraryID); err != nil {
			return err
		}
		targetTagIDs = append(targetTagIDs, row.ID)
		affectedTagIDs = append(affectedTagIDs, row.ID)
	}

	if len(targetTagIDs) == 0 {
		if _, err := tx.Exec(ctx, `DELETE FROM directory_tag_links WHERE directory_id = $1`, directoryID); err != nil {
			return err
		}
		return refreshAssetTagUsageCounts(ctx, tx, affectedTagIDs, s.now)
	}

	if _, err := tx.Exec(ctx, `
		DELETE FROM directory_tag_links
		WHERE directory_id = $1
		  AND tag_id <> ALL($2)
	`, directoryID, targetTagIDs); err != nil {
		return err
	}

	now := s.now().UTC()
	for index, tagID := range targetTagIDs {
		if _, err := tx.Exec(ctx, `
			INSERT INTO directory_tag_links (directory_id, tag_id, order_index, created_at)
			VALUES ($1, $2, $3, $4)
			ON CONFLICT (directory_id, tag_id)
			DO UPDATE SET order_index = EXCLUDED.order_index
		`, directoryID, tagID, index, now); err != nil {
			return err
		}
	}

	return refreshAssetTagUsageCounts(ctx, tx, affectedTagIDs, s.now)
}

func loadAssetTagIDs(ctx context.Context, tx pgx.Tx, assetID string) ([]string, error) {
	rows, err := tx.Query(ctx, `
		SELECT tag_id
		FROM asset_tag_links
		WHERE asset_id = $1
	`, assetID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	items := make([]string, 0)
	for rows.Next() {
		var tagID string
		if err := rows.Scan(&tagID); err != nil {
			return nil, err
		}
		items = append(items, tagID)
	}
	return items, rows.Err()
}

func loadDirectoryTagIDs(ctx context.Context, tx pgx.Tx, directoryID string) ([]string, error) {
	rows, err := tx.Query(ctx, `
		SELECT tag_id
		FROM directory_tag_links
		WHERE directory_id = $1
	`, directoryID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	items := make([]string, 0)
	for rows.Next() {
		var tagID string
		if err := rows.Scan(&tagID); err != nil {
			return nil, err
		}
		items = append(items, tagID)
	}
	return items, rows.Err()
}

func ensureDefaultAssetTagGroup(ctx context.Context, tx pgx.Tx, now func() time.Time) error {
	timestamp := now().UTC()
	_, err := tx.Exec(ctx, `
		INSERT INTO tag_groups (id, name, order_index, created_at, updated_at)
		VALUES ($1, $2, 0, $3, $3)
		ON CONFLICT (id) DO NOTHING
	`, defaultTagGroupID, "未分组", timestamp)
	return err
}

func createDefaultScopedTag(ctx context.Context, tx pgx.Tx, name string, libraryID string, now func() time.Time) (string, error) {
	orderIndex, err := nextAssetTagOrderIndex(ctx, tx, defaultTagGroupID, false)
	if err != nil {
		return "", err
	}

	tagID := newID("tag")
	timestamp := now().UTC()
	if _, err := tx.Exec(ctx, `
		INSERT INTO tags (id, name, normalized_name, group_id, order_index, is_pinned, usage_count, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, FALSE, 0, $6, $6)
	`, tagID, name, normalizeAssetTagName(name), defaultTagGroupID, orderIndex, timestamp); err != nil {
		return "", err
	}
	if _, err := tx.Exec(ctx, `
		INSERT INTO tag_library_scopes (tag_id, library_id)
		VALUES ($1, $2)
	`, tagID, libraryID); err != nil {
		return "", err
	}
	return tagID, nil
}

func nextAssetTagOrderIndex(ctx context.Context, tx pgx.Tx, groupID string, pinned bool) (int, error) {
	var value int
	if err := tx.QueryRow(ctx, `
		SELECT COALESCE(MAX(order_index), -1) + 1
		FROM tags
		WHERE group_id = $1
		  AND is_pinned = $2
	`, groupID, pinned).Scan(&value); err != nil {
		return 0, err
	}
	return value, nil
}

func loadTagByNormalizedName(ctx context.Context, tx pgx.Tx, normalizedName string) (assetTagRow, error) {
	var row assetTagRow
	err := tx.QueryRow(ctx, `
		SELECT id, name, normalized_name, group_id, is_pinned, order_index
		FROM tags
		WHERE normalized_name = $1
	`, normalizedName).Scan(&row.ID, &row.Name, &row.NormalizedName, &row.GroupID, &row.IsPinned, &row.OrderIndex)
	return row, err
}

func ensureTagScopeForLibrary(ctx context.Context, tx pgx.Tx, tagID string, libraryID string) error {
	var exists int
	if err := tx.QueryRow(ctx, `
		SELECT 1
		FROM tag_library_scopes
		WHERE tag_id = $1
		  AND library_id = $2
	`, tagID, libraryID).Scan(&exists); err != nil {
		if err == pgx.ErrNoRows {
			return apperrors.BadRequest("标签不在当前资产库作用域内")
		}
		return err
	}
	return nil
}

func refreshAssetTagUsageCounts(ctx context.Context, tx pgx.Tx, tagIDs []string, now func() time.Time) error {
	seen := map[string]struct{}{}
	timestamp := now().UTC()
	for _, tagID := range tagIDs {
		trimmed := strings.TrimSpace(tagID)
		if trimmed == "" {
			continue
		}
		if _, ok := seen[trimmed]; ok {
			continue
		}
		seen[trimmed] = struct{}{}

		var count int
		if err := tx.QueryRow(ctx, `
			SELECT
				COALESCE((
					SELECT COUNT(1)
					FROM asset_tag_links atl
					INNER JOIN assets a ON a.id = atl.asset_id
					WHERE atl.tag_id = $1
					  AND a.lifecycle_state <> 'DELETED'
				), 0) +
				COALESCE((
					SELECT COUNT(1)
					FROM directory_tag_links dtl
					INNER JOIN library_directories d ON d.id = dtl.directory_id
					WHERE dtl.tag_id = $1
					  AND d.status <> 'DELETED'
				), 0)
		`, trimmed).Scan(&count); err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, `
			UPDATE tags
			SET usage_count = $2,
			    updated_at = $3
			WHERE id = $1
		`, trimmed, count, timestamp); err != nil {
			return err
		}
	}
	return nil
}

func normalizeRequestedTagNames(names []string) []string {
	seen := map[string]struct{}{}
	items := make([]string, 0, len(names))
	for _, name := range names {
		trimmed := strings.TrimSpace(name)
		if trimmed == "" {
			continue
		}
		normalized := normalizeAssetTagName(trimmed)
		if _, ok := seen[normalized]; ok {
			continue
		}
		seen[normalized] = struct{}{}
		items = append(items, trimmed)
	}
	return items
}

func normalizeAssetTagName(name string) string {
	return strings.ToLower(strings.TrimSpace(name))
}
