package storage

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	apperrors "mare/services/center/internal/errors"
	storagedto "mare/shared/contracts/dto/storage"
)

type NASNodeService struct {
	pool      *pgxpool.Pool
	now       func() time.Time
	connector nasConnector
	cipher    credentialCipher
}

func NewNASNodeService(pool *pgxpool.Pool) *NASNodeService {
	return &NASNodeService{
		pool:      pool,
		now:       time.Now,
		connector: newSMBConnector(5 * time.Second),
		cipher:    newSystemCredentialCipher(),
	}
}

func (s *NASNodeService) ListNasNodes(ctx context.Context) ([]storagedto.NasRecord, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT
			sn.id,
			sn.name,
			sn.address,
			sn.access_mode,
			COALESCE(snc.username, ''),
			snc.updated_at,
			COALESCE(snc.secret_ciphertext, ''),
			COALESCE(snr.auth_status, 'UNKNOWN'),
			COALESCE(snr.health_status, 'UNKNOWN'),
			snr.last_check_at,
			COALESCE(sn.description, ''),
			COUNT(m.id) FILTER (WHERE m.deleted_at IS NULL)
		FROM storage_nodes sn
		LEFT JOIN storage_node_credentials snc ON snc.storage_node_id = sn.id
		LEFT JOIN storage_node_runtime snr ON snr.storage_node_id = sn.id
		LEFT JOIN mounts m ON m.storage_node_id = sn.id
		WHERE sn.node_type = 'NAS'
		  AND sn.deleted_at IS NULL
		GROUP BY
			sn.id, sn.name, sn.address, sn.access_mode,
			snc.username, snc.updated_at, snc.secret_ciphertext,
			snr.auth_status, snr.health_status, snr.last_check_at,
			sn.description
		ORDER BY sn.created_at DESC
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	items := make([]storagedto.NasRecord, 0)
	for rows.Next() {
		var (
			id               string
			name             string
			address          string
			accessMode       string
			username         string
			credentialAt     *time.Time
			secretCiphertext string
			authStatus       string
			healthStatus     string
			lastCheckAt      *time.Time
			notes            string
			mountCount       int
		)

		if err := rows.Scan(
			&id,
			&name,
			&address,
			&accessMode,
			&username,
			&credentialAt,
			&secretCiphertext,
			&authStatus,
			&healthStatus,
			&lastCheckAt,
			&notes,
			&mountCount,
		); err != nil {
			return nil, err
		}

		items = append(items, storagedto.NasRecord{
			ID:           id,
			Name:         name,
			Address:      address,
			AccessMode:   accessMode,
			Username:     username,
			PasswordHint: uiNasPasswordHint(secretCiphertext, credentialAt),
			LastTestAt:   uiNodeLastCheckAt(lastCheckAt),
			Status:       uiNasStatus(authStatus, healthStatus),
			Tone:         uiNasTone(authStatus, healthStatus),
			MountCount:   mountCount,
			Notes:        notes,
		})
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	return items, nil
}

func (s *NASNodeService) SaveNasNode(ctx context.Context, request storagedto.SaveNasNodeRequest) (storagedto.SaveNasNodeResponse, error) {
	request.Name = strings.TrimSpace(request.Name)
	request.Address = strings.TrimSpace(request.Address)
	request.Username = strings.TrimSpace(request.Username)
	request.Password = strings.TrimSpace(request.Password)
	request.Notes = strings.TrimSpace(request.Notes)

	if request.Name == "" {
		return storagedto.SaveNasNodeResponse{}, apperrors.BadRequest("NAS 名称不能为空")
	}
	if request.Address == "" {
		return storagedto.SaveNasNodeResponse{}, apperrors.BadRequest("NAS 地址不能为空")
	}
	if request.Username == "" {
		return storagedto.SaveNasNodeResponse{}, apperrors.BadRequest("NAS 账号不能为空")
	}
	if _, err := parseSMBAddress(request.Address); err != nil {
		return storagedto.SaveNasNodeResponse{}, apperrors.BadRequest(err.Error())
	}

	now := s.now().UTC()
	tx, err := s.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return storagedto.SaveNasNodeResponse{}, err
	}
	defer tx.Rollback(ctx)

	var ciphertext string
	if request.ID == "" {
		if request.Password == "" {
			return storagedto.SaveNasNodeResponse{}, apperrors.BadRequest("NAS 密码不能为空")
		}
		ciphertext, err = s.cipher.Encrypt(request.Password)
		if err != nil {
			return storagedto.SaveNasNodeResponse{}, err
		}

		nodeID := buildCode("nas-node-id", now)
		_, err = tx.Exec(ctx, `
			INSERT INTO storage_nodes (
				id, code, name, node_type, address, access_mode, account_alias, enabled, description, created_at, updated_at
			) VALUES ($1, $2, $3, 'NAS', $4, 'SMB', $5, true, $6, $7, $7)
		`, nodeID, buildCode("nas-node", now), request.Name, request.Address, request.Username, request.Notes, now)
		if err != nil {
			return storagedto.SaveNasNodeResponse{}, err
		}

		_, err = tx.Exec(ctx, `
			INSERT INTO storage_node_credentials (
				id, storage_node_id, credential_kind, username, secret_ciphertext, token_status, updated_at, created_at
			) VALUES ($1, $2, 'USERNAME_PASSWORD', $3, $4, 'UNKNOWN', $5, $5)
		`, buildCode("nas-node-credential", now), nodeID, request.Username, ciphertext, now)
		if err != nil {
			return storagedto.SaveNasNodeResponse{}, err
		}

		_, err = tx.Exec(ctx, `
			INSERT INTO storage_node_runtime (
				id, storage_node_id, health_status, auth_status, created_at, updated_at
			) VALUES ($1, $2, 'UNKNOWN', 'UNKNOWN', $3, $3)
		`, buildCode("nas-node-runtime", now), nodeID, now)
		if err != nil {
			return storagedto.SaveNasNodeResponse{}, err
		}

		request.ID = nodeID
	} else {
		var existingCiphertext string
		err = tx.QueryRow(ctx, `
			SELECT COALESCE(secret_ciphertext, '')
			FROM storage_node_credentials
			WHERE storage_node_id = $1
		`, request.ID).Scan(&existingCiphertext)
		if err != nil && err != pgx.ErrNoRows {
			return storagedto.SaveNasNodeResponse{}, err
		}

		if request.Password != "" {
			ciphertext, err = s.cipher.Encrypt(request.Password)
			if err != nil {
				return storagedto.SaveNasNodeResponse{}, err
			}
		} else {
			ciphertext = existingCiphertext
		}
		if ciphertext == "" {
			return storagedto.SaveNasNodeResponse{}, apperrors.BadRequest("NAS 密码不能为空")
		}

		tag, err := tx.Exec(ctx, `
			UPDATE storage_nodes
			SET name = $2,
			    address = $3,
			    access_mode = 'SMB',
			    account_alias = $4,
			    description = $5,
			    updated_at = $6
			WHERE id = $1
			  AND node_type = 'NAS'
			  AND deleted_at IS NULL
		`, request.ID, request.Name, request.Address, request.Username, request.Notes, now)
		if err != nil {
			return storagedto.SaveNasNodeResponse{}, err
		}
		if tag.RowsAffected() == 0 {
			return storagedto.SaveNasNodeResponse{}, apperrors.NotFound("NAS 节点不存在")
		}

		_, err = tx.Exec(ctx, `
			INSERT INTO storage_node_credentials (
				id, storage_node_id, credential_kind, username, secret_ciphertext, token_status, updated_at, created_at
			) VALUES ($1, $2, 'USERNAME_PASSWORD', $3, $4, 'UNKNOWN', $5, $5)
			ON CONFLICT (storage_node_id) DO UPDATE SET
				credential_kind = EXCLUDED.credential_kind,
				username = EXCLUDED.username,
				secret_ciphertext = EXCLUDED.secret_ciphertext,
				token_status = EXCLUDED.token_status,
				updated_at = EXCLUDED.updated_at
		`, buildCode("nas-node-credential", now), request.ID, request.Username, ciphertext, now)
		if err != nil {
			return storagedto.SaveNasNodeResponse{}, err
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return storagedto.SaveNasNodeResponse{}, err
	}

	record, err := s.loadNasNodeByID(ctx, request.ID)
	if err != nil {
		return storagedto.SaveNasNodeResponse{}, err
	}

	return storagedto.SaveNasNodeResponse{
		Message: "NAS 已保存",
		Record:  record,
	}, nil
}

func (s *NASNodeService) RunNasNodeConnectionTest(ctx context.Context, ids []string) (storagedto.RunNasNodeConnectionTestResponse, error) {
	if len(ids) == 0 {
		return storagedto.RunNasNodeConnectionTestResponse{}, apperrors.BadRequest("ids 不能为空")
	}

	now := s.now().UTC()
	results := make([]storagedto.ConnectionTestResult, 0, len(ids))
	for _, id := range ids {
		config, err := s.loadNasNodeConnectionConfig(ctx, id)
		if err != nil {
			return storagedto.RunNasNodeConnectionTestResponse{}, err
		}

		password, err := s.cipher.Decrypt(config.SecretCiphertext)
		if err != nil {
			probe := nasConnectionProbe{
				OverallTone:      "critical",
				Summary:          "NAS 凭据无法读取，请重新保存账号和密码。",
				Suggestion:       "重新编辑 NAS 节点并保存正确密码",
				Checks:           []storagedto.ConnectionCheck{{Label: "凭据读取", Status: "critical", Detail: err.Error()}},
				HealthStatus:     "ERROR",
				AuthStatus:       "FAILED",
				LastErrorCode:    "credential_unreadable",
				LastErrorMessage: err.Error(),
			}
			if _, updateErr := s.pool.Exec(ctx, `
				UPDATE storage_node_runtime
				SET health_status = $2,
				    auth_status = $3,
				    last_check_at = $4,
				    last_error_code = $5,
				    last_error_message = $6,
				    updated_at = $4
				WHERE storage_node_id = $1
			`, id, probe.HealthStatus, probe.AuthStatus, now, probe.LastErrorCode, probe.LastErrorMessage); updateErr != nil {
				return storagedto.RunNasNodeConnectionTestResponse{}, updateErr
			}
			results = append(results, storagedto.ConnectionTestResult{
				ID:          id,
				Name:        config.Name,
				OverallTone: probe.OverallTone,
				Summary:     probe.Summary,
				Checks:      probe.Checks,
				Suggestion:  probe.Suggestion,
				TestedAt:    "刚刚",
			})
			continue
		}

		probe, err := s.connector.Test(ctx, config.Address, config.Username, password)
		if err != nil {
			return storagedto.RunNasNodeConnectionTestResponse{}, err
		}

		_, err = s.pool.Exec(ctx, `
			UPDATE storage_node_runtime
			SET health_status = $2,
			    auth_status = $3,
			    last_check_at = $4,
			    last_success_at = CASE WHEN $5 THEN $4 ELSE last_success_at END,
			    last_error_code = NULLIF($6, ''),
			    last_error_message = NULLIF($7, ''),
			    updated_at = $4
			WHERE storage_node_id = $1
		`, id, probe.HealthStatus, probe.AuthStatus, now, probe.OverallTone == "success", probe.LastErrorCode, probe.LastErrorMessage)
		if err != nil {
			return storagedto.RunNasNodeConnectionTestResponse{}, err
		}

		results = append(results, storagedto.ConnectionTestResult{
			ID:          id,
			Name:        config.Name,
			OverallTone: probe.OverallTone,
			Summary:     probe.Summary,
			Checks:      probe.Checks,
			Suggestion:  probe.Suggestion,
			TestedAt:    "刚刚",
		})
	}

	message := "连接测试已完成"
	if len(ids) > 1 {
		message = fmt.Sprintf("已完成 %d 个 NAS 的连接测试", len(ids))
	}

	return storagedto.RunNasNodeConnectionTestResponse{
		Message: message,
		Results: results,
	}, nil
}

func (s *NASNodeService) DeleteNasNode(ctx context.Context, id string) (storagedto.DeleteNasNodeResponse, error) {
	var mountCount int
	if err := s.pool.QueryRow(ctx, `
		SELECT COUNT(1)
		FROM mounts
		WHERE storage_node_id = $1
		  AND deleted_at IS NULL
	`, id).Scan(&mountCount); err != nil {
		return storagedto.DeleteNasNodeResponse{}, err
	}
	if mountCount > 0 {
		return storagedto.DeleteNasNodeResponse{}, apperrors.BadRequest("当前 NAS 节点下仍存在挂载，请先删除挂载")
	}

	tag, err := s.pool.Exec(ctx, `
		UPDATE storage_nodes
		SET deleted_at = $2,
		    updated_at = $2
		WHERE id = $1
		  AND node_type = 'NAS'
		  AND deleted_at IS NULL
	`, id, s.now().UTC())
	if err != nil {
		return storagedto.DeleteNasNodeResponse{}, err
	}
	if tag.RowsAffected() == 0 {
		return storagedto.DeleteNasNodeResponse{}, apperrors.NotFound("NAS 节点不存在")
	}

	return storagedto.DeleteNasNodeResponse{Message: "NAS 已删除"}, nil
}

type nasNodeConnectionConfig struct {
	ID               string
	Name             string
	Address          string
	Username         string
	SecretCiphertext string
}

func (s *NASNodeService) loadNasNodeByID(ctx context.Context, id string) (storagedto.NasRecord, error) {
	items, err := s.ListNasNodes(ctx)
	if err != nil {
		return storagedto.NasRecord{}, err
	}
	for _, item := range items {
		if item.ID == id {
			return item, nil
		}
	}
	return storagedto.NasRecord{}, apperrors.NotFound("NAS 节点不存在")
}

func (s *NASNodeService) loadNasNodeConnectionConfig(ctx context.Context, id string) (nasNodeConnectionConfig, error) {
	var config nasNodeConnectionConfig
	err := s.pool.QueryRow(ctx, `
		SELECT
			sn.id,
			sn.name,
			sn.address,
			COALESCE(snc.username, ''),
			COALESCE(snc.secret_ciphertext, '')
		FROM storage_nodes sn
		INNER JOIN storage_node_credentials snc ON snc.storage_node_id = sn.id
		WHERE sn.id = $1
		  AND sn.node_type = 'NAS'
		  AND sn.deleted_at IS NULL
	`, id).Scan(&config.ID, &config.Name, &config.Address, &config.Username, &config.SecretCiphertext)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nasNodeConnectionConfig{}, apperrors.NotFound("NAS 节点不存在")
		}
		return nasNodeConnectionConfig{}, err
	}

	if strings.TrimSpace(config.SecretCiphertext) == "" {
		return nasNodeConnectionConfig{}, apperrors.BadRequest("NAS 凭据缺失，请先重新保存账号和密码")
	}

	return config, nil
}

func uiNasPasswordHint(secretCiphertext string, updatedAt *time.Time) string {
	if strings.TrimSpace(secretCiphertext) == "" {
		return "未配置"
	}
	if updatedAt == nil {
		return "已保存"
	}
	return fmt.Sprintf("已保存，最近更新于 %s", updatedAt.Format("2006-01-02 15:04"))
}

func uiNasStatus(authStatus string, healthStatus string) string {
	switch {
	case authStatus == "AUTHORIZED" && healthStatus == "ONLINE":
		return "鉴权正常"
	case authStatus == "FAILED":
		return "鉴权异常"
	case healthStatus == "OFFLINE":
		return "连接异常"
	case healthStatus == "ERROR":
		return "状态异常"
	default:
		return "待检测"
	}
}

func uiNasTone(authStatus string, healthStatus string) string {
	switch {
	case authStatus == "AUTHORIZED" && healthStatus == "ONLINE":
		return "success"
	case authStatus == "FAILED" || healthStatus == "ERROR" || healthStatus == "OFFLINE":
		return "critical"
	default:
		return "info"
	}
}
