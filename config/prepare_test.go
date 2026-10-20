package config

import (
	"testing"
)

func sourceKeys(slices []SourceSlice) []string {
	keys := make([]string, 0, len(slices))
	for _, item := range slices {
		keys = append(keys, item.Key)
	}
	return keys
}

func TestPrepareConfigDeterministicOrder(t *testing.T) {
	config = Config{
		Sources: map[string]Source{
			"go":           {UA: "Go-http-client", Mirror: "https://proxy.golang.org"},
			"golang":       {Path: "/go", Mirror: "https://dl.google.com"},
			"npm":          {UA: "npm", Mirror: "https://registry.npmjs.org"},
			"pypi_package": {Priority: 2, UA: "pip", Path: "/packages", Mirror: "https://files.pythonhosted.org"},
		},
		HostAliases: map[string]string{},
	}

	if err := prepareConfig(); err != nil {
		t.Fatalf("prepareConfig failed: %v", err)
	}

	// priority DESC, then path length DESC, then key ASC.
	want := []string{"pypi_package", "golang", "go", "npm"}
	got := sourceKeys(SourceSlices)
	if len(got) != len(want) {
		t.Fatalf("unexpected source count: %v", got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("position %d: want %s, got %v", i, want[i], got)
		}
	}

	// Rebuilding from the map must never shuffle same-priority sources.
	for round := 0; round < 50; round++ {
		if err := prepareConfig(); err != nil {
			t.Fatalf("prepareConfig round %d failed: %v", round, err)
		}
		if SourceSlices[1].Key != "golang" || SourceSlices[2].Key != "go" {
			t.Fatalf("unstable order at round %d: %v", round, sourceKeys(SourceSlices))
		}
	}
}

func TestValidateSourceStripPathRequiresPath(t *testing.T) {
	config = Config{
		Sources: map[string]Source{
			"bad": {UA: "x", StripPath: true, Mirror: "https://example.com"},
		},
	}
	if err := prepareConfig(); err == nil {
		t.Fatal("expected error for strip_path without path")
	}
}

func TestValidateSourceRejectsTemplateFields(t *testing.T) {
	config = Config{
		Sources: map[string]Source{
			"bad": {Template: true, Mirror: "https://example.com"},
		},
	}
	if err := prepareConfig(); err == nil {
		t.Fatal("expected error for template fields in sources")
	}
}
