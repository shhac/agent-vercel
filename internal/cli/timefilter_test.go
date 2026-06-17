package cli

import (
	"net/url"
	"strconv"
	"testing"
	"time"

	agenterrors "github.com/shhac/agent-vercel/internal/errors"
)

func TestRelativeMS(t *testing.T) {
	now := time.Now()
	approx := func(got int64, want time.Time) bool {
		diff := got - want.UnixMilli()
		return diff > -5000 && diff < 5000 // 5s tolerance
	}

	cases := []struct {
		in     string
		ok     bool
		expect time.Duration // ago
	}{
		{"24h", true, 24 * time.Hour},
		{"7d", true, 7 * 24 * time.Hour},
		{"2w", true, 14 * 24 * time.Hour},
		{"1m", true, time.Minute}, // ParseDuration: a minute, NOT a month
		{"30m", true, 30 * time.Minute},
		{"7", false, 0},   // bare number → rejected
		{"abc", false, 0}, // garbage → rejected
		{"", false, 0},
	}
	for _, c := range cases {
		got, ok := relativeMS(c.in)
		if ok != c.ok {
			t.Fatalf("relativeMS(%q) ok = %v; want %v", c.in, ok, c.ok)
		}
		if ok && !approx(got, now.Add(-c.expect)) {
			t.Fatalf("relativeMS(%q) = %d; want ~%d (%.0f ago)", c.in, got, now.Add(-c.expect).UnixMilli(), c.expect.Seconds())
		}
	}
}

func TestSetTimeFilter(t *testing.T) {
	t.Run("empty leaves key unset", func(t *testing.T) {
		q := url.Values{}
		if err := setTimeFilter(q, "since", ""); err != nil {
			t.Fatal(err)
		}
		if q.Has("since") {
			t.Fatalf("empty input should not set the key: %v", q)
		}
	})

	t.Run("date layout", func(t *testing.T) {
		q := url.Values{}
		if err := setTimeFilter(q, "since", "2026-01-02"); err != nil {
			t.Fatal(err)
		}
		want := strconv.FormatInt(time.Date(2026, 1, 2, 0, 0, 0, 0, time.UTC).UnixMilli(), 10)
		if q.Get("since") != want {
			t.Fatalf("date: got %q want %q", q.Get("since"), want)
		}
	})

	t.Run("RFC3339 layout", func(t *testing.T) {
		q := url.Values{}
		if err := setTimeFilter(q, "until", "2026-01-02T03:04:05Z"); err != nil {
			t.Fatal(err)
		}
		want := strconv.FormatInt(time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC).UnixMilli(), 10)
		if q.Get("until") != want {
			t.Fatalf("rfc3339: got %q want %q", q.Get("until"), want)
		}
	})

	t.Run("relative sets a non-empty cursor", func(t *testing.T) {
		q := url.Values{}
		if err := setTimeFilter(q, "since", "24h"); err != nil {
			t.Fatal(err)
		}
		if q.Get("since") == "" {
			t.Fatal("relative duration should set the key")
		}
	})

	t.Run("garbage is an agent error", func(t *testing.T) {
		q := url.Values{}
		err := setTimeFilter(q, "since", "not-a-time")
		var aerr *agenterrors.APIError
		if !agenterrors.As(err, &aerr) || aerr.FixableBy != agenterrors.FixableByAgent {
			t.Fatalf("garbage input should be an agent error, got %v", err)
		}
	})
}
