package storage

import (
	"context"
	"errors"
	"fmt"
	"net"
	"strings"
	"time"

	"github.com/hirochachacha/go-smb2"

	storagedto "mare/shared/contracts/dto/storage"
)

type nasConnectionProbe struct {
	OverallTone      string
	Summary          string
	Suggestion       string
	Checks           []storagedto.ConnectionCheck
	HealthStatus     string
	AuthStatus       string
	LastErrorCode    string
	LastErrorMessage string
}

type nasConnector interface {
	Test(ctx context.Context, address string, username string, password string) (nasConnectionProbe, error)
	EnsureDirectory(ctx context.Context, address string, username string, password string) error
}

type smbConnector struct {
	timeout time.Duration
}

func newSMBConnector(timeout time.Duration) nasConnector {
	return smbConnector{timeout: timeout}
}

func (c smbConnector) Test(ctx context.Context, address string, username string, password string) (nasConnectionProbe, error) {
	target, err := parseSMBAddress(address)
	if err != nil {
		return nasConnectionProbe{
			OverallTone:      "critical",
			Summary:          "NAS 地址格式无效，请改成 SMB 主机或共享目录地址。",
			Suggestion:       "检查 NAS 地址是否为 \\\\主机\\共享 或 smb://主机/共享 形式",
			Checks:           []storagedto.ConnectionCheck{{Label: "地址解析", Status: "critical", Detail: err.Error()}},
			HealthStatus:     "ERROR",
			AuthStatus:       "UNKNOWN",
			LastErrorCode:    "invalid_address",
			LastErrorMessage: err.Error(),
		}, nil
	}

	session, closeFn, err := c.openSession(ctx, target.Endpoint, username, password)
	if err != nil {
		return c.connectionProbeFromError(target.Endpoint, err), nil
	}
	defer closeFn()

	checks := []storagedto.ConnectionCheck{
		{Label: "SMB 端口", Status: "success", Detail: fmt.Sprintf("%s 可达", target.Endpoint)},
	}

	if target.Share == "" {
		shares, err := session.ListSharenames()
		if err != nil {
			return nasConnectionProbe{
				OverallTone: "critical",
				Summary:     "SMB 鉴权通过，但共享列表读取失败。",
				Suggestion:  "检查 NAS 是否允许当前账号枚举共享",
				Checks: append(checks,
					storagedto.ConnectionCheck{Label: "SMB 鉴权", Status: "success", Detail: "账号认证通过"},
					storagedto.ConnectionCheck{Label: "共享列表", Status: "critical", Detail: err.Error()},
				),
				HealthStatus:     "ONLINE",
				AuthStatus:       "AUTHORIZED",
				LastErrorCode:    "share_list_failed",
				LastErrorMessage: err.Error(),
			}, nil
		}

		return nasConnectionProbe{
			OverallTone: "success",
			Summary:     "NAS 连接测试通过，SMB 鉴权正常。",
			Suggestion:  "可继续配置挂载关系",
			Checks: append(checks,
				storagedto.ConnectionCheck{Label: "SMB 鉴权", Status: "success", Detail: "账号认证通过"},
				storagedto.ConnectionCheck{Label: "共享列表", Status: "success", Detail: fmt.Sprintf("可读取 %d 个共享", len(shares))},
			),
			HealthStatus: "ONLINE",
			AuthStatus:   "AUTHORIZED",
		}, nil
	}

	share, err := session.Mount(target.Share)
	if err != nil {
		return nasConnectionProbe{
			OverallTone: "critical",
			Summary:     "SMB 鉴权通过，但共享目录挂载失败。",
			Suggestion:  "检查共享名称、共享权限和 NAS 配置",
			Checks: append(checks,
				storagedto.ConnectionCheck{Label: "SMB 鉴权", Status: "success", Detail: "账号认证通过"},
				storagedto.ConnectionCheck{Label: "共享挂载", Status: "critical", Detail: err.Error()},
			),
			HealthStatus:     "ONLINE",
			AuthStatus:       "FAILED",
			LastErrorCode:    "share_mount_failed",
			LastErrorMessage: err.Error(),
		}, nil
	}
	defer share.Umount()

	readTarget := target.RelativePath
	if readTarget == "" {
		readTarget = "."
	}

	if _, err := share.ReadDir(readTarget); err != nil {
		return nasConnectionProbe{
			OverallTone: "critical",
			Summary:     "共享目录已连接，但目标路径不可读取。",
			Suggestion:  "检查共享子目录是否存在且当前账号具备访问权限",
			Checks: append(checks,
				storagedto.ConnectionCheck{Label: "SMB 鉴权", Status: "success", Detail: "账号认证通过"},
				storagedto.ConnectionCheck{Label: "共享挂载", Status: "success", Detail: fmt.Sprintf("已挂载 %s", target.Share)},
				storagedto.ConnectionCheck{Label: "目录读取", Status: "critical", Detail: err.Error()},
			),
			HealthStatus:     "ONLINE",
			AuthStatus:       "FAILED",
			LastErrorCode:    "path_read_failed",
			LastErrorMessage: err.Error(),
		}, nil
	}

	return nasConnectionProbe{
		OverallTone: "success",
		Summary:     "NAS 连接测试通过，SMB 鉴权正常。",
		Suggestion:  "可继续配置挂载关系",
		Checks: append(checks,
			storagedto.ConnectionCheck{Label: "SMB 鉴权", Status: "success", Detail: "账号认证通过"},
			storagedto.ConnectionCheck{Label: "共享挂载", Status: "success", Detail: fmt.Sprintf("已挂载 %s", target.Share)},
			storagedto.ConnectionCheck{Label: "目录读取", Status: "success", Detail: fmt.Sprintf("%s 可读取", readTarget)},
		),
		HealthStatus: "ONLINE",
		AuthStatus:   "AUTHORIZED",
	}, nil
}

func (c smbConnector) EnsureDirectory(ctx context.Context, address string, username string, password string) error {
	target, err := parseSMBAddress(address)
	if err != nil {
		return fmt.Errorf("parse smb address: %w", err)
	}
	if target.Share == "" {
		return fmt.Errorf("missing SMB share name")
	}

	session, closeFn, err := c.openSession(ctx, target.Endpoint, username, password)
	if err != nil {
		return err
	}
	defer closeFn()

	share, err := session.Mount(target.Share)
	if err != nil {
		return fmt.Errorf("mount share %s: %w", target.Share, err)
	}
	defer share.Umount()

	if target.RelativePath == "" {
		return nil
	}

	if err := share.MkdirAll(target.RelativePath, 0o755); err != nil {
		return fmt.Errorf("create smb directory %s: %w", target.RelativePath, err)
	}
	return nil
}

func (c smbConnector) openSession(ctx context.Context, endpoint string, username string, password string) (*smb2.Session, func(), error) {
	dialer := net.Dialer{Timeout: c.timeout}
	tcpConn, err := dialer.DialContext(ctx, "tcp", endpoint)
	if err != nil {
		return nil, func() {}, err
	}

	session, err := (&smb2.Dialer{
		Initiator: &smb2.NTLMInitiator{
			User:     username,
			Password: password,
		},
	}).DialContext(ctx, tcpConn)
	if err != nil {
		_ = tcpConn.Close()
		return nil, func() {}, err
	}

	closeFn := func() {
		_ = session.Logoff()
		_ = tcpConn.Close()
	}
	return session, closeFn, nil
}

func (c smbConnector) connectionProbeFromError(endpoint string, err error) nasConnectionProbe {
	var netErr net.Error
	if ok := errors.As(err, &netErr); ok {
		return nasConnectionProbe{
			OverallTone:      "critical",
			Summary:          "NAS 无法连通，SMB 端口不可达。",
			Suggestion:       "检查 NAS 主机地址、SMB 服务和网络连通性",
			Checks:           []storagedto.ConnectionCheck{{Label: "SMB 端口", Status: "critical", Detail: err.Error()}},
			HealthStatus:     "OFFLINE",
			AuthStatus:       "UNKNOWN",
			LastErrorCode:    "smb_unreachable",
			LastErrorMessage: err.Error(),
		}
	}

	return nasConnectionProbe{
		OverallTone: "critical",
		Summary:     "NAS 已连通，但 SMB 鉴权失败。",
		Suggestion:  "检查用户名、密码和 NAS 的 SMB 权限配置",
		Checks: []storagedto.ConnectionCheck{
			{Label: "SMB 端口", Status: "success", Detail: fmt.Sprintf("%s 可达", endpoint)},
			{Label: "SMB 鉴权", Status: "critical", Detail: err.Error()},
		},
		HealthStatus:     "ONLINE",
		AuthStatus:       "FAILED",
		LastErrorCode:    "auth_failed",
		LastErrorMessage: err.Error(),
	}
}

type smbAddressTarget struct {
	Endpoint     string
	Share        string
	RelativePath string
}

func parseSMBAddress(raw string) (smbAddressTarget, error) {
	value := strings.TrimSpace(raw)
	if value == "" {
		return smbAddressTarget{}, fmt.Errorf("NAS 地址不能为空")
	}

	value = strings.TrimPrefix(value, "smb://")
	value = strings.TrimPrefix(value, "SMB://")
	value = strings.TrimPrefix(value, `\\`)
	value = strings.TrimPrefix(value, `//`)

	parts := strings.FieldsFunc(value, func(r rune) bool {
		return r == '\\' || r == '/'
	})
	if len(parts) == 0 || strings.TrimSpace(parts[0]) == "" {
		return smbAddressTarget{}, fmt.Errorf("缺少 NAS 主机地址")
	}

	target := smbAddressTarget{
		Endpoint: withSMBPort(parts[0]),
	}
	if len(parts) > 1 {
		target.Share = parts[1]
	}
	if len(parts) > 2 {
		target.RelativePath = strings.Join(parts[2:], "/")
	}

	return target, nil
}

func withSMBPort(host string) string {
	if _, _, err := net.SplitHostPort(host); err == nil {
		return host
	}

	if strings.HasPrefix(host, "[") && strings.Contains(host, "]:") {
		return host
	}

	return net.JoinHostPort(host, "445")
}

func joinSMBPath(root string, relativePath string) string {
	base := strings.TrimRight(strings.TrimSpace(root), `\/`)
	relative := strings.Trim(strings.TrimSpace(relativePath), `\/`)
	if relative == "" {
		return base
	}
	return base + `\` + strings.ReplaceAll(relative, "/", `\`)
}
