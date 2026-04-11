package tags

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"strconv"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	apperrors "mare/services/center/internal/errors"
	tagdto "mare/shared/contracts/dto/tag"
)

const defaultGroupID = "tag-group-ungrouped"

type dbExecutor interface {
	Exec(ctx context.Context, sql string, arguments ...any) (pgconn.CommandTag, error)
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
}

type Service struct {
	pool *pgxpool.Pool
	now  func() time.Time
}

type groupRow struct {
	ID         string
	Name       string
	OrderIndex int
}

type tagRow struct {
	ID             string
	Name           string
	NormalizedName string
	GroupID        string
	GroupName      string
	OrderIndex     int
	IsPinned       bool
	UsageCount     int
	CreatedAt      time.Time
	UpdatedAt      time.Time
}

func NewService(pool *pgxpool.Pool) *Service {
	return &Service{pool: pool, now: time.Now}
}

func (s *Service) LoadManagementSnapshot(ctx context.Context, searchText string) (tagdto.ManagementSnapshot, error) {
	if err := s.ensureDefaultGroup(ctx, s.pool); err != nil {
		return tagdto.ManagementSnapshot{}, err
	}

	libraries, err := s.loadLibraries(ctx)
	if err != nil {
		return tagdto.ManagementSnapshot{}, err
	}

	groups, err := s.loadGroups(ctx)
	if err != nil {
		return tagdto.ManagementSnapshot{}, err
	}

	tags, err := s.loadTagRecords(ctx)
	if err != nil {
		return tagdto.ManagementSnapshot{}, err
	}

	if keyword := strings.ToLower(strings.TrimSpace(searchText)); keyword != "" {
		filtered := make([]tagdto.Record, 0, len(tags))
		for _, item := range tags {
			if strings.Contains(strings.ToLower(item.Name), keyword) || strings.Contains(strings.ToLower(item.GroupName), keyword) {
				filtered = append(filtered, item)
			}
		}
		tags = filtered
	}

	groupStats := make(map[string]*tagdto.GroupRecord, len(groups))
	for _, group := range groups {
		groupStats[group.ID] = &tagdto.GroupRecord{
			ID:         group.ID,
			Name:       group.Name,
			OrderIndex: group.OrderIndex,
		}
	}

	overview := tagdto.ManagementOverview{}
	for _, item := range tags {
		overview.TotalTags++
		if item.UsageCount > 0 {
			overview.UsedTagCount++
		}
		if item.GroupID == defaultGroupID {
			overview.UngroupedTagCount++
		}
		if len(item.LibraryIDs) > 1 {
			overview.CrossLibraryTagCount++
		}

		group := groupStats[item.GroupID]
		if group == nil {
			continue
		}
		group.TagCount++
		if item.UsageCount > 0 {
			group.UsedTagCount++
		}
	}

	groupRecords := make([]tagdto.GroupRecord, 0, len(groups))
	for _, group := range groups {
		if value, ok := groupStats[group.ID]; ok {
			groupRecords = append(groupRecords, *value)
		}
	}

	return tagdto.ManagementSnapshot{
		Overview:  overview,
		Groups:    groupRecords,
		Tags:      tags,
		Libraries: libraries,
	}, nil
}

func (s *Service) ListSuggestions(ctx context.Context, searchText string, libraryID *string) ([]tagdto.Suggestion, error) {
	if err := s.ensureDefaultGroup(ctx, s.pool); err != nil {
		return nil, err
	}

	args := []any{}
	clauses := []string{}
	if keyword := strings.ToLower(strings.TrimSpace(searchText)); keyword != "" {
		args = append(args, "%"+keyword+"%")
		clauses = append(clauses, "LOWER(t.name) LIKE $"+strconv.Itoa(len(args)))
	}

	query := `
		SELECT
			t.id,
			t.name,
			t.usage_count,
			g.name,
			t.is_pinned,
			COALESCE(ARRAY(
				SELECT tls.library_id
				FROM tag_library_scopes tls
				WHERE tls.tag_id = t.id
				ORDER BY tls.library_id
			), ARRAY[]::TEXT[])
		FROM tags t
		INNER JOIN tag_groups g ON g.id = t.group_id
	`
	if libraryID != nil && strings.TrimSpace(*libraryID) != "" {
		args = append(args, strings.TrimSpace(*libraryID))
		clauses = append(clauses, "EXISTS (SELECT 1 FROM tag_library_scopes tls WHERE tls.tag_id = t.id AND tls.library_id = $"+itoa(len(args))+")")
	}
	if len(clauses) > 0 {
		query += "\nWHERE " + strings.Join(clauses, " AND ")
	}
	query += "\nORDER BY t.is_pinned DESC, t.usage_count DESC, t.name ASC"

	rows, err := s.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	items := make([]tagdto.Suggestion, 0)
	for rows.Next() {
		var item tagdto.Suggestion
		if err := rows.Scan(&item.ID, &item.Name, &item.Count, &item.GroupName, &item.IsPinned, &item.LibraryIDs); err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func (s *Service) CreateGroup(ctx context.Context, request tagdto.CreateGroupRequest) (tagdto.CreateGroupResponse, error) {
	name := strings.TrimSpace(request.Name)
	if name == "" {
		return tagdto.CreateGroupResponse{}, apperrors.BadRequest("请输入分组名称")
	}

	if err := s.ensureDefaultGroup(ctx, s.pool); err != nil {
		return tagdto.CreateGroupResponse{}, err
	}

	var exists string
	err := s.pool.QueryRow(ctx, `SELECT id FROM tag_groups WHERE LOWER(name) = LOWER($1)`, name).Scan(&exists)
	if err == nil {
		return tagdto.CreateGroupResponse{}, apperrors.BadRequest("分组名称已存在")
	}
	if err != nil && err != pgx.ErrNoRows {
		return tagdto.CreateGroupResponse{}, err
	}

	orderIndex, err := s.nextGroupOrderIndex(ctx, s.pool)
	if err != nil {
		return tagdto.CreateGroupResponse{}, err
	}

	groupID := newID("tag-group")
	now := s.now().UTC()
	if _, err := s.pool.Exec(ctx, `
		INSERT INTO tag_groups (id, name, order_index, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $4)
	`, groupID, name, orderIndex, now); err != nil {
		return tagdto.CreateGroupResponse{}, err
	}

	return tagdto.CreateGroupResponse{Message: "分组已创建", GroupID: groupID}, nil
}

func (s *Service) UpdateGroup(ctx context.Context, id string, request tagdto.UpdateGroupRequest) (tagdto.MutationResponse, error) {
	name := strings.TrimSpace(request.Name)
	if name == "" {
		return tagdto.MutationResponse{}, apperrors.BadRequest("请输入分组名称")
	}

	if err := s.ensureDefaultGroup(ctx, s.pool); err != nil {
		return tagdto.MutationResponse{}, err
	}

	group, err := s.loadGroup(ctx, s.pool, id)
	if err != nil {
		return tagdto.MutationResponse{}, err
	}
	if group.ID == defaultGroupID && name != "未分组" {
		return tagdto.MutationResponse{}, apperrors.BadRequest("未分组名称不可修改")
	}

	var duplicateID string
	err = s.pool.QueryRow(ctx, `
		SELECT id
		FROM tag_groups
		WHERE LOWER(name) = LOWER($1)
		  AND id <> $2
	`, name, id).Scan(&duplicateID)
	if err == nil {
		return tagdto.MutationResponse{}, apperrors.BadRequest("分组名称已存在")
	}
	if err != nil && err != pgx.ErrNoRows {
		return tagdto.MutationResponse{}, err
	}

	if _, err := s.pool.Exec(ctx, `
		UPDATE tag_groups
		SET name = $2,
		    updated_at = $3
		WHERE id = $1
	`, id, name, s.now().UTC()); err != nil {
		return tagdto.MutationResponse{}, err
	}
	return tagdto.MutationResponse{Message: "分组已更新"}, nil
}

func (s *Service) MoveGroup(ctx context.Context, id string, request tagdto.MoveRequest) (tagdto.MutationResponse, error) {
	if request.Direction != "up" && request.Direction != "down" {
		return tagdto.MutationResponse{}, apperrors.BadRequest("分组移动方向无效")
	}
	if err := s.ensureDefaultGroup(ctx, s.pool); err != nil {
		return tagdto.MutationResponse{}, err
	}
	if _, err := s.loadGroup(ctx, s.pool, id); err != nil {
		return tagdto.MutationResponse{}, err
	}
	if err := s.moveGroup(ctx, id, request.Direction); err != nil {
		return tagdto.MutationResponse{}, err
	}
	return tagdto.MutationResponse{Message: "分组顺序已更新"}, nil
}

func (s *Service) CreateTag(ctx context.Context, request tagdto.CreateTagRequest) (tagdto.CreateTagResponse, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return tagdto.CreateTagResponse{}, err
	}
	defer tx.Rollback(ctx)

	if err := s.ensureDefaultGroup(ctx, tx); err != nil {
		return tagdto.CreateTagResponse{}, err
	}

	tagID, err := s.createTag(ctx, tx, request)
	if err != nil {
		return tagdto.CreateTagResponse{}, err
	}

	if err := tx.Commit(ctx); err != nil {
		return tagdto.CreateTagResponse{}, err
	}
	return tagdto.CreateTagResponse{Message: "标签已创建", TagID: tagID}, nil
}

func (s *Service) UpdateTag(ctx context.Context, id string, request tagdto.UpdateTagRequest) (tagdto.MutationResponse, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return tagdto.MutationResponse{}, err
	}
	defer tx.Rollback(ctx)

	if err := s.ensureDefaultGroup(ctx, tx); err != nil {
		return tagdto.MutationResponse{}, err
	}
	if err := s.updateTag(ctx, tx, id, request); err != nil {
		return tagdto.MutationResponse{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return tagdto.MutationResponse{}, err
	}
	return tagdto.MutationResponse{Message: "标签已更新"}, nil
}

func (s *Service) MoveTag(ctx context.Context, id string, request tagdto.MoveRequest) (tagdto.MutationResponse, error) {
	if request.Direction != "up" && request.Direction != "down" {
		return tagdto.MutationResponse{}, apperrors.BadRequest("标签移动方向无效")
	}
	tag, err := s.loadTag(ctx, s.pool, id)
	if err != nil {
		return tagdto.MutationResponse{}, err
	}
	if err := s.moveTag(ctx, tag, request.Direction); err != nil {
		return tagdto.MutationResponse{}, err
	}
	return tagdto.MutationResponse{Message: "标签顺序已更新"}, nil
}

func (s *Service) MergeTag(ctx context.Context, id string, request tagdto.MergeTagRequest) (tagdto.MutationResponse, error) {
	targetID := strings.TrimSpace(request.TargetID)
	if targetID == "" || targetID == id {
		return tagdto.MutationResponse{}, apperrors.BadRequest("合并目标无效")
	}

	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return tagdto.MutationResponse{}, err
	}
	defer tx.Rollback(ctx)

	source, err := s.loadTag(ctx, tx, id)
	if err != nil {
		return tagdto.MutationResponse{}, err
	}
	target, err := s.loadTag(ctx, tx, targetID)
	if err != nil {
		return tagdto.MutationResponse{}, err
	}

	if _, err := tx.Exec(ctx, `
		INSERT INTO tag_library_scopes (tag_id, library_id)
		SELECT $2, tls.library_id
		FROM tag_library_scopes tls
		WHERE tls.tag_id = $1
		ON CONFLICT (tag_id, library_id) DO NOTHING
	`, source.ID, target.ID); err != nil {
		return tagdto.MutationResponse{}, err
	}

	if _, err := tx.Exec(ctx, `
		INSERT INTO asset_tag_links (asset_id, tag_id, order_index, created_at)
		SELECT atl.asset_id, $2, atl.order_index, $3
		FROM asset_tag_links atl
		WHERE atl.tag_id = $1
		ON CONFLICT (asset_id, tag_id) DO NOTHING
	`, source.ID, target.ID, s.now().UTC()); err != nil {
		return tagdto.MutationResponse{}, err
	}
	if _, err := tx.Exec(ctx, `
		INSERT INTO directory_tag_links (directory_id, tag_id, order_index, created_at)
		SELECT dtl.directory_id, $2, dtl.order_index, $3
		FROM directory_tag_links dtl
		WHERE dtl.tag_id = $1
		ON CONFLICT (directory_id, tag_id) DO NOTHING
	`, source.ID, target.ID, s.now().UTC()); err != nil {
		return tagdto.MutationResponse{}, err
	}

	if source.IsPinned && !target.IsPinned {
		orderIndex, err := s.nextTagOrderIndex(ctx, tx, target.GroupID, true)
		if err != nil {
			return tagdto.MutationResponse{}, err
		}
		if _, err := tx.Exec(ctx, `
			UPDATE tags
			SET is_pinned = TRUE,
			    order_index = $2,
			    updated_at = $3
			WHERE id = $1
		`, target.ID, orderIndex, s.now().UTC()); err != nil {
			return tagdto.MutationResponse{}, err
		}
	}

	if _, err := tx.Exec(ctx, `DELETE FROM tags WHERE id = $1`, source.ID); err != nil {
		return tagdto.MutationResponse{}, err
	}
	if err := s.refreshTagUsageCounts(ctx, tx, []string{target.ID}); err != nil {
		return tagdto.MutationResponse{}, err
	}

	if err := tx.Commit(ctx); err != nil {
		return tagdto.MutationResponse{}, err
	}
	return tagdto.MutationResponse{Message: "标签已合并"}, nil
}

func (s *Service) DeleteTag(ctx context.Context, id string) (tagdto.MutationResponse, error) {
	tag, err := s.loadTag(ctx, s.pool, id)
	if err != nil {
		return tagdto.MutationResponse{}, err
	}
	if tag.ID == "" {
		return tagdto.MutationResponse{}, apperrors.NotFound("未找到标签")
	}

	if _, err := s.pool.Exec(ctx, `DELETE FROM tags WHERE id = $1`, id); err != nil {
		return tagdto.MutationResponse{}, err
	}
	return tagdto.MutationResponse{Message: "标签已删除"}, nil
}

func (s *Service) createTag(ctx context.Context, executor dbExecutor, request tagdto.CreateTagRequest) (string, error) {
	name, normalizedName, err := normalizeTagName(request.Name)
	if err != nil {
		return "", err
	}
	group, err := s.loadGroup(ctx, executor, request.GroupID)
	if err != nil {
		return "", err
	}
	_ = group

	var duplicateID string
	err = executor.QueryRow(ctx, `SELECT id FROM tags WHERE normalized_name = $1`, normalizedName).Scan(&duplicateID)
	if err == nil {
		return "", apperrors.BadRequest("标签名称已存在")
	}
	if err != nil && err != pgx.ErrNoRows {
		return "", err
	}

	libraryIDs, err := s.normalizeLibraryIDs(ctx, executor, request.LibraryIDs)
	if err != nil {
		return "", err
	}
	orderIndex, err := s.nextTagOrderIndex(ctx, executor, request.GroupID, request.IsPinned)
	if err != nil {
		return "", err
	}

	tagID := newID("tag")
	now := s.now().UTC()
	if _, err := executor.Exec(ctx, `
		INSERT INTO tags (id, name, normalized_name, group_id, order_index, is_pinned, usage_count, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, 0, $7, $7)
	`, tagID, name, normalizedName, request.GroupID, orderIndex, request.IsPinned, now); err != nil {
		return "", err
	}
	for _, libraryID := range libraryIDs {
		if _, err := executor.Exec(ctx, `
			INSERT INTO tag_library_scopes (tag_id, library_id)
			VALUES ($1, $2)
		`, tagID, libraryID); err != nil {
			return "", err
		}
	}
	return tagID, nil
}

func (s *Service) updateTag(ctx context.Context, executor dbExecutor, id string, request tagdto.UpdateTagRequest) error {
	current, err := s.loadTag(ctx, executor, id)
	if err != nil {
		return err
	}
	name, normalizedName, err := normalizeTagName(request.Name)
	if err != nil {
		return err
	}
	if _, err := s.loadGroup(ctx, executor, request.GroupID); err != nil {
		return err
	}

	var duplicateID string
	err = executor.QueryRow(ctx, `
		SELECT id
		FROM tags
		WHERE normalized_name = $1
		  AND id <> $2
	`, normalizedName, id).Scan(&duplicateID)
	if err == nil {
		return apperrors.BadRequest("标签名称已存在")
	}
	if err != nil && err != pgx.ErrNoRows {
		return err
	}

	libraryIDs, err := s.normalizeLibraryIDs(ctx, executor, request.LibraryIDs)
	if err != nil {
		return err
	}

	orderIndex := current.OrderIndex
	if current.GroupID != request.GroupID || current.IsPinned != request.IsPinned {
		orderIndex, err = s.nextTagOrderIndex(ctx, executor, request.GroupID, request.IsPinned)
		if err != nil {
			return err
		}
	}

	if _, err := executor.Exec(ctx, `
		UPDATE tags
		SET name = $2,
		    normalized_name = $3,
		    group_id = $4,
		    order_index = $5,
		    is_pinned = $6,
		    updated_at = $7
		WHERE id = $1
	`, id, name, normalizedName, request.GroupID, orderIndex, request.IsPinned, s.now().UTC()); err != nil {
		return err
	}

	if _, err := executor.Exec(ctx, `DELETE FROM tag_library_scopes WHERE tag_id = $1`, id); err != nil {
		return err
	}
	for _, libraryID := range libraryIDs {
		if _, err := executor.Exec(ctx, `
			INSERT INTO tag_library_scopes (tag_id, library_id)
			VALUES ($1, $2)
		`, id, libraryID); err != nil {
			return err
		}
	}
	return nil
}

func (s *Service) loadLibraries(ctx context.Context) ([]tagdto.LibraryRecord, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT id, name
		FROM libraries
		WHERE status <> 'ARCHIVED'
		ORDER BY created_at ASC
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	items := make([]tagdto.LibraryRecord, 0)
	for rows.Next() {
		var item tagdto.LibraryRecord
		if err := rows.Scan(&item.ID, &item.Name); err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func (s *Service) loadGroups(ctx context.Context) ([]groupRow, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT id, name, order_index
		FROM tag_groups
		ORDER BY order_index ASC, name ASC
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	items := make([]groupRow, 0)
	for rows.Next() {
		var item groupRow
		if err := rows.Scan(&item.ID, &item.Name, &item.OrderIndex); err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func (s *Service) loadTagRecords(ctx context.Context) ([]tagdto.Record, error) {
	baseRows, err := s.pool.Query(ctx, `
		SELECT
			t.id,
			t.name,
			t.normalized_name,
			t.group_id,
			g.name,
			t.order_index,
			t.is_pinned,
			t.usage_count,
			t.created_at,
			t.updated_at
		FROM tags t
		INNER JOIN tag_groups g ON g.id = t.group_id
		ORDER BY g.order_index ASC, t.is_pinned DESC, t.order_index ASC, t.name ASC
	`)
	if err != nil {
		return nil, err
	}
	defer baseRows.Close()

	base := make([]tagRow, 0)
	for baseRows.Next() {
		var row tagRow
		if err := baseRows.Scan(
			&row.ID,
			&row.Name,
			&row.NormalizedName,
			&row.GroupID,
			&row.GroupName,
			&row.OrderIndex,
			&row.IsPinned,
			&row.UsageCount,
			&row.CreatedAt,
			&row.UpdatedAt,
		); err != nil {
			return nil, err
		}
		base = append(base, row)
	}
	if err := baseRows.Err(); err != nil {
		return nil, err
	}

	scopeMap, err := s.loadScopeMap(ctx)
	if err != nil {
		return nil, err
	}
	linkedLibraryMap, outOfScopeMap, err := s.loadUsageMaps(ctx)
	if err != nil {
		return nil, err
	}

	items := make([]tagdto.Record, 0, len(base))
	for _, row := range base {
		libraryIDs := scopeMap[row.ID]
		if libraryIDs == nil {
			libraryIDs = []string{}
		}
		linkedLibraryIDs := linkedLibraryMap[row.ID]
		if linkedLibraryIDs == nil {
			linkedLibraryIDs = []string{}
		}
		items = append(items, tagdto.Record{
			ID:                   row.ID,
			Name:                 row.Name,
			NormalizedName:       row.NormalizedName,
			GroupID:              row.GroupID,
			GroupName:            row.GroupName,
			OrderIndex:           row.OrderIndex,
			IsPinned:             row.IsPinned,
			UsageCount:           row.UsageCount,
			LibraryIDs:           libraryIDs,
			LinkedLibraryIDs:     linkedLibraryIDs,
			OutOfScopeUsageCount: outOfScopeMap[row.ID],
			CreatedAt:            row.CreatedAt.UTC().Format("2006-01-02 15:04"),
			UpdatedAt:            row.UpdatedAt.UTC().Format("2006-01-02 15:04"),
		})
	}
	return items, nil
}

func (s *Service) loadScopeMap(ctx context.Context) (map[string][]string, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT tag_id, library_id
		FROM tag_library_scopes
		ORDER BY tag_id ASC, library_id ASC
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	result := map[string][]string{}
	for rows.Next() {
		var tagID string
		var libraryID string
		if err := rows.Scan(&tagID, &libraryID); err != nil {
			return nil, err
		}
		result[tagID] = append(result[tagID], libraryID)
	}
	return result, rows.Err()
}

func (s *Service) loadUsageMaps(ctx context.Context) (map[string][]string, map[string]int, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT
			links.tag_id,
			COALESCE(ARRAY_AGG(DISTINCT links.library_id ORDER BY links.library_id), ARRAY[]::TEXT[]),
			COUNT(*) FILTER (WHERE tls.library_id IS NULL)
		FROM (
			SELECT atl.tag_id, a.library_id
			FROM asset_tag_links atl
			INNER JOIN assets a ON a.id = atl.asset_id
			WHERE a.lifecycle_state <> 'DELETED'
			UNION ALL
			SELECT dtl.tag_id, d.library_id
			FROM directory_tag_links dtl
			INNER JOIN library_directories d ON d.id = dtl.directory_id
			WHERE d.status <> 'DELETED'
		) links
		LEFT JOIN tag_library_scopes tls
		  ON tls.tag_id = links.tag_id
		 AND tls.library_id = links.library_id
		GROUP BY links.tag_id
	`)
	if err != nil {
		return nil, nil, err
	}
	defer rows.Close()

	linkedLibraryMap := map[string][]string{}
	outOfScopeMap := map[string]int{}
	for rows.Next() {
		var tagID string
		var linked []string
		var outOfScope int
		if err := rows.Scan(&tagID, &linked, &outOfScope); err != nil {
			return nil, nil, err
		}
		linkedLibraryMap[tagID] = linked
		outOfScopeMap[tagID] = outOfScope
	}
	if err := rows.Err(); err != nil {
		return nil, nil, err
	}
	return linkedLibraryMap, outOfScopeMap, nil
}

func (s *Service) normalizeLibraryIDs(ctx context.Context, executor dbExecutor, ids []string) ([]string, error) {
	seen := map[string]struct{}{}
	items := make([]string, 0, len(ids))
	for _, id := range ids {
		trimmed := strings.TrimSpace(id)
		if trimmed == "" {
			continue
		}
		if _, ok := seen[trimmed]; ok {
			continue
		}
		var exists string
		if err := executor.QueryRow(ctx, `
			SELECT id
			FROM libraries
			WHERE id = $1
			  AND status <> 'ARCHIVED'
		`, trimmed).Scan(&exists); err != nil {
			if err == pgx.ErrNoRows {
				return nil, apperrors.BadRequest("标签作用资产库不存在")
			}
			return nil, err
		}
		seen[trimmed] = struct{}{}
		items = append(items, trimmed)
	}
	return items, nil
}

func (s *Service) loadGroup(ctx context.Context, executor dbExecutor, id string) (groupRow, error) {
	var row groupRow
	err := executor.QueryRow(ctx, `
		SELECT id, name, order_index
		FROM tag_groups
		WHERE id = $1
	`, id).Scan(&row.ID, &row.Name, &row.OrderIndex)
	if err != nil {
		if err == pgx.ErrNoRows {
			return groupRow{}, apperrors.NotFound("未找到标签分组")
		}
		return groupRow{}, err
	}
	return row, nil
}

func (s *Service) loadTag(ctx context.Context, executor dbExecutor, id string) (tagRow, error) {
	var row tagRow
	err := executor.QueryRow(ctx, `
		SELECT
			t.id,
			t.name,
			t.normalized_name,
			t.group_id,
			g.name,
			t.order_index,
			t.is_pinned,
			t.usage_count,
			t.created_at,
			t.updated_at
		FROM tags t
		INNER JOIN tag_groups g ON g.id = t.group_id
		WHERE t.id = $1
	`, id).Scan(
		&row.ID,
		&row.Name,
		&row.NormalizedName,
		&row.GroupID,
		&row.GroupName,
		&row.OrderIndex,
		&row.IsPinned,
		&row.UsageCount,
		&row.CreatedAt,
		&row.UpdatedAt,
	)
	if err != nil {
		if err == pgx.ErrNoRows {
			return tagRow{}, apperrors.NotFound("未找到标签")
		}
		return tagRow{}, err
	}
	return row, nil
}

func (s *Service) ensureDefaultGroup(ctx context.Context, executor dbExecutor) error {
	_, err := executor.Exec(ctx, `
		INSERT INTO tag_groups (id, name, order_index, created_at, updated_at)
		VALUES ($1, $2, 0, $3, $3)
		ON CONFLICT (id) DO NOTHING
	`, defaultGroupID, "未分组", s.now().UTC())
	return err
}

func (s *Service) nextGroupOrderIndex(ctx context.Context, executor dbExecutor) (int, error) {
	var value int
	if err := executor.QueryRow(ctx, `
		SELECT COALESCE(MAX(order_index), -1) + 1
		FROM tag_groups
	`).Scan(&value); err != nil {
		return 0, err
	}
	return value, nil
}

func (s *Service) nextTagOrderIndex(ctx context.Context, executor dbExecutor, groupID string, pinned bool) (int, error) {
	var value int
	if err := executor.QueryRow(ctx, `
		SELECT COALESCE(MAX(order_index), -1) + 1
		FROM tags
		WHERE group_id = $1
		  AND is_pinned = $2
	`, groupID, pinned).Scan(&value); err != nil {
		return 0, err
	}
	return value, nil
}

func (s *Service) moveGroup(ctx context.Context, id string, direction string) error {
	rows, err := s.pool.Query(ctx, `
		SELECT id, order_index
		FROM tag_groups
		ORDER BY order_index ASC, name ASC
	`)
	if err != nil {
		return err
	}
	defer rows.Close()

	ordered := make([]groupRow, 0)
	for rows.Next() {
		var row groupRow
		if err := rows.Scan(&row.ID, &row.OrderIndex); err != nil {
			return err
		}
		ordered = append(ordered, row)
	}
	if err := rows.Err(); err != nil {
		return err
	}

	index := findGroupIndex(ordered, id)
	if index < 0 {
		return apperrors.NotFound("未找到标签分组")
	}
	swapIndex := index - 1
	if direction == "down" {
		swapIndex = index + 1
	}
	if swapIndex < 0 || swapIndex >= len(ordered) {
		return nil
	}

	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	now := s.now().UTC()
	if _, err := tx.Exec(ctx, `
		UPDATE tag_groups
		SET order_index = $2,
		    updated_at = $3
		WHERE id = $1
	`, ordered[index].ID, ordered[swapIndex].OrderIndex, now); err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, `
		UPDATE tag_groups
		SET order_index = $2,
		    updated_at = $3
		WHERE id = $1
	`, ordered[swapIndex].ID, ordered[index].OrderIndex, now); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func (s *Service) moveTag(ctx context.Context, tag tagRow, direction string) error {
	rows, err := s.pool.Query(ctx, `
		SELECT id, order_index
		FROM tags
		WHERE group_id = $1
		  AND is_pinned = $2
		ORDER BY order_index ASC, name ASC
	`, tag.GroupID, tag.IsPinned)
	if err != nil {
		return err
	}
	defer rows.Close()

	ordered := make([]tagRow, 0)
	for rows.Next() {
		var row tagRow
		if err := rows.Scan(&row.ID, &row.OrderIndex); err != nil {
			return err
		}
		ordered = append(ordered, row)
	}
	if err := rows.Err(); err != nil {
		return err
	}

	index := findTagIndex(ordered, tag.ID)
	if index < 0 {
		return apperrors.NotFound("未找到标签")
	}
	swapIndex := index - 1
	if direction == "down" {
		swapIndex = index + 1
	}
	if swapIndex < 0 || swapIndex >= len(ordered) {
		return nil
	}

	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	now := s.now().UTC()
	if _, err := tx.Exec(ctx, `
		UPDATE tags
		SET order_index = $2,
		    updated_at = $3
		WHERE id = $1
	`, ordered[index].ID, ordered[swapIndex].OrderIndex, now); err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, `
		UPDATE tags
		SET order_index = $2,
		    updated_at = $3
		WHERE id = $1
	`, ordered[swapIndex].ID, ordered[index].OrderIndex, now); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func (s *Service) refreshTagUsageCounts(ctx context.Context, executor dbExecutor, tagIDs []string) error {
	seen := map[string]struct{}{}
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
		if err := executor.QueryRow(ctx, `
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
		if _, err := executor.Exec(ctx, `
			UPDATE tags
			SET usage_count = $2,
			    updated_at = $3
			WHERE id = $1
		`, trimmed, count, s.now().UTC()); err != nil {
			return err
		}
	}
	return nil
}

func normalizeTagName(value string) (string, string, error) {
	name := strings.TrimSpace(value)
	if name == "" {
		return "", "", apperrors.BadRequest("请输入标签名称")
	}
	return name, strings.ToLower(name), nil
}

func newID(prefix string) string {
	buf := make([]byte, 8)
	_, _ = rand.Read(buf)
	return prefix + "-" + hex.EncodeToString(buf)
}

func itoa(value int) string {
	return strconv.Itoa(value)
}

func findGroupIndex(items []groupRow, id string) int {
	for index, item := range items {
		if item.ID == id {
			return index
		}
	}
	return -1
}

func findTagIndex(items []tagRow, id string) int {
	for index, item := range items {
		if item.ID == id {
			return index
		}
	}
	return -1
}
