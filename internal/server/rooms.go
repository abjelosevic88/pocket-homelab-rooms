package server

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"math"
	"net/http"
	"regexp"
	"strings"
	"time"

	"github.com/abjelosevic88/pocket-homelab-rooms/internal/store"
)

var reColor = regexp.MustCompile(`^#[0-9a-fA-F]{6}$`)

// userErr is a reason the person can act on (400, not 502).
type userErr struct{ msg string }

func (e *userErr) Error() string { return e.msg }

// today is the local day (the box's zone).
func today() string { return time.Now().Format("2006-01-02") }

func (s *Server) settingsOf(ctx context.Context, uid int64) store.Settings {
	st, _ := s.store.GetSettings(ctx, uid)
	return st
}

func (s *Server) userFail(w http.ResponseWriter, err error, what string) {
	var ue *userErr
	if errors.As(err, &ue) {
		writeErr(w, http.StatusBadRequest, ue.msg)
		return
	}
	if isNotConnected(err) {
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}
	s.fail(w, err, what)
}

// link is a page of Rooms as a full address (for messages and events).
func (s *Server) link(path string) string {
	return strings.TrimRight(s.cfg.BaseURL, "/") + path
}

func niceDate(d string) string {
	t, err := store.ParseDate(d)
	if err != nil {
		return d
	}
	return t.Format("Mon 2 Jan")
}

func plural(n int, one, many string) string {
	if n == 1 {
		return one
	}
	return many
}

func photoURL(id *int64) (string, string) {
	if id == nil || *id == 0 {
		return "", ""
	}
	u := fmt.Sprintf("/api/v1/photos/%d", *id)
	return u, u + "?thumb=1"
}

// sourceOf is who adds a row: the acting app for a link token, "" here.
func sourceOf(ctx context.Context) string { return linkApp(ctx) }

// mayChange says whether the caller may change a row with this source: a
// link token only what its own app added.
func mayChange(ctx context.Context, source string) bool {
	app := linkApp(ctx)
	return app == "" || app == source
}

// text limits for what people type
const (
	maxName  = 120
	maxLine  = 300
	maxNotes = 40000
)

func cleanText(v string, n int) string { return clip(strings.TrimSpace(v), n) }

func firstLine(s string) string {
	if i := strings.IndexByte(s, '\n'); i >= 0 {
		return strings.TrimSpace(s[:i])
	}
	return s
}

func firstLines(text string, n int) []string {
	out := []string{}
	for _, l := range strings.Split(text, "\n") {
		if l = strings.TrimSpace(l); l != "" && len(out) < n {
			out = append(out, clip(l, 200))
		}
	}
	return out
}

func newToken() string {
	b := make([]byte, 16)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}

// ---- lengths and areas in words ----

// m2 is an area in square metres (or square feet) for messages.
func areaText(cm2 float64, units string) string {
	if units == "imperial" {
		return fmt.Sprintf("%.0f sq ft", cm2/929.0304)
	}
	return fmt.Sprintf("%.1f m²", cm2/10000)
}

// lenText is a length for messages: 2.45 m, 85 cm or 8′ 2″.
func lenText(cm float64, units string) string {
	if units == "imperial" {
		in := cm / 2.54
		ft := math.Floor(in / 12)
		rest := math.Round(in - ft*12)
		if rest == 12 {
			ft, rest = ft+1, 0
		}
		if ft == 0 {
			return fmt.Sprintf("%.0f″", rest)
		}
		return fmt.Sprintf("%.0f′ %.0f″", ft, rest)
	}
	if cm >= 100 {
		return strings.TrimRight(strings.TrimRight(fmt.Sprintf("%.2f", cm/100), "0"), ".") + " m"
	}
	return fmt.Sprintf("%.0f cm", cm)
}

func sizeText(w, d, h float64, units string) string {
	out := lenText(w, units) + " × " + lenText(d, units)
	if h > 0 {
		out += " × " + lenText(h, units)
	}
	return out
}

func money(minor int64, cur string) string {
	return fmt.Sprintf("%s %.2f", cur, float64(minor)/100)
}

func clampF(v, lo, hi float64) float64 {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}
