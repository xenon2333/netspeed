package cli

import "testing"

func TestParseSize(t *testing.T) {
	cases := []struct {
		in      string
		want    int64
		wantErr bool
	}{
		{"1024", 1024, false},
		{"64KiB", 64 << 10, false},
		{"64kib", 64 << 10, false},
		{"4MiB", 4 << 20, false},
		{"1GiB", 1 << 30, false},
		{"1500KB", 1_500_000, false},
		{"2MB", 2_000_000, false},
		{"512B", 512, false},
		{"1.5MiB", 1572864, false},
		{" 64 KiB ", 64 << 10, false},
		{"", 0, true},
		{"abc", 0, true},
		{"10XB", 0, true},
		{"0", 0, true},
		{"-1", 0, true},
		{"1.5.5MiB", 0, true},
		{"1e3", 0, true},
		{"9999999999GiB", 0, true},
	}
	for _, c := range cases {
		got, err := parseSize(c.in)
		if c.wantErr {
			if err == nil {
				t.Errorf("parseSize(%q) = %d, want error", c.in, got)
			}
			continue
		}
		if err != nil {
			t.Errorf("parseSize(%q) unexpected error: %v", c.in, err)
			continue
		}
		if got != c.want {
			t.Errorf("parseSize(%q) = %d, want %d", c.in, got, c.want)
		}
	}
}
