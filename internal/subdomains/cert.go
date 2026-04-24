package subdomains

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/sirschubert/cyclops/internal/utils"
)

// CertificateResponse represents a single record from the crt.sh JSON API.
type CertificateResponse struct {
	NameValue  string `json:"name_value"`
	CommonName string `json:"common_name"`
	IssuerName string `json:"issuer_name"`
	NotBefore  string `json:"not_before"`
	NotAfter   string `json:"not_after"`
}

// CertTransparency queries certificate transparency logs (crt.sh) for subdomains.
// It retries up to 3 times with exponential backoff to handle transient crt.sh failures.
func CertTransparency(ctx context.Context, domain string) ([]string, error) {
	ctURL := fmt.Sprintf("https://crt.sh/?q=%%25.%s&output=json", domain)
	client := &http.Client{Timeout: 30 * time.Second}

	var body []byte
	err := utils.RetryWithBackoff(ctx, 3, func() error {
		req, err := http.NewRequestWithContext(ctx, "GET", ctURL, nil)
		if err != nil {
			return err
		}
		req.Header.Set("User-Agent", "Cyclops/1.0")

		resp, err := client.Do(req)
		if err != nil {
			return err
		}
		defer resp.Body.Close()

		if resp.StatusCode >= 500 {
			return fmt.Errorf("crt.sh returned %d", resp.StatusCode)
		}

		body, err = io.ReadAll(resp.Body)
		return err
	})
	if err != nil {
		return nil, err
	}

	if len(body) == 0 || string(body) == "[]\n" {
		return []string{}, nil
	}

	var certs []CertificateResponse
	if err := json.Unmarshal(body, &certs); err != nil {
		return nil, err
	}

	subdomainsMap := make(map[string]bool)
	for _, cert := range certs {
		if cert.NameValue != "" {
			for _, name := range strings.Split(cert.NameValue, "\n") {
				name = strings.TrimSpace(name)
				if name != "" && (strings.HasSuffix(name, "."+domain) || name == domain) {
					subdomainsMap[name] = true
				}
			}
		}
		if cert.CommonName != "" && (strings.HasSuffix(cert.CommonName, "."+domain) || cert.CommonName == domain) {
			subdomainsMap[cert.CommonName] = true
		}
	}

	result := make([]string, 0, len(subdomainsMap))
	for sub := range subdomainsMap {
		result = append(result, sub)
	}
	return result, nil
}

// QueryCTLogs queries multiple certificate transparency logs.
func QueryCTLogs(ctx context.Context, domain string) ([]string, error) {
	return CertTransparency(ctx, domain)
}
