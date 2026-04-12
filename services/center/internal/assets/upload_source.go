package assets

import (
	"context"
	"fmt"
	"io"
	"net"
	"os"

	"github.com/hirochachacha/go-smb2"

	"mare/services/center/internal/integration"
)

type localUploadSource struct {
	file *os.File
	info os.FileInfo
}

func openLocalUploadSource(physicalPath string) (integration.UploadSource, error) {
	file, err := os.Open(physicalPath)
	if err != nil {
		return nil, err
	}
	info, err := file.Stat()
	if err != nil {
		file.Close()
		return nil, err
	}
	return &localUploadSource{file: file, info: info}, nil
}

func (s *localUploadSource) Size() int64 {
	return s.info.Size()
}

func (s *localUploadSource) ReadChunk(ctx context.Context, offset int64, length int64) ([]byte, bool, error) {
	_ = ctx
	buffer := make([]byte, length)
	n, err := s.file.ReadAt(buffer, offset)
	if err != nil && err != io.EOF {
		return nil, false, err
	}
	return buffer[:n], offset+int64(n) >= s.info.Size(), nil
}

func (s *localUploadSource) Close() error {
	return s.file.Close()
}

type nasUploadSource struct {
	session *smb2.Session
	share   *smb2.Share
	file    *smb2.File
	size    int64
}

func openNASUploadSource(ctx context.Context, physicalPath string, username string, secretCiphertext string) (integration.UploadSource, error) {
	cipher := newSystemCredentialCipher()
	password, err := cipher.Decrypt(secretCiphertext)
	if err != nil {
		return nil, err
	}
	target, err := parseSMBPath(physicalPath)
	if err != nil {
		return nil, err
	}
	tcpConn, err := (&net.Dialer{}).DialContext(ctx, "tcp", target.Endpoint)
	if err != nil {
		return nil, err
	}
	session, err := (&smb2.Dialer{
		Initiator: &smb2.NTLMInitiator{User: username, Password: password},
	}).DialContext(ctx, tcpConn)
	if err != nil {
		tcpConn.Close()
		return nil, err
	}
	share, err := session.Mount(target.Share)
	if err != nil {
		session.Logoff()
		return nil, err
	}
	file, err := share.Open(target.RelativePath)
	if err != nil {
		share.Umount()
		session.Logoff()
		return nil, err
	}
	info, err := file.Stat()
	if err != nil {
		file.Close()
		share.Umount()
		session.Logoff()
		return nil, err
	}
	return &nasUploadSource{
		session: session,
		share:   share,
		file:    file,
		size:    info.Size(),
	}, nil
}

func (s *nasUploadSource) Size() int64 {
	return s.size
}

func (s *nasUploadSource) ReadChunk(ctx context.Context, offset int64, length int64) ([]byte, bool, error) {
	_ = ctx
	buffer := make([]byte, length)
	n, err := s.file.ReadAt(buffer, offset)
	if err != nil && err != io.EOF {
		return nil, false, err
	}
	return buffer[:n], offset+int64(n) >= s.size, nil
}

func (s *nasUploadSource) Close() error {
	if s.file != nil {
		_ = s.file.Close()
	}
	if s.share != nil {
		_ = s.share.Umount()
	}
	if s.session != nil {
		_ = s.session.Logoff()
	}
	return nil
}

func (s *Service) openUploadSource(ctx context.Context, replica operationReplica) (integration.UploadSource, error) {
	switch replica.NodeType {
	case "LOCAL":
		return openLocalUploadSource(replica.PhysicalPath)
	case "NAS":
		return openNASUploadSource(ctx, replica.PhysicalPath, replica.Username, replica.SecretCiphertext)
	default:
		return nil, fmt.Errorf("当前副本类型暂不支持作为上传源")
	}
}
