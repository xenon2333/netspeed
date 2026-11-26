package output

import "testing"

func TestFormatBitsPerSecond(t *testing.T) {
	cases := []struct {
		bps  float64
		want string
	}{
		{0, "0.00 bit/s"},
		{999, "999.00 bit/s"},
		{1000, "1.00 kbit/s"},
		{1500, "1.50 kbit/s"},
		{1e6, "1.00 Mbit/s"},
		{95_370_000, "95.37 Mbit/s"},
		{1.5e9, "1.50 Gbit/s"},
	}
	for _, c := range cases {
		if got := FormatBitsPerSecond(c.bps); got != c.want {
			t.Errorf("FormatBitsPerSecond(%v) = %q, want %q", c.bps, got, c.want)
		}
	}
}

func TestFormatBytes(t *testing.T) {
	cases := []struct {
		n    int64
		want string
	}{
		{0, "0 B"},
		{1023, "1023 B"},
		{1024, "1.00 KiB"},
		{1536, "1.50 KiB"},
		{1 << 20, "1.00 MiB"},
		{1 << 30, "1.00 GiB"},
		{1 << 40, "1.00 TiB"},
		{1 << 50, "1.00 PiB"},
		{1 << 60, "1.00 EiB"},
		{125_000_000, "119.21 MiB"},
	}
	for _, c := range cases {
		if got := FormatBytes(c.n); got != c.want {
			t.Errorf("FormatBytes(%d) = %q, want %q", c.n, got, c.want)
		}
	}
}
