package updater

import (
	"bytes"
	"os"
	"strings"
	"testing"
	"text/template"
)

// goreleaserField extracts the value of a top-level or nested YAML key by
// scanning for the first line (after trimming) that starts with "key:".
// Surrounding quotes are stripped. Returns "" when not found.
func goreleaserField(content, key string) string {
	prefix := key + ":"
	for line := range strings.SplitSeq(content, "\n") {
		trimmed := strings.TrimSpace(line)
		if after, ok := strings.CutPrefix(trimmed, prefix); ok {
			return strings.Trim(strings.TrimSpace(after), `"'`)
		}
	}
	return ""
}

// archiveNameTemplate extracts the name_template from the archives: section of
// goreleaser YAML. It enters the section on the "archives:" top-level key and
// stops at the next non-blank, non-indented line — making it resilient to other
// name_template entries (e.g. the checksum block) that appear elsewhere in the file.
func archiveNameTemplate(content string) string {
	inArchives := false
	prefix := "name_template:"
	for line := range strings.SplitSeq(content, "\n") {
		if line == "archives:" {
			inArchives = true
			continue
		}
		if inArchives {
			// A non-blank, non-indented line signals a new top-level YAML key.
			if len(line) > 0 && line[0] != ' ' {
				break
			}
			trimmed := strings.TrimSpace(line)
			if after, ok := strings.CutPrefix(trimmed, prefix); ok && strings.Contains(trimmed, "{{") {
				return strings.Trim(strings.TrimSpace(after), `"'`)
			}
		}
	}
	return ""
}

// TestAssetNameContract verifies that the updater's assetNameFor helper produces
// exactly the filename goreleaser would generate for every release target.
// Any edit to .goreleaser.yaml's name_template that drifts from updater.go will
// fail this test in CI before it can silently break rimba update.
func TestAssetNameContract(t *testing.T) {
	data, err := os.ReadFile("../../.goreleaser.yaml")
	if err != nil {
		t.Fatalf("reading .goreleaser.yaml: %v", err)
	}
	content := string(data)

	projectName := goreleaserField(content, "project_name")
	if projectName == "" {
		t.Fatal("project_name not found in .goreleaser.yaml")
	}

	nameTmplStr := archiveNameTemplate(content)
	if nameTmplStr == "" {
		t.Fatal("archive name_template not found in .goreleaser.yaml")
	}

	// Verify the checksum filename is what Check() looks for.
	if !strings.Contains(content, "name_template: checksums.txt") {
		t.Error("goreleaser checksum name_template is not 'checksums.txt'; update checksumsFileName const")
	}

	tmpl, err := template.New("asset").Parse(nameTmplStr)
	if err != nil {
		t.Fatalf("parsing name_template %q: %v", nameTmplStr, err)
	}

	const contractVersion = "1.0.0"

	targets := []struct {
		goos   string
		goarch string
	}{
		{"linux", "amd64"},
		{"linux", "arm64"},
		{"darwin", "amd64"},
		{"darwin", "arm64"},
		{goosWindows, "amd64"},
		{goosWindows, "arm64"},
	}

	for _, tt := range targets {
		t.Run(tt.goos+"_"+tt.goarch, func(t *testing.T) {
			var buf bytes.Buffer
			if err := tmpl.Execute(&buf, map[string]string{
				"ProjectName": projectName,
				"Version":     contractVersion,
				"Os":          tt.goos,
				"Arch":        tt.goarch,
			}); err != nil {
				t.Fatalf("rendering template: %v", err)
			}

			rendered := buf.String()
			if strings.HasSuffix(rendered, ".tar.gz") || strings.HasSuffix(rendered, ".zip") {
				t.Fatalf("goreleaser name_template already embeds an extension (%q); "+
					"update the contract test's extension-append logic", rendered)
			}
			ext := ".tar.gz"
			if tt.goos == goosWindows {
				ext = ".zip"
			}
			got := rendered + ext
			want := assetNameFor(tt.goos, tt.goarch, contractVersion)

			if got != want {
				t.Errorf("goreleaser renders %q but updater builds %q — update assetNameFor() or .goreleaser.yaml", got, want)
			}
		})
	}
}
