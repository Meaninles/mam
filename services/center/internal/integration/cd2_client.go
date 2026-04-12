package integration

import (
	"context"
	"fmt"
	"net/url"
	"strings"
	"time"

	cd2pb "mare/services/center/internal/integration/cd2/pb"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/metadata"
	emptypb "google.golang.org/protobuf/types/known/emptypb"
)

type cd2Client struct {
	config      CD2GatewayConfig
	serverURL   *url.URL
	conn        *grpc.ClientConn
	client      cd2pb.CloudDriveFileSrvClient
	token       string
	tokenExpiry time.Time
}

func newCD2Client(ctx context.Context, config CD2GatewayConfig) (*cd2Client, error) {
	if strings.TrimSpace(config.BaseURL) == "" {
		return nil, fmt.Errorf("CloudDrive2 地址未配置")
	}
	if !config.Enabled {
		return nil, fmt.Errorf("CloudDrive2 当前已禁用")
	}
	if strings.TrimSpace(config.Username) == "" || strings.TrimSpace(config.Password) == "" {
		return nil, fmt.Errorf("CloudDrive2 账号或密码未配置")
	}

	serverURL, err := parseGatewayURL(config.BaseURL)
	if err != nil {
		return nil, err
	}

	dialCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	conn, err := grpc.DialContext(
		dialCtx,
		serverURL.Host,
		grpc.WithTransportCredentials(resolveTransportCredentials(serverURL)),
		grpc.WithBlock(),
	)
	if err != nil {
		return nil, fmt.Errorf("连接 CloudDrive2 失败: %w", err)
	}

	return &cd2Client{
		config:    config,
		serverURL: serverURL,
		conn:      conn,
		client:    cd2pb.NewCloudDriveFileSrvClient(conn),
	}, nil
}

func (c *cd2Client) Close() error {
	if c.conn == nil {
		return nil
	}
	return c.conn.Close()
}

func (c *cd2Client) authContext(ctx context.Context) (context.Context, error) {
	if strings.TrimSpace(c.token) == "" || time.Now().Add(30*time.Second).After(c.tokenExpiry) {
		if err := c.refreshToken(ctx); err != nil {
			return nil, err
		}
	}
	return metadata.AppendToOutgoingContext(ctx, "authorization", "Bearer "+c.token), nil
}

func (c *cd2Client) refreshToken(ctx context.Context) error {
	authCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	token, err := c.client.GetToken(authCtx, &cd2pb.GetTokenRequest{
		UserName: c.config.Username,
		Password: c.config.Password,
	})
	if err != nil {
		return fmt.Errorf("获取 CloudDrive2 令牌失败: %w", err)
	}
	if !token.GetSuccess() || strings.TrimSpace(token.GetToken()) == "" {
		if msg := strings.TrimSpace(token.GetErrorMessage()); msg != "" {
			return fmt.Errorf("CloudDrive2 登录失败: %s", msg)
		}
		return fmt.Errorf("CloudDrive2 登录失败")
	}

	c.token = token.GetToken()
	if token.GetExpiration() != nil {
		c.tokenExpiry = token.GetExpiration().AsTime()
	} else {
		c.tokenExpiry = time.Now().Add(30 * time.Minute)
	}
	return nil
}

func (c *cd2Client) getAllCloudAPIs(ctx context.Context) ([]*cd2pb.CloudAPI, error) {
	authCtx, err := c.authContext(ctx)
	if err != nil {
		return nil, err
	}
	reply, err := c.client.GetAllCloudApis(authCtx, &emptypb.Empty{})
	if err != nil {
		return nil, fmt.Errorf("读取 CloudDrive2 云盘列表失败: %w", err)
	}
	return reply.GetApis(), nil
}

func (c *cd2Client) getRuntimeInfo(ctx context.Context) (*cd2pb.RuntimeInfo, error) {
	authCtx, err := c.authContext(ctx)
	if err != nil {
		return nil, err
	}
	return c.client.GetRuntimeInfo(authCtx, &emptypb.Empty{})
}

func (c *cd2Client) getSystemSettings(ctx context.Context) (*cd2pb.SystemSettings, error) {
	authCtx, err := c.authContext(ctx)
	if err != nil {
		return nil, err
	}
	reply, err := c.client.GetSystemSettings(authCtx, &emptypb.Empty{})
	if err != nil {
		return nil, fmt.Errorf("读取 CloudDrive2 系统设置失败: %w", err)
	}
	return reply, nil
}

func (c *cd2Client) setSystemSettings(ctx context.Context, settings *cd2pb.SystemSettings) error {
	authCtx, err := c.authContext(ctx)
	if err != nil {
		return err
	}
	if _, err := c.client.SetSystemSettings(authCtx, settings); err != nil {
		return fmt.Errorf("更新 CloudDrive2 系统设置失败: %w", err)
	}
	return nil
}

func parseGatewayURL(raw string) (*url.URL, error) {
	value := strings.TrimSpace(raw)
	if value == "" {
		return nil, fmt.Errorf("CloudDrive2 地址不能为空")
	}
	if !strings.Contains(value, "://") {
		value = "http://" + value
	}
	parsed, err := url.Parse(value)
	if err != nil {
		return nil, fmt.Errorf("CloudDrive2 地址格式无效: %w", err)
	}
	if strings.TrimSpace(parsed.Host) == "" {
		return nil, fmt.Errorf("CloudDrive2 地址格式无效")
	}
	return parsed, nil
}

func resolveTransportCredentials(serverURL *url.URL) credentials.TransportCredentials {
	if strings.EqualFold(serverURL.Scheme, "https") {
		return credentials.NewTLS(nil)
	}
	return insecure.NewCredentials()
}
