package integration

import (
	"context"
	"fmt"
	"net/url"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"mare/services/center/internal/db"
	integrationdto "mare/shared/contracts/dto/integration"
)

type fakeIntegrationCipher struct{}

func (fakeIntegrationCipher) Encrypt(plaintext string) (string, error) {
	return "cipher::" + plaintext, nil
}

func (fakeIntegrationCipher) Decrypt(ciphertext string) (string, error) {
	return ciphertext[len("cipher::"):], nil
}

type fakeGatewayDriver struct {
	testErr error
}

func (f fakeGatewayDriver) Vendor() string { return "115" }

func (f fakeGatewayDriver) AuthenticateToken(context.Context, string) (ProviderAuthResult, error) {
	return ProviderAuthResult{}, nil
}

func (f fakeGatewayDriver) CreateQRCodeSession(context.Context, string) (QRCodeSession, error) {
	return QRCodeSession{}, nil
}

func (f fakeGatewayDriver) GetQRCodeSession(context.Context, string) (QRCodeSession, error) {
	return QRCodeSession{}, nil
}

func (f fakeGatewayDriver) ConsumeQRCodeSession(context.Context, string) (ProviderAuthResult, error) {
	return ProviderAuthResult{}, nil
}

func (f fakeGatewayDriver) EnsureRemoteRoot(context.Context, CloudProviderPayload, string) error {
	return nil
}

func (f fakeGatewayDriver) StartUpload(context.Context, CloudProviderPayload, string, string, UploadSource) (string, string, error) {
	return "", "", nil
}

func (f fakeGatewayDriver) AttachUpload(context.Context, string, string, UploadSource) error {
	return nil
}

func (f fakeGatewayDriver) WaitUpload(context.Context, string, string, func(TransferProgress)) error {
	return nil
}

func (f fakeGatewayDriver) ResetUploadSession(context.Context) error {
	return nil
}

func (f fakeGatewayDriver) PauseUpload(context.Context, string) error {
	return nil
}

func (f fakeGatewayDriver) ResumeUpload(context.Context, string) error {
	return nil
}

func (f fakeGatewayDriver) CancelUpload(context.Context, string) error {
	return nil
}

func (f fakeGatewayDriver) ResolveDownloadSource(context.Context, CloudProviderPayload, string, string) (DownloadSource, error) {
	return DownloadSource{}, nil
}

func (f fakeGatewayDriver) DeleteFile(context.Context, CloudProviderPayload, string, string) error {
	return nil
}

func (f fakeGatewayDriver) TestGateway(context.Context, CD2GatewayConfig) error {
	return f.testErr
}

func TestServiceTestCD2GatewayPersistsRuntimeStatus(t *testing.T) {
	if testing.Short() {
		t.Skip("skip integration test in short mode")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	pool := openIntegrationTestPool(t, ctx)
	defer pool.Close()
	resetIntegrationSchema(t, ctx, pool)

	migrator := db.NewMigrator()
	if _, err := migrator.Apply(ctx, pool); err != nil {
		t.Fatalf("apply migrations: %v", err)
	}

	service := NewService(pool)
	service.cipher = fakeIntegrationCipher{}
	service.now = func() time.Time {
		return time.Date(2026, 4, 12, 10, 27, 37, 0, time.UTC)
	}
	service.RegisterProvider(fakeGatewayDriver{})

	_, err := service.SaveCD2Gateway(ctx, integrationdto.SaveCD2GatewayRequest{
		BaseURL:  "http://localhost:29798",
		Username: "mare",
		Password: "secret",
		Enabled:  true,
	})
	if err != nil {
		t.Fatalf("save gateway: %v", err)
	}

	result, err := service.TestCD2Gateway(ctx, integrationdto.TestCD2GatewayRequest{})
	if err != nil {
		t.Fatalf("test gateway: %v", err)
	}
	if result.Record.RuntimeStatus != "ONLINE" {
		t.Fatalf("expected runtime status ONLINE, got %s", result.Record.RuntimeStatus)
	}

	var (
		runtimeStatus string
		lastTestAt    *time.Time
	)
	if err := pool.QueryRow(ctx, `
		SELECT runtime_status, last_test_at
		FROM integration_gateways
		WHERE gateway_type = 'CD2'
	`).Scan(&runtimeStatus, &lastTestAt); err != nil {
		t.Fatalf("load gateway runtime: %v", err)
	}

	if runtimeStatus != "ONLINE" {
		t.Fatalf("expected persisted runtime status ONLINE, got %s", runtimeStatus)
	}
	if lastTestAt == nil {
		t.Fatal("expected last_test_at to be persisted")
	}
}

func openIntegrationTestPool(t *testing.T, ctx context.Context) *pgxpool.Pool {
	t.Helper()

	databaseURL := isolatedIntegrationSchemaDatabaseURL(t, ctx)
	pool, err := db.Open(ctx, databaseURL)
	if err != nil {
		t.Fatalf("open database: %v", err)
	}
	return pool
}

func isolatedIntegrationSchemaDatabaseURL(t *testing.T, ctx context.Context) string {
	t.Helper()

	baseURL := "postgres://mare:mare@localhost:5432/mare_dev?sslmode=disable"
	schema := fmt.Sprintf("test_integration_%d", time.Now().UnixNano())

	adminPool, err := pgxpool.New(ctx, baseURL)
	if err != nil {
		t.Fatalf("open admin pool: %v", err)
	}
	t.Cleanup(adminPool.Close)

	if _, err := adminPool.Exec(ctx, `CREATE SCHEMA IF NOT EXISTS `+pgx.Identifier{schema}.Sanitize()); err != nil {
		t.Fatalf("create schema: %v", err)
	}

	t.Cleanup(func() {
		cleanupCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_, _ = adminPool.Exec(cleanupCtx, `DROP SCHEMA IF EXISTS `+pgx.Identifier{schema}.Sanitize()+` CASCADE`)
	})

	parsed, err := url.Parse(baseURL)
	if err != nil {
		t.Fatalf("parse database url: %v", err)
	}
	query := parsed.Query()
	query.Set("search_path", schema)
	parsed.RawQuery = query.Encode()
	return parsed.String()
}

func resetIntegrationSchema(t *testing.T, ctx context.Context, pool *pgxpool.Pool) {
	t.Helper()

	if _, err := pool.Exec(ctx, `
		DROP TABLE IF EXISTS integration_gateway_credentials;
		DROP TABLE IF EXISTS integration_gateways;
		DROP TABLE IF EXISTS schema_migrations;
	`); err != nil {
		t.Fatalf("reset schema: %v", err)
	}
}
