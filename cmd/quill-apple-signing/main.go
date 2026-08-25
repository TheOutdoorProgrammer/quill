package main

import (
	"context"
	"crypto/x509"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/TheOutdoorProgrammer/quill/internal/actions"
	"github.com/TheOutdoorProgrammer/quill/internal/appleprofiles"
)

const (
	appleCertificatePathVar = "QUILL_APPLE_CERTIFICATE_PATH"
	ascIssuerIDVar          = "QUILL_ASC_ISSUER_ID"
	ascKeyIDVar             = "QUILL_ASC_KEY_ID"
	ascPrivateKeyPathVar    = "QUILL_ASC_PRIVATE_KEY_PATH"
	bundleIdentifiersVar    = "QUILL_BUNDLE_IDENTIFIERS"
	profileOutputDirVar     = "QUILL_PROFILE_OUTPUT_DIR"
)

var bundleIdentifier = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9.-]*$`)

func main() {
	if err := run(); err != nil {
		actions.Errorf("%v", err)
		os.Exit(1)
	}
}

func run() error {
	if len(os.Args) != 1 {
		return fmt.Errorf("quill-apple-signing takes no arguments")
	}

	bundles, err := parseBundleIdentifiers(os.Getenv(bundleIdentifiersVar))
	if err != nil {
		return err
	}
	certificatePath, err := requiredEnv(appleCertificatePathVar)
	if err != nil {
		return err
	}
	certificate, err := readCertificate(certificatePath)
	if err != nil {
		return err
	}
	privateKeyPath, err := requiredEnv(ascPrivateKeyPathVar)
	if err != nil {
		return err
	}
	privateKey, err := os.ReadFile(privateKeyPath)
	if err != nil {
		return fmt.Errorf("reading App Store Connect private key: %w", err)
	}
	issuerID, err := requiredEnv(ascIssuerIDVar)
	if err != nil {
		return err
	}
	keyID, err := requiredEnv(ascKeyIDVar)
	if err != nil {
		return err
	}
	client, err := appleprofiles.NewClient(issuerID, keyID, privateKey)
	if err != nil {
		return err
	}

	outputDir := os.Getenv(profileOutputDirVar)
	if outputDir == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return fmt.Errorf("finding home directory: %w", err)
		}
		outputDir = filepath.Join(home, "Library", "MobileDevice", "Provisioning Profiles")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	results, err := client.Ensure(ctx, bundles, certificate, outputDir)
	if err != nil {
		return err
	}

	encoded, err := json.Marshal(results)
	if err != nil {
		return fmt.Errorf("encoding provisioning profile outputs: %w", err)
	}
	if err := actions.Output("profiles", string(encoded)); err != nil {
		return err
	}
	if len(bundles) == 1 {
		result := results[bundles[0]]
		if err := actions.Output("profile-name", result.Name); err != nil {
			return err
		}
		if err := actions.Output("profile-uuid", result.UUID); err != nil {
			return err
		}
	}

	for _, bundle := range bundles {
		verb := "created"
		if results[bundle].Reused {
			verb = "reused"
		}
		actions.Noticef("%s Ad Hoc profile for %s", verb, bundle)
	}
	return nil
}

func requiredEnv(name string) (string, error) {
	value := strings.TrimSpace(os.Getenv(name))
	if value == "" {
		return "", fmt.Errorf("%s is required", name)
	}
	return value, nil
}

func parseBundleIdentifiers(raw string) ([]string, error) {
	seen := make(map[string]bool)
	var identifiers []string
	for _, value := range strings.FieldsFunc(raw, func(r rune) bool {
		return r == ',' || r == '\n' || r == '\r' || r == '\t' || r == ' '
	}) {
		if !bundleIdentifier.MatchString(value) {
			return nil, fmt.Errorf("invalid bundle identifier %q", value)
		}
		if !seen[value] {
			identifiers = append(identifiers, value)
			seen[value] = true
		}
	}
	if len(identifiers) == 0 {
		return nil, fmt.Errorf("%s is required", bundleIdentifiersVar)
	}
	sort.Strings(identifiers)
	return identifiers, nil
}

func readCertificate(path string) (*x509.Certificate, error) {
	contents, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading Apple Distribution certificate: %w", err)
	}
	for len(contents) > 0 {
		block, rest := pem.Decode(contents)
		if block == nil {
			break
		}
		contents = rest
		if block.Type != "CERTIFICATE" {
			continue
		}
		certificate, err := x509.ParseCertificate(block.Bytes)
		if err != nil {
			return nil, fmt.Errorf("parsing Apple Distribution certificate: %w", err)
		}
		return certificate, nil
	}
	return nil, fmt.Errorf("distribution certificate file contains no certificate")
}
