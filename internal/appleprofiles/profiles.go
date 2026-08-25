package appleprofiles

import (
	"bytes"
	"context"
	"crypto/ecdsa"
	"crypto/rand"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

const appStoreConnectURL = "https://api.appstoreconnect.apple.com"

type Client struct {
	baseURL    string
	httpClient *http.Client
	now        func() time.Time
	token      string
}

type Result struct {
	Name   string `json:"name"`
	Path   string `json:"path"`
	Reused bool   `json:"reused"`
	UUID   string `json:"uuid"`
}

type bundleIDResource struct {
	ID         string `json:"id"`
	Attributes struct {
		Identifier string `json:"identifier"`
	} `json:"attributes"`
}

type certificateResource struct {
	ID         string `json:"id"`
	Attributes struct {
		CertificateContent string `json:"certificateContent"`
	} `json:"attributes"`
}

type capabilityResource struct {
	ID         string `json:"id"`
	Attributes struct {
		CapabilityType string `json:"capabilityType"`
		Settings       []struct {
			Key     string `json:"key"`
			Options []struct {
				Enabled bool   `json:"enabled"`
				Key     string `json:"key"`
			} `json:"options"`
		} `json:"settings"`
	} `json:"attributes"`
}

type deviceResource struct {
	ID         string `json:"id"`
	Attributes struct {
		Platform string `json:"platform"`
		Status   string `json:"status"`
	} `json:"attributes"`
}

type profileResource struct {
	ID         string `json:"id"`
	Attributes struct {
		ExpirationDate time.Time `json:"expirationDate"`
		Name           string    `json:"name"`
		ProfileContent string    `json:"profileContent"`
		ProfileState   string    `json:"profileState"`
		UUID           string    `json:"uuid"`
	} `json:"attributes"`
}

type listResponse[T any] struct {
	Data  []T `json:"data"`
	Links struct {
		Next string `json:"next"`
	} `json:"links"`
}

type resourceResponse[T any] struct {
	Data T `json:"data"`
}

func NewClient(issuerID, keyID string, privateKey []byte) (*Client, error) {
	now := time.Now
	token, err := signedToken(issuerID, keyID, privateKey, now())
	if err != nil {
		return nil, err
	}
	return &Client{
		baseURL:    appStoreConnectURL,
		httpClient: http.DefaultClient,
		now:        now,
		token:      token,
	}, nil
}

func (c *Client) Ensure(ctx context.Context, bundleIdentifiers []string, certificate *x509.Certificate, outputDir string) (map[string]Result, error) {
	certificates, err := listResources[certificateResource](ctx, c, "/v1/certificates?limit=200")
	if err != nil {
		return nil, err
	}
	matchingCertificate, err := findCertificate(certificates, certificate.Raw)
	if err != nil {
		return nil, err
	}

	devices, err := listResources[deviceResource](ctx, c, "/v1/devices?limit=200")
	if err != nil {
		return nil, err
	}
	deviceIDs := enabledIOSDeviceIDs(devices)
	if len(deviceIDs) == 0 {
		return nil, fmt.Errorf("at least one enabled iOS device is required for an Ad Hoc profile")
	}

	profiles, err := listResources[profileResource](ctx, c, "/v1/profiles?filter%5BprofileType%5D=IOS_APP_ADHOC&limit=200")
	if err != nil {
		return nil, err
	}
	if err := os.MkdirAll(outputDir, 0o700); err != nil {
		return nil, fmt.Errorf("creating provisioning profile directory: %w", err)
	}

	results := make(map[string]Result, len(bundleIdentifiers))
	for _, identifier := range bundleIdentifiers {
		bundle, err := c.bundleID(ctx, identifier)
		if err != nil {
			return nil, err
		}

		capabilities, err := listResources[capabilityResource](ctx, c, "/v1/bundleIds/"+url.PathEscape(bundle.ID)+"/bundleIdCapabilities")
		if err != nil {
			return nil, fmt.Errorf("reading capabilities for %s: %w", identifier, err)
		}
		name := profileName(identifier, matchingCertificate.ID, deviceIDs, capabilityMembership(capabilities))
		profile, reused := reusableProfile(profiles, name, c.now())
		if !reused {
			if err := c.deleteStaleProfiles(ctx, profiles, name); err != nil {
				return nil, fmt.Errorf("deleting stale profiles for %s: %w", identifier, err)
			}
			profile, err = c.createProfile(ctx, name, bundle.ID, matchingCertificate.ID, deviceIDs)
			if err != nil {
				profile, reused = c.reuseAfterCreateConflict(ctx, name)
				if !reused {
					return nil, fmt.Errorf("creating profile for %s: %w", identifier, err)
				}
			}
		}
		if profile.Attributes.ProfileContent == "" {
			profile, err = c.profile(ctx, profile.ID)
			if err != nil {
				return nil, fmt.Errorf("reading profile for %s: %w", identifier, err)
			}
		}

		content, err := base64.StdEncoding.DecodeString(profile.Attributes.ProfileContent)
		if err != nil {
			return nil, fmt.Errorf("decoding profile for %s: %w", identifier, err)
		}
		profilePath := filepath.Join(outputDir, profile.Attributes.UUID+".mobileprovision")
		if err := os.WriteFile(profilePath, content, 0o600); err != nil {
			return nil, fmt.Errorf("installing profile for %s: %w", identifier, err)
		}
		results[identifier] = Result{
			Name:   profile.Attributes.Name,
			Path:   profilePath,
			Reused: reused,
			UUID:   profile.Attributes.UUID,
		}
	}
	return results, nil
}

func (c *Client) bundleID(ctx context.Context, identifier string) (bundleIDResource, error) {
	query := url.Values{"filter[identifier]": {identifier}, "limit": {"2"}}
	bundles, err := listResources[bundleIDResource](ctx, c, "/v1/bundleIds?"+query.Encode())
	if err != nil {
		return bundleIDResource{}, err
	}
	var matches []bundleIDResource
	for _, bundle := range bundles {
		if bundle.Attributes.Identifier == identifier {
			matches = append(matches, bundle)
		}
	}
	if len(matches) != 1 {
		return bundleIDResource{}, fmt.Errorf("expected exactly one bundle ID %s, found %d", identifier, len(matches))
	}
	return matches[0], nil
}

func (c *Client) profile(ctx context.Context, id string) (profileResource, error) {
	var response resourceResponse[profileResource]
	if err := c.request(ctx, http.MethodGet, "/v1/profiles/"+url.PathEscape(id), nil, &response); err != nil {
		return profileResource{}, err
	}
	return response.Data, nil
}

func (c *Client) createProfile(ctx context.Context, name, bundleID, certificateID string, deviceIDs []string) (profileResource, error) {
	devices := make([]map[string]string, 0, len(deviceIDs))
	for _, id := range deviceIDs {
		devices = append(devices, map[string]string{"id": id, "type": "devices"})
	}
	body := map[string]any{
		"data": map[string]any{
			"attributes": map[string]string{"name": name, "profileType": "IOS_APP_ADHOC"},
			"relationships": map[string]any{
				"bundleId":     map[string]any{"data": map[string]string{"id": bundleID, "type": "bundleIds"}},
				"certificates": map[string]any{"data": []map[string]string{{"id": certificateID, "type": "certificates"}}},
				"devices":      map[string]any{"data": devices},
			},
			"type": "profiles",
		},
	}
	var response resourceResponse[profileResource]
	if err := c.request(ctx, http.MethodPost, "/v1/profiles", body, &response); err != nil {
		return profileResource{}, err
	}
	return response.Data, nil
}

func (c *Client) deleteStaleProfiles(ctx context.Context, profiles []profileResource, name string) error {
	for _, profile := range profiles {
		if profile.Attributes.Name != name {
			continue
		}
		if err := c.request(ctx, http.MethodDelete, "/v1/profiles/"+url.PathEscape(profile.ID), nil, nil); err != nil {
			return err
		}
	}
	return nil
}

func (c *Client) reuseAfterCreateConflict(ctx context.Context, name string) (profileResource, bool) {
	profiles, err := listResources[profileResource](ctx, c, "/v1/profiles?filter%5BprofileType%5D=IOS_APP_ADHOC&limit=200")
	if err != nil {
		return profileResource{}, false
	}
	return reusableProfile(profiles, name, c.now())
}

func listResources[T any](ctx context.Context, c *Client, path string) ([]T, error) {
	var resources []T
	next := path
	for next != "" {
		var response listResponse[T]
		if err := c.request(ctx, http.MethodGet, next, nil, &response); err != nil {
			return nil, err
		}
		resources = append(resources, response.Data...)
		next = response.Links.Next
	}
	return resources, nil
}

func (c *Client) request(ctx context.Context, method, path string, body any, response any) error {
	endpoint := path
	if !strings.HasPrefix(endpoint, "http://") && !strings.HasPrefix(endpoint, "https://") {
		endpoint = strings.TrimRight(c.baseURL, "/") + "/" + strings.TrimLeft(path, "/")
	}
	var encoded io.Reader
	if body != nil {
		var buffer bytes.Buffer
		if err := json.NewEncoder(&buffer).Encode(body); err != nil {
			return fmt.Errorf("encoding App Store Connect request: %w", err)
		}
		encoded = &buffer
	}
	req, err := http.NewRequestWithContext(ctx, method, endpoint, encoded)
	if err != nil {
		return fmt.Errorf("creating App Store Connect request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+c.token)
	req.Header.Set("Content-Type", "application/json")

	res, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("calling App Store Connect: %w", err)
	}
	defer func() { _ = res.Body.Close() }()
	if res.StatusCode < 200 || res.StatusCode >= 300 {
		message, _ := io.ReadAll(io.LimitReader(res.Body, 8192))
		return fmt.Errorf("the App Store Connect API returned %s: %s", res.Status, strings.TrimSpace(string(message)))
	}
	if response == nil || res.StatusCode == http.StatusNoContent {
		return nil
	}
	if err := json.NewDecoder(res.Body).Decode(response); err != nil {
		return fmt.Errorf("decoding App Store Connect response: %w", err)
	}
	return nil
}

func findCertificate(certificates []certificateResource, raw []byte) (certificateResource, error) {
	var matches []certificateResource
	for _, certificate := range certificates {
		remote, err := base64.StdEncoding.DecodeString(certificate.Attributes.CertificateContent)
		if err == nil && bytes.Equal(remote, raw) {
			matches = append(matches, certificate)
		}
	}
	if len(matches) != 1 {
		return certificateResource{}, fmt.Errorf("expected exactly one App Store Connect certificate matching the imported identity, found %d", len(matches))
	}
	return matches[0], nil
}

func enabledIOSDeviceIDs(devices []deviceResource) []string {
	var ids []string
	for _, device := range devices {
		if device.Attributes.Platform == "IOS" && device.Attributes.Status == "ENABLED" {
			ids = append(ids, device.ID)
		}
	}
	sort.Strings(ids)
	return ids
}

func profileName(bundleID, certificateID string, deviceIDs, capabilities []string) string {
	ids := append([]string(nil), deviceIDs...)
	sort.Strings(ids)
	membership := []string{"bundle:" + bundleID, "certificate:" + certificateID}
	for _, id := range ids {
		membership = append(membership, "device:"+id)
	}
	for _, capability := range capabilities {
		membership = append(membership, "capability:"+capability)
	}
	digest := sha256.Sum256([]byte(strings.Join(membership, "\n")))
	return fmt.Sprintf("Quill Ad Hoc %s %s", bundleID, hex.EncodeToString(digest[:6]))
}

func capabilityMembership(capabilities []capabilityResource) []string {
	var membership []string
	for _, capability := range capabilities {
		prefix := capability.ID + ":" + capability.Attributes.CapabilityType
		membership = append(membership, prefix)
		for _, setting := range capability.Attributes.Settings {
			membership = append(membership, prefix+":"+setting.Key)
			for _, option := range setting.Options {
				membership = append(membership, fmt.Sprintf("%s:%s:%t", prefix, option.Key, option.Enabled))
			}
		}
	}
	sort.Strings(membership)
	return membership
}

func reusableProfile(profiles []profileResource, name string, now time.Time) (profileResource, bool) {
	for _, profile := range profiles {
		if profile.Attributes.Name == name && profile.Attributes.ProfileState == "ACTIVE" && profile.Attributes.ExpirationDate.After(now) {
			return profile, true
		}
	}
	return profileResource{}, false
}

func signedToken(issuerID, keyID string, privateKey []byte, now time.Time) (string, error) {
	block, _ := pem.Decode(privateKey)
	if block == nil {
		return "", fmt.Errorf("the App Store Connect private key is not PEM")
	}
	parsed, err := x509.ParsePKCS8PrivateKey(block.Bytes)
	if err != nil {
		return "", fmt.Errorf("parsing App Store Connect private key: %w", err)
	}
	key, ok := parsed.(*ecdsa.PrivateKey)
	if !ok {
		return "", fmt.Errorf("the App Store Connect private key is not EC")
	}

	header, _ := json.Marshal(struct {
		Algorithm string `json:"alg"`
		KeyID     string `json:"kid"`
		Type      string `json:"typ"`
	}{"ES256", keyID, "JWT"})
	payload, _ := json.Marshal(struct {
		Audience  string `json:"aud"`
		ExpiresAt int64  `json:"exp"`
		Issuer    string `json:"iss"`
		IssuedAt  int64  `json:"iat"`
	}{"appstoreconnect-v1", now.Add(15 * time.Minute).Unix(), issuerID, now.Unix()})
	unsigned := rawBase64(header) + "." + rawBase64(payload)
	digest := sha256.Sum256([]byte(unsigned))
	r, s, err := ecdsa.Sign(rand.Reader, key, digest[:])
	if err != nil {
		return "", fmt.Errorf("signing App Store Connect token: %w", err)
	}
	signature := make([]byte, 64)
	r.FillBytes(signature[:32])
	s.FillBytes(signature[32:])
	return unsigned + "." + rawBase64(signature), nil
}

func rawBase64(value []byte) string {
	return base64.RawURLEncoding.EncodeToString(value)
}
