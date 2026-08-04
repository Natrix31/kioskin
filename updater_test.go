package main

import "testing"

func TestParseChecksum(t *testing.T) {
	valid := "3b1f2e4d5c6b7a8901234567890abcdef1234567890abcdef1234567890abcde"
	cases := []struct {
		name    string
		in      string
		want    string
		wantErr bool
	}{
		{"только hex", valid, valid, false},
		{"hex + имя файла", valid + "  kioskin.exe", valid, false},
		{"верхний регистр", "3B1F2E4D5C6B7A8901234567890ABCDEF1234567890ABCDEF1234567890ABCDE", valid, false},
		{"пусто", "   \n", "", true},
		{"короткая", "abc123", "", true},
	}
	for _, c := range cases {
		got, err := parseChecksum([]byte(c.in))
		if c.wantErr {
			if err == nil {
				t.Errorf("%s: ожидалась ошибка", c.name)
			}
			continue
		}
		if err != nil {
			t.Errorf("%s: неожиданная ошибка: %v", c.name, err)
			continue
		}
		if got != c.want {
			t.Errorf("%s: parseChecksum = %q, want %q", c.name, got, c.want)
		}
	}
}

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
