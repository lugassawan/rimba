package updater

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"
)

// maxChecksumSize caps the checksums.txt download at 1 MiB — orders of magnitude
// larger than any real goreleaser checksums file.
var maxChecksumSize int64 = 1 << 20

// FetchChecksums downloads the checksums file at url and returns a map of
// filename → lowercase SHA-256 hex. Each line must be in sha256sum(1) format:
// "<hex>  <filename>" (one or two spaces). Lines that don't parse are skipped.
func (u *Updater) FetchChecksums(ctx context.Context, url string) (map[string]string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("creating checksums request: %w", err)
	}

	resp, err := u.Client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("fetching checksums: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("checksums request returned status %d", resp.StatusCode)
	}

	body, err := io.ReadAll(io.LimitReader(resp.Body, maxChecksumSize+1))
	if err != nil {
		return nil, fmt.Errorf("reading checksums: %w", err)
	}
	if int64(len(body)) > maxChecksumSize {
		return nil, fmt.Errorf("checksums response exceeds %d bytes", maxChecksumSize)
	}

	sums := make(map[string]string)
	for line := range strings.SplitSeq(string(body), "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) < 2 {
			continue
		}
		hex := strings.ToLower(fields[0])
		filename := fields[len(fields)-1]
		sums[filename] = hex
	}
	return sums, nil
}

// verifyChecksumMatch checks that gotHex matches the expected SHA-256 for assetName.
// An absent row is treated as a hard error (fail-closed).
func verifyChecksumMatch(sums map[string]string, assetName, gotHex string) error {
	expected, ok := sums[assetName]
	if !ok {
		return fmt.Errorf("asset %q not found in checksums.txt", assetName)
	}
	if strings.ToLower(gotHex) != expected {
		return fmt.Errorf("checksum mismatch for %s: got %s, want %s", assetName, gotHex, expected)
	}
	return nil
}
