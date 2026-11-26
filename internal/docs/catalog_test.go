package docs

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestGeneratedCatalog(t *testing.T) {
	catalog, err := Load(filepath.Join("..", "..", "docs.json"))
	if err != nil {
		t.Fatal(err)
	}
	if len(catalog.Entries) < 100 {
		t.Fatalf("catalog has only %d entries", len(catalog.Entries))
	}
	for _, entry := range catalog.Entries {
		if catalog.ByID(entry.ID) != entry {
			t.Errorf("id lookup failed for %s", entry.Path)
		}
		if strings.HasPrefix(entry.Description, "Documentation for ") {
			t.Errorf("fabricated description for %s", entry.Path)
		}
		if len(entry.Languages) > 0 && entry.Content != nil {
			t.Errorf("duplicated default content for %s", entry.Path)
		}
		if entry.Path == "/PyTgCalls/Using Proxies.xml" && (!strings.Contains(entry.Description, "<h2>Troubleshooting</h2>") || !strings.Contains(entry.Description, "<ol><li>") || !strings.Contains(entry.Description, "HTTP Error 403")) {
			t.Errorf("proxy guide is missing source sections or its numbered list")
		}
		if entry.Path == "/PyTgCalls/Using Filters.xml" && (!strings.Contains(entry.Description, "<h2>Combining Filters</h2>") || !strings.Contains(entry.Description, "<ul><li>")) {
			t.Errorf("filters guide is missing source sections or its list")
		}
		if entry.Path == "/PyTgCalls/Examples.xml" && (!strings.Contains(entry.Description, "<table bordered striped compact><tr><th>Example</th><th>Description</th></tr>") || strings.Count(entry.Description, "<tr>") != 27) {
			t.Errorf("examples guide is missing its source table")
		}
	}
	results := catalog.Search("create remove block", 5)
	if len(results) == 0 || results[0].Title != "Create Remove Block" {
		t.Fatalf("search did not find current documentation page: %+v", results)
	}
}
