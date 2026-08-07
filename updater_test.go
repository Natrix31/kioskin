package main

import "testing"

func TestAssetURL(t *testing.T) {
	rel := &ghRelease{Assets: []ghAsset{
		{Name: "kioskin.exe", URL: "https://example/exe"},
		{Name: "kioskin.exe.sha256", URL: "https://example/sum"},
	}}
	if got := rel.assetURL("kioskin.exe"); got != "https://example/exe" {
		t.Errorf("assetURL(exe) = %q", got)
	}
	if got := rel.assetURL("kioskin.exe.sha256"); got != "https://example/sum" {
		t.Errorf("assetURL(sum) = %q", got)
	}
	if got := rel.assetURL("missing"); got != "" {
		t.Errorf("assetURL(missing) = %q, want empty", got)
	}
}
