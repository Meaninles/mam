package assets

import (
	"context"
	"fmt"
	"io"
	"net"
	"os"
	pathpkg "path"
	"strings"
	"time"

	"github.com/hirochachacha/go-smb2"
)

type pathExecutionContext struct {
	PhysicalPath     string
	Username         string
	SecretCiphertext string
	FileContent      []byte
}

type fileMetadata struct {
	SizeBytes  int64
	ModifiedAt time.Time
}

type mountPathExecutor interface {
	EnsureDirectory(ctx context.Context, input pathExecutionContext) error
	WriteFile(ctx context.Context, input pathExecutionContext) error
	WriteStream(ctx context.Context, input pathExecutionContext, reader io.Reader) error
	DeleteFile(ctx context.Context, input pathExecutionContext) error
	DeleteDirectory(ctx context.Context, input pathExecutionContext) error
	StreamFile(ctx context.Context, input pathExecutionContext, consume func(reader io.Reader) error) error
	StatFile(ctx context.Context, input pathExecutionContext) (fileMetadata, error)
	SetFileModifiedTime(ctx context.Context, input pathExecutionContext, modifiedAt time.Time) error
}

type localPathExecutor struct{}

func (localPathExecutor) EnsureDirectory(ctx context.Context, input pathExecutionContext) error {
	_ = ctx
	return ensureLocalDirectory(input.PhysicalPath)
}

func (localPathExecutor) WriteFile(ctx context.Context, input pathExecutionContext) error {
	_ = ctx
	return writeLocalFile(input.PhysicalPath, input.FileContent)
}

func (localPathExecutor) WriteStream(ctx context.Context, input pathExecutionContext, reader io.Reader) error {
	_ = ctx
	return writeLocalStream(input.PhysicalPath, reader)
}

func (localPathExecutor) DeleteFile(ctx context.Context, input pathExecutionContext) error {
	_ = ctx
	return deleteLocalFile(input.PhysicalPath)
}

func (localPathExecutor) DeleteDirectory(ctx context.Context, input pathExecutionContext) error {
	_ = ctx
	return deleteLocalDirectory(input.PhysicalPath)
}

func (localPathExecutor) StreamFile(ctx context.Context, input pathExecutionContext, consume func(reader io.Reader) error) error {
	_ = ctx
	return streamLocalFile(input.PhysicalPath, consume)
}

func (localPathExecutor) StatFile(ctx context.Context, input pathExecutionContext) (fileMetadata, error) {
	_ = ctx
	return statLocalFile(input.PhysicalPath)
}

func (localPathExecutor) SetFileModifiedTime(ctx context.Context, input pathExecutionContext, modifiedAt time.Time) error {
	_ = ctx
	return setLocalFileModifiedTime(input.PhysicalPath, modifiedAt)
}

type nasPathExecutor struct {
	timeout time.Duration
	cipher  credentialCipher
}

func newNASPathExecutor() nasPathExecutor {
	return nasPathExecutor{
		timeout: 5 * time.Second,
		cipher:  newSystemCredentialCipher(),
	}
}

func (n nasPathExecutor) EnsureDirectory(ctx context.Context, input pathExecutionContext) error {
	return n.withShare(ctx, input, func(share *smb2.Share, relativePath string) error {
		if relativePath == "" {
			return nil
		}
		return share.MkdirAll(relativePath, 0o755)
	})
}

func (n nasPathExecutor) WriteFile(ctx context.Context, input pathExecutionContext) error {
	return n.withShare(ctx, input, func(share *smb2.Share, relativePath string) error {
		if relativePath == "" {
			return fmt.Errorf("missing SMB relative path")
		}
		parent := pathpkg.Dir(relativePath)
		if parent != "." && parent != "" {
			if err := share.MkdirAll(parent, 0o755); err != nil {
				return err
			}
		}
		file, err := share.OpenFile(relativePath, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o644)
		if err != nil {
			return err
		}
		defer file.Close()
		_, err = file.Write(input.FileContent)
		return err
	})
}

func (n nasPathExecutor) WriteStream(ctx context.Context, input pathExecutionContext, reader io.Reader) error {
	return n.withShare(ctx, input, func(share *smb2.Share, relativePath string) error {
		if relativePath == "" {
			return fmt.Errorf("missing SMB relative path")
		}
		parent := pathpkg.Dir(relativePath)
		if parent != "." && parent != "" {
			if err := share.MkdirAll(parent, 0o755); err != nil {
				return err
			}
		}
		file, err := share.OpenFile(relativePath, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o644)
		if err != nil {
			return err
		}
		defer file.Close()
		_, err = io.Copy(file, reader)
		return err
	})
}

func (n nasPathExecutor) DeleteFile(ctx context.Context, input pathExecutionContext) error {
	return n.withShare(ctx, input, func(share *smb2.Share, relativePath string) error {
		if relativePath == "" {
			return fmt.Errorf("missing SMB relative path")
		}
		return share.Remove(relativePath)
	})
}

func (n nasPathExecutor) DeleteDirectory(ctx context.Context, input pathExecutionContext) error {
	return n.withShare(ctx, input, func(share *smb2.Share, relativePath string) error {
		if relativePath == "" {
			return fmt.Errorf("missing SMB relative path")
		}
		return share.RemoveAll(relativePath)
	})
}

func (n nasPathExecutor) StreamFile(ctx context.Context, input pathExecutionContext, consume func(reader io.Reader) error) error {
	return n.withShare(ctx, input, func(share *smb2.Share, relativePath string) error {
		if relativePath == "" {
			return fmt.Errorf("missing SMB relative path")
		}
		file, err := share.Open(relativePath)
		if err != nil {
			return err
		}
		defer file.Close()
		return consume(file)
	})
}

func (n nasPathExecutor) StatFile(ctx context.Context, input pathExecutionContext) (fileMetadata, error) {
	var metadata fileMetadata
	err := n.withShare(ctx, input, func(share *smb2.Share, relativePath string) error {
		if relativePath == "" {
			return fmt.Errorf("missing SMB relative path")
		}
		info, err := share.Stat(relativePath)
		if err != nil {
			return err
		}
		metadata = fileMetadata{
			SizeBytes:  info.Size(),
			ModifiedAt: info.ModTime().UTC(),
		}
		return nil
	})
	return metadata, err
}

func (n nasPathExecutor) SetFileModifiedTime(ctx context.Context, input pathExecutionContext, modifiedAt time.Time) error {
	return n.withShare(ctx, input, func(share *smb2.Share, relativePath string) error {
		if relativePath == "" {
			return fmt.Errorf("missing SMB relative path")
		}
		return share.Chtimes(relativePath, modifiedAt, modifiedAt)
	})
}

func (n nasPathExecutor) withShare(ctx context.Context, input pathExecutionContext, run func(share *smb2.Share, relativePath string) error) error {
	password, err := n.cipher.Decrypt(input.SecretCiphertext)
	if err != nil {
		return fmt.Errorf("decrypt nas credential: %w", err)
	}

	target, err := parseSMBPath(input.PhysicalPath)
	if err != nil {
		return err
	}

	dialer := net.Dialer{Timeout: n.timeout}
	tcpConn, err := dialer.DialContext(ctx, "tcp", target.Endpoint)
	if err != nil {
		return err
	}
	defer tcpConn.Close()

	session, err := (&smb2.Dialer{
		Initiator: &smb2.NTLMInitiator{
			User:     input.Username,
			Password: password,
		},
	}).DialContext(ctx, tcpConn)
	if err != nil {
		return err
	}
	defer session.Logoff()

	share, err := session.Mount(target.Share)
	if err != nil {
		return err
	}
	defer share.Umount()

	return run(share, target.RelativePath)
}

type smbPathTarget struct {
	Endpoint     string
	Share        string
	RelativePath string
}

func parseSMBPath(raw string) (smbPathTarget, error) {
	value := strings.TrimSpace(raw)
	if value == "" {
		return smbPathTarget{}, fmt.Errorf("NAS 路径不能为空")
	}
	value = strings.TrimPrefix(value, "smb://")
	value = strings.TrimPrefix(value, "SMB://")
	value = strings.TrimPrefix(value, `\\`)
	value = strings.TrimPrefix(value, `//`)

	parts := strings.FieldsFunc(value, func(r rune) bool {
		return r == '\\' || r == '/'
	})
	if len(parts) < 2 {
		return smbPathTarget{}, fmt.Errorf("NAS 路径必须包含共享名")
	}

	target := smbPathTarget{
		Endpoint: withSMBPort(parts[0]),
		Share:    parts[1],
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

func executorForNodeType(nodeType string) (mountPathExecutor, error) {
	switch nodeType {
	case "LOCAL":
		return localPathExecutor{}, nil
	case "NAS":
		return newNASPathExecutor(), nil
	default:
		return nil, fmt.Errorf("unsupported node type %s", nodeType)
	}
}
