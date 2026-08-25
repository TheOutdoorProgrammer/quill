package appleprofiles

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestProfileNameIsStableForDeviceOrder(t *testing.T) {
	first := profileName("com.example.app", "certificate", []string{"device-a", "device-b"}, nil)
	second := profileName("com.example.app", "certificate", []string{"device-b", "device-a"}, nil)
	if first != second {
		t.Fatalf("profile name changed: %q != %q", first, second)
	}
	if first == profileName("com.example.app", "certificate", []string{"device-a"}, nil) {
		t.Fatal("profile name did not change with device membership")
	}
	if first == profileName("com.example.app", "replacement", []string{"device-a", "device-b"}, nil) {
		t.Fatal("profile name did not change with certificate")
	}
	if first == profileName("com.example.app", "certificate", []string{"device-a", "device-b"}, []string{"push"}) {
		t.Fatal("profile name did not change with capabilities")
	}
}

func TestEnsureCreatesAndInstallsProfile(t *testing.T) {
	now := time.Date(2026, 8, 25, 12, 0, 0, 0, time.UTC)
	certificateDER := []byte("distribution-certificate")
	var created map[string]any

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/v1/certificates":
			writeJSON(t, w, map[string]any{"data": []any{resource("certificate", map[string]any{
				"certificateContent": base64.StdEncoding.EncodeToString(certificateDER),
			})}})
		case r.Method == http.MethodGet && r.URL.Path == "/v1/devices":
			writeJSON(t, w, map[string]any{"data": []any{
				resource("device-b", map[string]any{"platform": "IOS", "status": "ENABLED"}),
				resource("ignored", map[string]any{"platform": "MAC_OS", "status": "ENABLED"}),
				resource("device-a", map[string]any{"platform": "IOS", "status": "ENABLED"}),
			}})
		case r.Method == http.MethodGet && r.URL.Path == "/v1/profiles":
			writeJSON(t, w, map[string]any{"data": []any{}})
		case r.Method == http.MethodGet && r.URL.Path == "/v1/bundleIds":
			identifier := r.URL.Query().Get("filter[identifier]")
			writeJSON(t, w, map[string]any{"data": []any{resource("bundle", map[string]any{"identifier": identifier})}})
		case r.Method == http.MethodGet && r.URL.Path == "/v1/bundleIds/bundle/bundleIdCapabilities":
			writeJSON(t, w, map[string]any{"data": []any{}})
		case r.Method == http.MethodPost && r.URL.Path == "/v1/profiles":
			if err := json.NewDecoder(r.Body).Decode(&created); err != nil {
				t.Fatalf("decode create request: %v", err)
			}
			writeJSON(t, w, map[string]any{"data": profile("profile", "created", "uuid-created", now.AddDate(1, 0, 0), "signed-profile")})
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	client := &Client{baseURL: server.URL, httpClient: server.Client(), now: func() time.Time { return now }, token: "token"}
	outputDir := t.TempDir()
	results, err := client.Ensure(context.Background(), []string{"com.example.app"}, &x509.Certificate{Raw: certificateDER}, outputDir)
	if err != nil {
		t.Fatal(err)
	}
	result := results["com.example.app"]
	if result.Reused {
		t.Fatal("new profile reported as reused")
	}
	contents, err := os.ReadFile(filepath.Join(outputDir, "uuid-created.mobileprovision"))
	if err != nil {
		t.Fatal(err)
	}
	if string(contents) != "signed-profile" {
		t.Fatalf("installed %q", contents)
	}

	data := created["data"].(map[string]any)
	relationships := data["relationships"].(map[string]any)
	devices := relationships["devices"].(map[string]any)["data"].([]any)
	if devices[0].(map[string]any)["id"] != "device-a" || devices[1].(map[string]any)["id"] != "device-b" {
		t.Fatalf("devices were not sorted: %#v", devices)
	}
}

func TestEnsureReusesActiveProfileAndFetchesItsContent(t *testing.T) {
	now := time.Date(2026, 8, 25, 12, 0, 0, 0, time.UTC)
	certificateDER := []byte("distribution-certificate")
	name := profileName("com.example.app", "certificate", []string{"device"}, nil)
	posts := 0

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/v1/certificates":
			writeJSON(t, w, map[string]any{"data": []any{resource("certificate", map[string]any{
				"certificateContent": base64.StdEncoding.EncodeToString(certificateDER),
			})}})
		case r.Method == http.MethodGet && r.URL.Path == "/v1/devices":
			writeJSON(t, w, map[string]any{"data": []any{resource("device", map[string]any{"platform": "IOS", "status": "ENABLED"})}})
		case r.Method == http.MethodGet && r.URL.Path == "/v1/profiles":
			writeJSON(t, w, map[string]any{"data": []any{profile("profile", name, "uuid-reused", now.AddDate(1, 0, 0), "")}})
		case r.Method == http.MethodGet && r.URL.Path == "/v1/profiles/profile":
			writeJSON(t, w, map[string]any{"data": profile("profile", name, "uuid-reused", now.AddDate(1, 0, 0), "reused-profile")})
		case r.Method == http.MethodGet && r.URL.Path == "/v1/bundleIds":
			writeJSON(t, w, map[string]any{"data": []any{resource("bundle", map[string]any{"identifier": "com.example.app"})}})
		case r.Method == http.MethodGet && r.URL.Path == "/v1/bundleIds/bundle/bundleIdCapabilities":
			writeJSON(t, w, map[string]any{"data": []any{}})
		case r.Method == http.MethodPost:
			posts++
			http.Error(w, "unexpected create", http.StatusInternalServerError)
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	client := &Client{baseURL: server.URL, httpClient: server.Client(), now: func() time.Time { return now }, token: "token"}
	results, err := client.Ensure(context.Background(), []string{"com.example.app"}, &x509.Certificate{Raw: certificateDER}, t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if !results["com.example.app"].Reused {
		t.Fatal("active profile was not reused")
	}
	if posts != 0 {
		t.Fatalf("created %d profiles", posts)
	}
}

func TestEnsureDeletesStaleExactNameBeforeCreatingReplacement(t *testing.T) {
	now := time.Date(2026, 8, 25, 12, 0, 0, 0, time.UTC)
	certificateDER := []byte("distribution-certificate")
	name := profileName("com.example.app", "certificate", []string{"device"}, nil)
	var deleted []string

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/v1/certificates":
			writeJSON(t, w, map[string]any{"data": []any{resource("certificate", map[string]any{
				"certificateContent": base64.StdEncoding.EncodeToString(certificateDER),
			})}})
		case r.Method == http.MethodGet && r.URL.Path == "/v1/devices":
			writeJSON(t, w, map[string]any{"data": []any{resource("device", map[string]any{"platform": "IOS", "status": "ENABLED"})}})
		case r.Method == http.MethodGet && r.URL.Path == "/v1/profiles":
			writeJSON(t, w, map[string]any{"data": []any{
				profile("expired", name, "uuid-expired", now.Add(-time.Hour), "expired"),
				profile("unrelated", "Someone else's profile", "uuid-unrelated", now.AddDate(1, 0, 0), "unrelated"),
			}})
		case r.Method == http.MethodGet && r.URL.Path == "/v1/bundleIds":
			writeJSON(t, w, map[string]any{"data": []any{resource("bundle", map[string]any{"identifier": "com.example.app"})}})
		case r.Method == http.MethodGet && r.URL.Path == "/v1/bundleIds/bundle/bundleIdCapabilities":
			writeJSON(t, w, map[string]any{"data": []any{}})
		case r.Method == http.MethodDelete && r.URL.Path == "/v1/profiles/expired":
			deleted = append(deleted, r.URL.Path)
			w.WriteHeader(http.StatusNoContent)
		case r.Method == http.MethodDelete:
			t.Fatalf("deleted unrelated profile %s", r.URL.Path)
		case r.Method == http.MethodPost && r.URL.Path == "/v1/profiles":
			writeJSON(t, w, map[string]any{"data": profile("replacement", name, "uuid-replacement", now.AddDate(1, 0, 0), "replacement")})
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	client := &Client{baseURL: server.URL, httpClient: server.Client(), now: func() time.Time { return now }, token: "token"}
	results, err := client.Ensure(context.Background(), []string{"com.example.app"}, &x509.Certificate{Raw: certificateDER}, t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if len(deleted) != 1 || deleted[0] != "/v1/profiles/expired" {
		t.Fatalf("deleted profiles: %v", deleted)
	}
	if results["com.example.app"].UUID != "uuid-replacement" {
		t.Fatalf("unexpected replacement: %#v", results["com.example.app"])
	}
}

func TestEnsureReusesConcurrentCreatorAfterCreateConflict(t *testing.T) {
	now := time.Date(2026, 8, 25, 12, 0, 0, 0, time.UTC)
	certificateDER := []byte("distribution-certificate")
	name := profileName("com.example.app", "certificate", []string{"device"}, nil)
	profileLists := 0

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/v1/certificates":
			writeJSON(t, w, map[string]any{"data": []any{resource("certificate", map[string]any{
				"certificateContent": base64.StdEncoding.EncodeToString(certificateDER),
			})}})
		case r.Method == http.MethodGet && r.URL.Path == "/v1/devices":
			writeJSON(t, w, map[string]any{"data": []any{resource("device", map[string]any{"platform": "IOS", "status": "ENABLED"})}})
		case r.Method == http.MethodGet && r.URL.Path == "/v1/profiles":
			profileLists++
			if profileLists == 1 {
				writeJSON(t, w, map[string]any{"data": []any{}})
				return
			}
			writeJSON(t, w, map[string]any{"data": []any{profile("winner", name, "uuid-winner", now.AddDate(1, 0, 0), "winner")}})
		case r.Method == http.MethodGet && r.URL.Path == "/v1/bundleIds":
			writeJSON(t, w, map[string]any{"data": []any{resource("bundle", map[string]any{"identifier": "com.example.app"})}})
		case r.Method == http.MethodGet && r.URL.Path == "/v1/bundleIds/bundle/bundleIdCapabilities":
			writeJSON(t, w, map[string]any{"data": []any{}})
		case r.Method == http.MethodPost && r.URL.Path == "/v1/profiles":
			http.Error(w, "profile name already exists", http.StatusConflict)
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	client := &Client{baseURL: server.URL, httpClient: server.Client(), now: func() time.Time { return now }, token: "token"}
	results, err := client.Ensure(context.Background(), []string{"com.example.app"}, &x509.Certificate{Raw: certificateDER}, t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if !results["com.example.app"].Reused || results["com.example.app"].UUID != "uuid-winner" {
		t.Fatalf("did not reuse concurrent profile: %#v", results["com.example.app"])
	}
}

func TestSignedTokenUsesRawES256Signature(t *testing.T) {
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	encoded, err := x509.MarshalPKCS8PrivateKey(key)
	if err != nil {
		t.Fatal(err)
	}
	privateKey := pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: encoded})
	token, err := signedToken("issuer", "key", privateKey, time.Unix(1000, 0))
	if err != nil {
		t.Fatal(err)
	}
	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		t.Fatalf("token has %d parts", len(parts))
	}
	signature, err := base64.RawURLEncoding.DecodeString(parts[2])
	if err != nil {
		t.Fatal(err)
	}
	if len(signature) != 64 {
		t.Fatalf("signature is %d bytes, want 64", len(signature))
	}
}

func resource(id string, attributes map[string]any) map[string]any {
	return map[string]any{"attributes": attributes, "id": id, "type": "test"}
}

func profile(id, name, uuid string, expires time.Time, content string) map[string]any {
	attributes := map[string]any{
		"expirationDate": expires.Format(time.RFC3339),
		"name":           name,
		"profileState":   "ACTIVE",
		"uuid":           uuid,
	}
	if content != "" {
		attributes["profileContent"] = base64.StdEncoding.EncodeToString([]byte(content))
	}
	return resource(id, attributes)
}

func writeJSON(t *testing.T, w http.ResponseWriter, value any) {
	t.Helper()
	if err := json.NewEncoder(w).Encode(value); err != nil {
		t.Fatalf("encode response: %v", err)
	}
}
