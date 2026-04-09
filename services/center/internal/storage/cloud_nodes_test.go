package storage

import (
	"context"
	"io"
	"net/http"
	"net/url"
	"strings"
	"testing"
	"time"

	"mare/services/center/internal/db"
	storagedto "mare/shared/contracts/dto/storage"
)

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(request *http.Request) (*http.Response, error) {
	return f(request)
}

func jsonHTTPResponse(body string) *http.Response {
	return &http.Response{
		StatusCode: http.StatusOK,
		Header: http.Header{
			"Content-Type": []string{"application/json"},
		},
		Body: io.NopCloser(strings.NewReader(body)),
	}
}

func TestCloudNodeServiceSaveQRCodeNodePersistsValidatedCookie(t *testing.T) {
	if testing.Short() {
		t.Skip("skip integration test in short mode")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	pool := openStorageTestPool(t, ctx)
	defer pool.Close()
	resetStorageSchema(t, ctx, pool)

	migrator := db.NewMigrator()
	if _, err := migrator.Apply(ctx, pool); err != nil {
		t.Fatalf("apply migrations: %v", err)
	}

	service := NewCloudNodeService(pool)
	service.cipher = fakeCredentialCipher{}
	service.client = &http.Client{
		Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
			switch {
			case request.Method == http.MethodPost && strings.Contains(request.URL.Host, "passportapi.115.com") && strings.Contains(request.URL.Path, "/login/qrcode/"):
				body, _ := io.ReadAll(request.Body)
				form := string(body)
				if !strings.Contains(form, "account=uid-1") || !strings.Contains(form, "app=wechatmini") {
					t.Fatalf("unexpected login exchange request body: %s", form)
				}
				return jsonHTTPResponse(`{"state":1,"data":{"cookie":{"UID":"uid-1","CID":"cid-1"}}}`), nil
			case request.Method == http.MethodGet && strings.Contains(request.URL.Host, "qrcodeapi.115.com") && strings.Contains(request.URL.Path, "/login_devices"):
				if request.Header.Get("Cookie") != "CID=cid-1; UID=uid-1" {
					t.Fatalf("unexpected cookie header: %s", request.Header.Get("Cookie"))
				}
				return jsonHTTPResponse(`{"state":true,"code":0,"data":{"devices":[]}}`), nil
			case request.Method == http.MethodGet && strings.Contains(request.URL.Host, "webapi.115.com") && request.URL.Path == "/files":
				return jsonHTTPResponse(`{"state":true,"data":[]}`), nil
			case request.Method == http.MethodPost && strings.Contains(request.URL.Host, "webapi.115.com") && request.URL.Path == "/files/add":
				return jsonHTTPResponse(`{"state":true,"cid":"100"}`), nil
			default:
				t.Fatalf("unexpected request: %s %s", request.Method, request.URL.String())
				return nil, nil
			}
		}),
	}

	result, err := service.SaveCloudNode(ctx, storagedto.SaveCloudNodeRequest{
		Name:         "115 云归档",
		Vendor:       "115",
		AccessMethod: "鎵爜鐧诲綍鑾峰彇 Token",
		QRChannel:    "微信小程序",
		MountPath:    "/MareArchive",
		QRSession: &storagedto.CloudQRCodeSession{
			UID:     "uid-1",
			Time:    123,
			Sign:    "sign-1",
			QRCode:  "https://115.com/scan/mock",
			Channel: "微信小程序",
		},
	})
	if err != nil {
		t.Fatalf("save cloud node: %v", err)
	}

	var ciphertext string
	var authStatus string
	var healthStatus string
	if err := pool.QueryRow(ctx, `
		SELECT snc.secret_ciphertext, snr.auth_status, snr.health_status
		FROM storage_node_credentials snc
		JOIN storage_node_runtime snr ON snr.storage_node_id = snc.storage_node_id
		WHERE snc.storage_node_id = $1
	`, result.Record.ID).Scan(&ciphertext, &authStatus, &healthStatus); err != nil {
		t.Fatalf("load saved cloud node: %v", err)
	}

	if ciphertext != "cipher::CID=cid-1; UID=uid-1" {
		t.Fatalf("unexpected ciphertext: %s", ciphertext)
	}
	if authStatus != "AUTHORIZED" || healthStatus != "ONLINE" {
		t.Fatalf("unexpected runtime status: auth=%s health=%s", authStatus, healthStatus)
	}
}

func TestCloudNodeServiceConnectionTestUsesSavedCookie(t *testing.T) {
	if testing.Short() {
		t.Skip("skip integration test in short mode")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	pool := openStorageTestPool(t, ctx)
	defer pool.Close()
	resetStorageSchema(t, ctx, pool)

	migrator := db.NewMigrator()
	if _, err := migrator.Apply(ctx, pool); err != nil {
		t.Fatalf("apply migrations: %v", err)
	}

	requestCount := 0
	service := NewCloudNodeService(pool)
	service.cipher = fakeCredentialCipher{}
	service.client = &http.Client{
		Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
			if request.Method == http.MethodGet && strings.Contains(request.URL.Path, "/login_devices") {
				requestCount++
				if request.Header.Get("Cookie") != "UID=uid-1; CID=cid-1" {
					t.Fatalf("unexpected cookie header: %s", request.Header.Get("Cookie"))
				}
				return jsonHTTPResponse(`{"state":true,"code":0,"data":{"devices":[{"icon":"desktop"}]}}`), nil
			}
			if request.Method == http.MethodGet && strings.Contains(request.URL.Host, "webapi.115.com") && request.URL.Path == "/files" {
				return jsonHTTPResponse(`{"state":true,"data":[]}`), nil
			}
			if request.Method == http.MethodPost && strings.Contains(request.URL.Host, "webapi.115.com") && request.URL.Path == "/files/add" {
				return jsonHTTPResponse(`{"state":true,"cid":"100"}`), nil
			}
			t.Fatalf("unexpected request: %s %s", request.Method, request.URL.String())
			return nil, nil
		}),
	}

	created, err := service.SaveCloudNode(ctx, storagedto.SaveCloudNodeRequest{
		Name:         "115 云归档",
		Vendor:       "115",
		AccessMethod: "濉叆 Token",
		MountPath:    "/MareArchive",
		Token:        "UID=uid-1; CID=cid-1",
	})
	if err != nil {
		t.Fatalf("save token cloud node: %v", err)
	}

	response, err := service.RunCloudNodeConnectionTest(ctx, []string{created.Record.ID})
	if err != nil {
		t.Fatalf("run cloud connection test: %v", err)
	}

	if requestCount < 2 {
		t.Fatalf("expected cookie validation to call 115 endpoint during save and test, got %d", requestCount)
	}
	if len(response.Results) != 1 || response.Results[0].OverallTone != "success" {
		t.Fatalf("unexpected connection test response: %+v", response.Results)
	}
}

func TestCloudNodeServiceConnectionTestFailsWhenCookieRejected(t *testing.T) {
	if testing.Short() {
		t.Skip("skip integration test in short mode")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	pool := openStorageTestPool(t, ctx)
	defer pool.Close()
	resetStorageSchema(t, ctx, pool)

	migrator := db.NewMigrator()
	if _, err := migrator.Apply(ctx, pool); err != nil {
		t.Fatalf("apply migrations: %v", err)
	}

	service := NewCloudNodeService(pool)
	service.cipher = fakeCredentialCipher{}
	service.client = &http.Client{
		Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
			if request.Method == http.MethodGet && strings.Contains(request.URL.Path, "/login_devices") {
				return jsonHTTPResponse(`{"state":0,"code":40101032,"errno":40101032,"message":"请重新登录","error":"请重新登录","data":{}}`), nil
			}
			t.Fatalf("unexpected request: %s %s", request.Method, request.URL.String())
			return nil, nil
		}),
	}

	now := time.Now().UTC()
	if _, err := pool.Exec(ctx, `
		INSERT INTO storage_nodes (
			id, code, name, node_type, vendor, address, access_mode, account_alias, enabled, created_at, updated_at
		) VALUES (
			'cloud-1', 'cloud-1', '115 云归档', 'CLOUD', '115', '/MareArchive', 'QR', '115 云归档', true, $1, $1
		)
	`, now); err != nil {
		t.Fatalf("insert storage node: %v", err)
	}
	if _, err := pool.Exec(ctx, `
		INSERT INTO storage_node_credentials (
			id, storage_node_id, credential_kind, secret_ciphertext, secret_ref, token_status, updated_at, created_at
		) VALUES (
			'cred-1', 'cloud-1', 'TOKEN', $1, '微信小程序', 'CONFIGURED', $2, $2
		)
	`, "cipher::UID=uid-1; CID=cid-1", now); err != nil {
		t.Fatalf("insert storage credential: %v", err)
	}
	if _, err := pool.Exec(ctx, `
		INSERT INTO storage_node_runtime (
			id, storage_node_id, health_status, auth_status, created_at, updated_at
		) VALUES (
			'runtime-1', 'cloud-1', 'ONLINE', 'AUTHORIZED', $1, $1
		)
	`, now); err != nil {
		t.Fatalf("insert storage runtime: %v", err)
	}

	response, err := service.RunCloudNodeConnectionTest(ctx, []string{"cloud-1"})
	if err != nil {
		t.Fatalf("run cloud connection test: %v", err)
	}
	if len(response.Results) != 1 || response.Results[0].OverallTone != "critical" {
		t.Fatalf("unexpected response: %+v", response.Results)
	}

	var authStatus string
	if err := pool.QueryRow(ctx, `
		SELECT auth_status
		FROM storage_node_runtime
		WHERE storage_node_id = 'cloud-1'
	`).Scan(&authStatus); err != nil {
		t.Fatalf("load runtime: %v", err)
	}
	if authStatus != "FAILED" {
		t.Fatalf("expected auth status FAILED, got %s", authStatus)
	}
}

func TestCloudNodeServiceSaveCloudNodeCreatesMountPathRecursively(t *testing.T) {
	if testing.Short() {
		t.Skip("skip integration test in short mode")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	pool := openStorageTestPool(t, ctx)
	defer pool.Close()
	resetStorageSchema(t, ctx, pool)

	migrator := db.NewMigrator()
	if _, err := migrator.Apply(ctx, pool); err != nil {
		t.Fatalf("apply migrations: %v", err)
	}

	var created []string
	service := NewCloudNodeService(pool)
	service.cipher = fakeCredentialCipher{}
	service.client = &http.Client{
		Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
			switch {
			case request.Method == http.MethodGet && strings.Contains(request.URL.Path, "/login_devices"):
				if request.Header.Get("Cookie") != "UID=uid-1; CID=cid-1" {
					t.Fatalf("unexpected cookie header: %s", request.Header.Get("Cookie"))
				}
				return jsonHTTPResponse(`{"state":true,"code":0,"data":{"devices":[]}}`), nil
			case request.Method == http.MethodGet && strings.Contains(request.URL.Host, "webapi.115.com") && request.URL.Path == "/files":
				cid := request.URL.Query().Get("cid")
				switch cid {
				case "0":
					return jsonHTTPResponse(`{"state":true,"data":[]}`), nil
				case "100":
					return jsonHTTPResponse(`{"state":true,"data":[]}`), nil
				case "101":
					return jsonHTTPResponse(`{"state":true,"data":[]}`), nil
				default:
					t.Fatalf("unexpected list cid: %s", cid)
					return nil, nil
				}
			case request.Method == http.MethodPost && strings.Contains(request.URL.Host, "webapi.115.com") && request.URL.Path == "/files/add":
				body, _ := io.ReadAll(request.Body)
				values, err := url.ParseQuery(string(body))
				if err != nil {
					t.Fatalf("parse body: %v", err)
				}
				created = append(created, values.Get("pid")+":"+values.Get("cname"))
				switch values.Get("cname") {
				case "Mare":
					return jsonHTTPResponse(`{"state":true,"cid":"100"}`), nil
				case "Archive":
					return jsonHTTPResponse(`{"state":true,"cid":"101"}`), nil
				case "2026":
					return jsonHTTPResponse(`{"state":true,"cid":"102"}`), nil
				default:
					t.Fatalf("unexpected created folder: %s", values.Get("cname"))
					return nil, nil
				}
			default:
				t.Fatalf("unexpected request: %s %s", request.Method, request.URL.String())
				return nil, nil
			}
		}),
	}

	_, err := service.SaveCloudNode(ctx, storagedto.SaveCloudNodeRequest{
		Name:         "115 云归档",
		Vendor:       "115",
		AccessMethod: "TOKEN",
		MountPath:    "/Mare/Archive/2026",
		Token:        "UID=uid-1; CID=cid-1",
	})
	if err != nil {
		t.Fatalf("save token cloud node: %v", err)
	}

	expected := []string{"0:Mare", "100:Archive", "101:2026"}
	if strings.Join(created, ",") != strings.Join(expected, ",") {
		t.Fatalf("unexpected created folders: %+v", created)
	}
}
