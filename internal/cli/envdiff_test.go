package cli

import "testing"

func TestEnvDiffStatus(t *testing.T) {
	cases := []struct {
		name string
		va   string
		oka  bool
		vb   string
		okb  bool
		want string
	}{
		{"only on a", "x", true, "", false, "only_production"},
		{"only on b", "", false, "y", true, "only_preview"},
		{"same value", "x", true, "x", true, "same"},
		{"different value", "x", true, "y", true, "different"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := envDiffStatus(c.va, c.oka, c.vb, c.okb, "production", "preview")
			if got != c.want {
				t.Fatalf("envDiffStatus = %q, want %q", got, c.want)
			}
		})
	}
}

func TestEnvDiffRows(t *testing.T) {
	byKey := map[string]map[string]string{
		"SAME":      {"production": "v", "preview": "v"}, // dropped (no diff)
		"DIFFERENT": {"production": "a", "preview": "b"},
		"ONLY_PROD": {"production": "p"},
		"ONLY_PREV": {"preview": "q"},
	}
	rows := envDiffRows(byKey, "production", "preview")

	// SAME is excluded; the rest remain, sorted by key.
	if len(rows) != 3 {
		t.Fatalf("got %d rows, want 3: %v", len(rows), rows)
	}
	keys := make([]string, len(rows))
	for i, r := range rows {
		keys[i] = r.(map[string]any)["key"].(string)
	}
	want := []string{"DIFFERENT", "ONLY_PREV", "ONLY_PROD"}
	for i := range want {
		if keys[i] != want[i] {
			t.Fatalf("rows not sorted: got %v, want %v", keys, want)
		}
	}

	diff := rows[0].(map[string]any)
	if diff["status"] != "different" || diff["production"] != "a" || diff["preview"] != "b" {
		t.Fatalf("different row = %v", diff)
	}
	onlyProd := rows[2].(map[string]any)
	if onlyProd["status"] != "only_production" || onlyProd["production"] != "p" {
		t.Fatalf("only_production row = %v", onlyProd)
	}
	// the absent side is not added as a key
	if _, has := onlyProd["preview"]; has {
		t.Fatalf("only_production row should omit the preview key: %v", onlyProd)
	}
}
