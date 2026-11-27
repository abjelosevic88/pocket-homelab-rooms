package server

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	core "github.com/abjelosevic88/pocket-homelab-core/sdk"
	"github.com/abjelosevic88/pocket-homelab-rooms/internal/config"
	"github.com/abjelosevic88/pocket-homelab-rooms/internal/store"
)

// fakeCapCore is a Pocket Core that vouches for link tokens
// pcx_<alice|bob>_<search|day|widget|capture|read|write> from Pocket Hub
// and remembers the manifest Rooms registers with.
type fakeCapCore struct {
	*httptest.Server
	mu       sync.Mutex
	manifest map[string]any
}

func newCapApp(t *testing.T) (*app, *fakeCapCore) {
	t.Helper()
	fc := &fakeCapCore{}
	scopes := map[string]string{"search": core.ScopeSearch, "day": core.ScopeDay, "widget": core.ScopeWidget, "capture": core.ScopeCapture,
		"read": "rooms:read", "write": "rooms:write"}
	fc.Server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/api/v1/apps/register":
			var m map[string]any
			_ = json.NewDecoder(r.Body).Decode(&m)
			fc.mu.Lock()
			fc.manifest = m
			fc.mu.Unlock()
			io.WriteString(w, `{}`)
		case "/api/v1/auth/introspect":
			var in struct{ Token string }
			_ = json.NewDecoder(r.Body).Decode(&in)
			parts := strings.Split(in.Token, "_")
			if len(parts) != 3 || parts[0] != "pcx" || scopes[parts[2]] == "" {
				io.WriteString(w, `{"active":false}`)
				return
			}
			_ = json.NewEncoder(w).Encode(map[string]any{"active": true, "kind": "delegated", "audience": "rooms", "scope": scopes[parts[2]],
				"user": map[string]any{"id": 1, "sub": parts[1] + "@example.com", "email": parts[1] + "@example.com", "name": parts[1], "role": "member"},
				"app":  map[string]any{"id": 9, "slug": "hub", "name": "Pocket Hub"}})
		default:
			io.WriteString(w, `{}`)
		}
	}))
	t.Cleanup(fc.Close)
	return newApp(t, func(c *config.Config) { c.CoreURL, c.CoreRegistrationKey = fc.URL, "key" }), fc
}

// items are the rows of a capability answer.
func items(t *testing.T, m map[string]any) []map[string]any {
	t.Helper()
	l, ok := m["items"].([]any)
	if !ok {
		t.Fatalf("no items list in %v", m)
	}
	out := []map[string]any{}
	for _, v := range l {
		out = append(out, v.(map[string]any))
	}
	return out
}

// item finds the row with that title.
func item(t *testing.T, m map[string]any, title string) map[string]any {
	t.Helper()
	for _, it := range items(t, m) {
		if it["title"] == title {
			return it
		}
	}
	t.Fatalf("no %q in %v", title, m["items"])
	return nil
}

// todo adds a checklist item to a plan; doneAt ticks it off at that moment.
func (a *app) todo(tok, plan, title, due, doneAt string) string {
	a.t.Helper()
	td := a.expect(201, tok, "POST", "/api/v1/plans/"+plan+"/todos", map[string]any{"title": title, "due": due})
	if doneAt != "" {
		if _, err := a.st.DB.Exec(`UPDATE todos SET done = 1, done_at = ? WHERE id = ?`, doneAt, n(id(td))); err != nil {
			a.t.Fatal(err)
		}
	}
	return id(td)
}

func TestPocketSearch(t *testing.T) {
	a := newApp(t, nil)
	ids := flat(a, a.alice)
	got := items(t, a.ok(a.alice, "GET", "/api/v1/pocket/search?q=sofa", nil))
	if len(got) != 1 || got[0]["id"] != "piece:"+ids["piece"] || got[0]["kind"] != "piece" || got[0]["title"] != "Sofa" || got[0]["snippet"] != "2.2 m × 95 cm × 85 cm" ||
		got[0]["url"] != "/furniture/"+ids["piece"] || got[0]["date"] == nil {
		t.Fatalf("the sofa: %v", got)
	}
	plan := item(t, a.ok(a.alice, "GET", "/api/v1/pocket/search?q=our+flat", nil), "Our flat")
	if plan["id"] != "plan:"+ids["plan"] || plan["kind"] != "plan" || plan["snippet"] != "Flat" || plan["url"] != "/plans/"+ids["plan"] {
		t.Fatalf("the plan: %v", plan)
	}
	// found by its notes: the line says where
	if got := items(t, a.ok(a.alice, "GET", "/api/v1/pocket/search?q=PRIVATE", nil)); len(got) != 1 || got[0]["title"] != "Our flat" || got[0]["snippet"] != "Flat · Private notes" {
		t.Fatalf("by the notes: %v", got)
	}
	a.ok(a.alice, "PUT", "/api/v1/plans/"+ids["plan"], map[string]any{"address": "Main Street 4"})
	if got := items(t, a.ok(a.alice, "GET", "/api/v1/pocket/search?q=main+street", nil)); len(got) != 1 || got[0]["snippet"] != "Main Street 4" {
		t.Fatalf("by the address: %v", got)
	}
	room := item(t, a.ok(a.alice, "GET", "/api/v1/pocket/search?q=bedroom", nil), "Bedroom")
	if room["id"] != "room:"+ids["layout"]+":r3" || room["kind"] != "room" || room["snippet"] != "Our flat · As it is · 14.4 m²" ||
		room["url"] != "/plans/"+ids["plan"]+"?layout="+ids["layout"] {
		t.Fatalf("the room: %v", room)
	}
	// a name that has the words comes before notes that have them, the exact name first
	a.ok(a.alice, "PUT", "/api/v1/plans/"+ids["plan"], map[string]any{"notes": "The bed goes under the window"})
	bed := a.expect(201, a.alice, "POST", "/api/v1/pieces", map[string]any{"name": "Bed", "status": "want", "shop": "Corner shop", "notes": "oak, 160 wide"})
	got = items(t, a.ok(a.alice, "GET", "/api/v1/pocket/search?q=bed", nil))
	if len(got) != 3 || got[0]["id"] != "piece:"+id(bed) || got[0]["snippet"] != "1 m × 60 cm × 80 cm · To buy" || got[1]["title"] != "Bedroom" ||
		got[2]["title"] != "Our flat" || got[2]["snippet"] != "Main Street 4 · The bed goes under the window" {
		t.Fatalf("bed: %v", got)
	}
	if got := items(t, a.ok(a.alice, "GET", "/api/v1/pocket/search?q=oak", nil)); len(got) != 1 || got[0]["snippet"] != "1 m × 60 cm × 80 cm · To buy · Corner shop · oak, 160 wide" {
		t.Fatalf("a piece by its notes: %v", got)
	}
	if got := items(t, a.ok(a.alice, "GET", "/api/v1/pocket/search?q=bed&limit=1", nil)); len(got) != 1 || got[0]["title"] != "Bed" {
		t.Fatalf("limit 1: %v", got)
	}
	// no words, nothing: not even where the app's own list would match everything
	for _, q := range []string{"", "%3F%3F", "*", "%25", "_"} {
		if got := items(t, a.ok(a.alice, "GET", "/api/v1/pocket/search?q="+q, nil)); len(got) != 0 {
			t.Fatalf("q=%q: %v", q, got)
		}
	}
	// another person's plans, rooms and furniture never appear
	for _, q := range []string{"sofa", "flat", "bed", "hall", "oak", "main"} {
		if got := items(t, a.ok(a.bob, "GET", "/api/v1/pocket/search?q="+q, nil)); len(got) != 0 {
			t.Fatalf("bob finds alice's %q: %v", q, got)
		}
	}
	a.expect(401, "", "GET", "/api/v1/pocket/search?q=sofa", nil)
	// the old route still answers as before
	old := a.ok(a.alice, "GET", "/api/v1/search?q=sofa", nil)["results"].([]any)
	if o := old[0].(map[string]any); len(old) != 1 || id(o) != ids["piece"] || o["kind"] != "piece" || o["title"] != "Sofa" || o["subtitle"] != "2.2 m × 95 cm × 85 cm" ||
		o["path"] != "/furniture/"+ids["piece"] || o["url"] != "http://rooms.test/furniture/"+ids["piece"] {
		t.Fatalf("/search changed: %v", old)
	}
}

func TestPocketDay(t *testing.T) {
	a := newApp(t, nil)
	ids := flat(a, a.alice) // "Measure the hall", due tomorrow
	day := today()
	at := func(clock string) string { // a moment of today, as the box's clock shows it
		m, err := time.ParseInLocation("2006-01-02 15:04", day+" "+clock, time.Local)
		if err != nil {
			t.Fatal(err)
		}
		return m.UTC().Format(time.RFC3339Nano)
	}
	a.todo(a.alice, ids["plan"], "Buy a rug", day, "")
	a.todo(a.alice, ids["plan"], "Book the movers", "", at("12:40"))
	a.todo(a.alice, ids["plan"], "Order the paint", dayAfter(3), at("09:15"))

	d := a.ok(a.alice, "GET", "/api/v1/pocket/day/"+day, nil)
	rows := items(t, d)
	if d["date"] != day || d["summary"] != "2 checklist items done, 1 still due" || len(rows) != 3 {
		t.Fatalf("today: %v", d)
	}
	if it := rows[0]; it["title"] != "Order the paint" || it["kind"] != "done" || it["done"] != true || it["time"] != "09:15" ||
		it["sub"] != "Our flat" || it["url"] != "/plans/"+ids["plan"]+"?tab=checklist" {
		t.Fatalf("ticked off first: %v", it)
	}
	if it := rows[2]; it["title"] != "Buy a rug" || it["kind"] != "due" || it["done"] != nil || it["time"] != nil || it["sub"] != "Our flat" {
		t.Fatalf("still due: %v", it)
	}
	ms := d["metrics"].([]any)
	for i, want := range [][2]string{{"Done", "2"}, {"Due", "1"}} {
		if m := ms[i].(map[string]any); len(ms) != 2 || m["label"] != want[0] || m["value"] != want[1] {
			t.Fatalf("metrics: %v", ms)
		}
	}
	// a day still to come: what is due then; what was ticked off early is on the day it was done
	if d := a.ok(a.alice, "GET", "/api/v1/pocket/day/"+dayAfter(1), nil); d["summary"] != "1 checklist item due" || len(items(t, d)) != 1 || items(t, d)[0]["title"] != "Measure the hall" {
		t.Fatalf("tomorrow: %v", d)
	}
	if d := a.ok(a.alice, "GET", "/api/v1/pocket/day/"+dayAfter(3), nil); d["summary"] != nil || len(items(t, d)) != 0 {
		t.Fatalf("due that day, done before: %v", d)
	}
	// the day is the caller's: half past eleven at night in UTC is the next morning in Sarajevo
	a.todo(a.alice, ids["plan"], "Hand over the keys", "", "2026-03-01T23:30:00.5Z")
	if d := a.ok(a.alice, "GET", "/api/v1/pocket/day/2026-03-01?tz=UTC", nil); d["summary"] != "1 checklist item done" || items(t, d)[0]["time"] != "23:30" {
		t.Fatalf("in UTC: %v", d)
	}
	if d := a.ok(a.alice, "GET", "/api/v1/pocket/day/2026-03-02?tz=Europe/Sarajevo", nil); len(items(t, d)) != 1 || items(t, d)[0]["time"] != "00:30" {
		t.Fatalf("in Sarajevo: %v", d)
	}
	if d := a.ok(a.alice, "GET", "/api/v1/pocket/day/2026-03-01?tz=Europe/Sarajevo", nil); len(items(t, d)) != 0 {
		t.Fatalf("the evening before in Sarajevo: %v", d)
	}
	// a day with nothing, and somebody else's day
	if d := a.ok(a.alice, "GET", "/api/v1/pocket/day/2019-03-04", nil); d["summary"] != nil || d["metrics"] != nil || len(items(t, d)) != 0 {
		t.Fatalf("an empty day: %v", d)
	}
	if d := a.ok(a.bob, "GET", "/api/v1/pocket/day/"+day, nil); d["summary"] != nil || len(items(t, d)) != 0 {
		t.Fatalf("bob sees alice's day: %v", d)
	}
	a.expect(400, a.alice, "GET", "/api/v1/pocket/day/yesterday", nil)
	a.expect(401, "", "GET", "/api/v1/pocket/day/"+day, nil)
	// the old route still answers as before
	old := a.ok(a.alice, "GET", "/api/v1/day/"+dayAfter(1), nil)
	if l := old["items"].([]any); len(l) != 1 || l[0].(map[string]any)["kind"] != "todo" || l[0].(map[string]any)["subtitle"] != "Our flat" || l[0].(map[string]any)["due"] != dayAfter(1) {
		t.Fatalf("/day changed: %v", old)
	}
}

func TestPocketWidget(t *testing.T) {
	a := newApp(t, nil)
	// no plan: nothing to show
	if w := a.ok(a.bob, "GET", "/api/v1/pocket/widget", nil); w["line"] != nil || w["metrics"] != nil || w["url"] != nil || len(items(t, w)) != 0 {
		t.Fatalf("without plans: %v", w)
	}
	ids := flat(a, a.alice) // "Measure the hall", due tomorrow
	checklist := "/plans/" + ids["plan"] + "?tab=checklist"
	w := a.ok(a.alice, "GET", "/api/v1/pocket/widget", nil)
	if w["line"] != "1 thing to do for Our flat" || w["url"] != checklist || len(items(t, w)) != 1 {
		t.Fatalf("one thing: %v", w)
	}
	a.todo(a.alice, ids["plan"], "Pick the paint", "", "")
	late := a.todo(a.alice, ids["plan"], "Call the landlord", dayAfter(-3), "")
	a.todo(a.alice, ids["plan"], "Sign the lease", dayAfter(-9), store.Now())
	w = a.ok(a.alice, "GET", "/api/v1/pocket/widget", nil)
	if w["line"] != "3 things to do for Our flat, 1 late" || w["url"] != checklist {
		t.Fatalf("widget: %v", w)
	}
	ms := w["metrics"].([]any)
	for i, want := range [][2]string{{"To do", "3"}, {"Late", "1"}} {
		if m := ms[i].(map[string]any); len(ms) != 2 || m["label"] != want[0] || m["value"] != want[1] {
			t.Fatalf("metrics: %v", ms)
		}
	}
	rows := items(t, w)
	if len(rows) != 3 || rows[0]["title"] != "Call the landlord" || rows[0]["id"] != "todo:"+late || rows[0]["url"] != checklist ||
		rows[0]["detail"] != "Our flat · "+dueWords(dayAfter(-3), today()) || !strings.HasPrefix(rows[0]["detail"].(string), "Our flat · was due ") ||
		rows[1]["title"] != "Measure the hall" || rows[1]["detail"] != "Our flat · due tomorrow" || rows[2]["title"] != "Pick the paint" || rows[2]["detail"] != "Our flat" {
		t.Fatalf("late first, then the soonest, then what has no day: %v", rows)
	}
	// a second plan; one that was put away no longer counts
	office := a.expect(201, a.alice, "POST", "/api/v1/plans", map[string]any{"name": "Office", "kind": "office", "template": "room"})["plan"].(map[string]any)
	a.todo(a.alice, id(office), "Measure the desk", today(), "")
	w = a.ok(a.alice, "GET", "/api/v1/pocket/widget", nil)
	if rows := items(t, w); w["line"] != "4 things to do on 2 plans, 1 late" || w["url"] != "/" || len(rows) != 4 || rows[1]["detail"] != "Office · due today" {
		t.Fatalf("two plans: %v", w)
	}
	a.ok(a.alice, "PUT", "/api/v1/plans/"+id(office), map[string]any{"archived": true})
	if w := a.ok(a.alice, "GET", "/api/v1/pocket/widget", nil); w["line"] != "3 things to do for Our flat, 1 late" {
		t.Fatalf("with the office put away: %v", w)
	}
	// more than fits: the eight most pressing, all of them counted
	for i := range 9 {
		a.todo(a.alice, ids["plan"], "Box "+itoa(int64(i)), "", "")
	}
	if w := a.ok(a.alice, "GET", "/api/v1/pocket/widget", nil); w["line"] != "12 things to do for Our flat, 1 late" || len(items(t, w)) != 8 || items(t, w)[0]["title"] != "Call the landlord" {
		t.Fatalf("a long checklist: %v", w)
	}
	// nothing to do: the plans changed last
	if w := a.ok(a.bob, "GET", "/api/v1/pocket/widget", nil); len(items(t, w)) != 0 {
		t.Fatalf("bob sees alice's checklist: %v", w)
	}
	first := a.expect(201, a.bob, "POST", "/api/v1/plans", map[string]any{"name": "Cabin", "kind": "house", "status": "idea", "template": "room"})["plan"].(map[string]any)
	second := a.expect(201, a.bob, "POST", "/api/v1/plans", map[string]any{"name": "New flat", "status": "moving", "address": "Main Street 4", "template": "room"})["plan"].(map[string]any)
	if _, err := a.st.DB.Exec(`UPDATE plans SET updated_at = '2026-01-01T00:00:00.000Z' WHERE id = ?`, n(id(first))); err != nil {
		t.Fatal(err)
	}
	w = a.ok(a.bob, "GET", "/api/v1/pocket/widget", nil)
	rows = items(t, w)
	if w["line"] != nil || w["metrics"] != nil || w["url"] != "/" || len(rows) != 2 || rows[0]["title"] != "New flat" || rows[0]["id"] != "plan:"+id(second) ||
		rows[0]["detail"] != "Moving in · Main Street 4" || rows[0]["url"] != "/plans/"+id(second) || rows[1]["title"] != "Cabin" || rows[1]["detail"] != "Idea" {
		t.Fatalf("bob's plans: %v", w)
	}
	a.expect(401, "", "GET", "/api/v1/pocket/widget", nil)
}

// capture posts a quick line and returns the answer.
func capture(a *app, tok, requestID, text string, more map[string]any) (int, map[string]any) {
	body := map[string]any{"request_id": requestID, "kind": "piece", "text": text}
	for k, v := range more {
		body[k] = v
	}
	code, m, _ := a.call(tok, "POST", "/api/v1/pocket/capture", body)
	return code, m
}

func TestPocketCapture(t *testing.T) {
	a := newApp(t, nil)
	keep := func(tok, requestID, text string, more map[string]any) map[string]any {
		t.Helper()
		code, m := capture(a, tok, requestID, text, more)
		if code != 201 {
			t.Fatalf("capture %q → %d %v", text, code, m)
		}
		pid := strings.TrimPrefix(m["id"].(string), "piece:")
		p := a.ok(tok, "GET", "/api/v1/pieces/"+pid, nil)["piece"].(map[string]any)
		if m["title"] != p["name"] || m["url"] != "/furniture/"+pid {
			t.Fatalf("capture %q answered %v for %v", text, m, p)
		}
		return p
	}
	refuse := func(tok, requestID, text, want string) {
		t.Helper()
		if code, m := capture(a, tok, requestID, text, nil); code != 400 || m["error"] != want {
			t.Fatalf("capture %q → %d %v, want 400 %q", text, code, m, want)
		}
	}
	// a name: a piece to buy, with the size every new piece starts with
	code, first := capture(a, a.alice, "req-0000000000000001", "Grey sofa", nil)
	if code != 201 || first["title"] != "Grey sofa" {
		t.Fatalf("capture: %d %v", code, first)
	}
	p := keep(a.alice, "req-0000000000000001", "Grey sofa", nil)
	if "piece:"+id(p) != first["id"] || p["name"] != "Grey sofa" || p["status"] != "want" || p["url"] != "" || p["source"] != "" || p["notes"] != sizeLater ||
		num(p, "width") != 100 || num(p, "depth") != 60 || num(p, "height") != 80 {
		t.Fatalf("the piece: %v", p)
	}
	// the same request again: the same answer, nothing added
	if l := a.list(a.alice, "/api/v1/pieces"); len(l) != 1 {
		t.Fatalf("a retry added a piece: %d", len(l))
	}
	refuse(a.alice, "req-0000000000000001", "Blue sofa", "this was already kept with other words")

	// a web address in the line is the shop's page
	for i, c := range []struct{ text, name, link string }{
		{"Kivik sofa - https://shop.example/kivik-3?x=1.", "Kivik sofa", "https://shop.example/kivik-3?x=1"},
		{"https://shop.example/p/1 Oak table", "Oak table", "https://shop.example/p/1"},
		{"Lamp (http://shop.example/lamp)", "Lamp", "http://shop.example/lamp"},
		{"https://www.Shop.example/p/123", "shop.example", "https://www.Shop.example/p/123"},
		{"Shelf from http://", "Shelf from http://", ""},
	} {
		if p := keep(a.alice, "req-10000000000000"+itoa(int64(10+i)), c.text, nil); p["name"] != c.name || p["url"] != c.link || p["status"] != "want" {
			t.Fatalf("%q → %q %q, want %q %q", c.text, p["name"], p["url"], c.name, c.link)
		}
	}
	// a title, and more lines: the rest is the notes
	if p := keep(a.alice, "req-0000000000000002", "seats six\nhttps://shop.example/oak", map[string]any{"title": "Dining table"}); p["name"] != "Dining table" ||
		p["url"] != "https://shop.example/oak" || p["notes"] != "seats six\nhttps://shop.example/oak\n\n"+sizeLater {
		t.Fatalf("title and text: %v", p)
	}
	if p := keep(a.alice, "req-0000000000000003", "Bookcase\n80 wide at most,\nwhite", nil); p["name"] != "Bookcase" || p["notes"] != "80 wide at most,\nwhite\n\n"+sizeLater {
		t.Fatalf("more lines: %v", p)
	}
	long := strings.Repeat("ž", 150)
	if p := keep(a.alice, "req-0000000000000004", long, nil); len([]rune(p["name"].(string))) != maxName || !strings.HasSuffix(p["name"].(string), "…") ||
		p["notes"] != long+"\n\n"+sizeLater {
		t.Fatalf("a long line: %v", p)
	}

	// what the route checks itself
	before := len(a.list(a.alice, "/api/v1/pieces"))
	a.expect(400, a.alice, "POST", "/api/v1/pocket/capture", map[string]any{"request_id": "short", "text": "x"})
	a.expect(400, a.alice, "POST", "/api/v1/pocket/capture", map[string]any{"request_id": "req-0000000000000009", "kind": "plan", "text": "x"})
	a.expect(400, a.alice, "POST", "/api/v1/pocket/capture", map[string]any{"request_id": "req-0000000000000009", "text": " "})
	refuse(a.alice, "req-0000000000000009", "-", `write what the piece is, like "Grey sofa"`)
	a.expect(401, "", "POST", "/api/v1/pocket/capture", map[string]any{"request_id": "req-0000000000000009", "text": "x"})
	if l := a.list(a.alice, "/api/v1/pieces"); len(l) != before {
		t.Fatal("a refused line was kept")
	}
	// the kind may be left out: there is only one
	if code, m, _ := a.call(a.alice, "POST", "/api/v1/pocket/capture", map[string]any{"request_id": "req-0000000000000009", "text": "Rug"}); code != 201 || m["title"] != "Rug" {
		t.Fatalf("without a kind: %d %v", code, m)
	}

	// per person: a request id is one's own, the piece too
	if p := keep(a.bob, "req-0000000000000001", "Blue sofa", nil); p["name"] != "Blue sofa" {
		t.Fatalf("bob's line: %v", p)
	}
	if l := a.list(a.bob, "/api/v1/pieces"); len(l) != 1 {
		t.Fatalf("bob has %d pieces", len(l))
	}
	if l := a.list(a.alice, "/api/v1/pieces"); len(l) != before+1 {
		t.Fatalf("alice has %d pieces, want %d", len(l), before+1)
	}
	// the app's own way of adding a piece is as before: yours, with the size given
	if p := a.expect(201, a.alice, "POST", "/api/v1/pieces", map[string]any{"name": "Armchair", "width": 80}); p["status"] != "own" || num(p, "width") != 80 || p["notes"] != "" {
		t.Fatalf("POST /pieces changed: %v", p)
	}
}

// A kept line tells the Core bus once, a retry not again.
func TestPocketCaptureEvent(t *testing.T) {
	cfg := testConfig(t.TempDir())
	cfg.CoreURL = "http://core.invalid"
	st := openTestStore(t, cfg)
	s := New(cfg, st, slog.New(slog.NewTextHandler(io.Discard, nil)), nil)
	ctx := context.Background()
	if err := s.core.Init(ctx); err != nil {
		t.Fatal(err)
	}
	u := &store.User{Email: "ana@example.com", Name: "Ana", Role: "admin"}
	if err := st.CreateUser(ctx, u); err != nil {
		t.Fatal(err)
	}
	tok, _, _ := st.CreateAPIToken(ctx, u.ID, "t")
	ts := httptest.NewServer(s.Router())
	t.Cleanup(ts.Close)
	a := &app{t: t, url: ts.URL, s: s, st: st}
	for range 2 {
		if code, m := capture(a, tok, "req-0000000000000001", "Grey sofa", nil); code != 201 {
			t.Fatalf("capture → %d %v", code, m)
		}
	}
	rows, err := st.DB.QueryContext(ctx, `SELECT body FROM pocket_outbox ORDER BY created_at, rowid`)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	added := 0
	for rows.Next() {
		var b []byte
		if err := rows.Scan(&b); err != nil {
			t.Fatal(err)
		}
		var ev struct {
			Topic   string
			Payload map[string]any
		}
		if err := json.Unmarshal(b, &ev); err != nil {
			t.Fatal(err)
		}
		if ev.Topic != "rooms.piece_added" {
			continue
		}
		added++
		if ev.Payload["name"] != "Grey sofa" || ev.Payload["status"] != "want" || ev.Payload["user_email"] != "ana@example.com" {
			t.Errorf("payload = %v", ev.Payload)
		}
	}
	if added != 1 {
		t.Fatalf("piece_added events = %d, want 1", added)
	}
}

// A link token with one pocket:* scope reaches its one route and nothing else.
func TestPocketLinkTokens(t *testing.T) {
	a, _ := newCapApp(t)
	ids := flat(a, a.alice)
	day := dayAfter(1)
	line := func(requestID string) map[string]any {
		return map[string]any{"request_id": requestID, "kind": "piece", "text": "Rug https://shop.example/rug"}
	}
	for _, c := range []struct {
		tok, method, path string
		body              any
		want              int
	}{
		{"pcx_alice_search", "GET", "/api/v1/pocket/search?q=sofa", nil, 200},
		{"pcx_alice_day", "GET", "/api/v1/pocket/day/" + day, nil, 200},
		{"pcx_alice_widget", "GET", "/api/v1/pocket/widget", nil, 200},
		{"pcx_alice_capture", "POST", "/api/v1/pocket/capture", line("req-0000000000000001"), 201},
		{"pcx_alice_widget", "GET", "/api/v1/auth/me", nil, 200},
		// another capability's route
		{"pcx_alice_search", "GET", "/api/v1/pocket/day/" + day, nil, 403},
		{"pcx_alice_search", "GET", "/api/v1/pocket/widget", nil, 403},
		{"pcx_alice_search", "POST", "/api/v1/pocket/capture", line("req-0000000000000002"), 403},
		{"pcx_alice_day", "GET", "/api/v1/pocket/search?q=sofa", nil, 403},
		{"pcx_alice_day", "GET", "/api/v1/pocket/widget", nil, 403},
		{"pcx_alice_widget", "GET", "/api/v1/pocket/search?q=sofa", nil, 403},
		{"pcx_alice_widget", "GET", "/api/v1/pocket/day/" + day, nil, 403},
		{"pcx_alice_widget", "POST", "/api/v1/pocket/capture", line("req-0000000000000003"), 403},
		{"pcx_alice_capture", "GET", "/api/v1/pocket/widget", nil, 403},
		{"pcx_alice_capture", "GET", "/api/v1/pocket/search?q=sofa", nil, 403},
		// the rest of the app
		{"pcx_alice_widget", "GET", "/api/v1/home", nil, 403},
		{"pcx_alice_widget", "GET", "/api/v1/summary", nil, 403},
		{"pcx_alice_widget", "GET", "/api/v1/todos", nil, 403},
		{"pcx_alice_search", "GET", "/api/v1/search?q=sofa", nil, 403},
		{"pcx_alice_search", "GET", "/api/v1/plans", nil, 403},
		{"pcx_alice_search", "GET", "/api/v1/pieces/" + ids["piece"], nil, 403},
		{"pcx_alice_day", "GET", "/api/v1/day/" + day, nil, 403},
		{"pcx_alice_day", "GET", "/api/v1/todos", nil, 403},
		{"pcx_alice_capture", "POST", "/api/v1/pieces", map[string]any{"name": "x"}, 403},
		{"pcx_alice_capture", "PUT", "/api/v1/pieces/" + ids["piece"], map[string]any{"name": "x"}, 403},
		{"pcx_alice_capture", "DELETE", "/api/v1/pieces/" + ids["piece"], nil, 403},
		{"pcx_alice_capture", "POST", "/api/v1/plans/" + ids["plan"] + "/todos", map[string]any{"title": "x"}, 403},
		{"pcx_alice_capture", "GET", "/api/v1/settings", nil, 403},
		// the app's own scopes do not open the new routes
		{"pcx_alice_read", "GET", "/api/v1/pocket/search?q=sofa", nil, 403},
		{"pcx_alice_read", "GET", "/api/v1/pocket/day/" + day, nil, 403},
		{"pcx_alice_read", "GET", "/api/v1/pocket/widget", nil, 403},
		{"pcx_alice_write", "POST", "/api/v1/pocket/capture", line("req-0000000000000004"), 403},
		{"pcx_alice_read", "GET", "/api/v1/search?q=sofa", nil, 200},
		{"pcx_alice_read", "GET", "/api/v1/day/" + day, nil, 200},
		{"pcx_nobody", "GET", "/api/v1/pocket/widget", nil, 401},
	} {
		if code, m, _ := a.call(c.tok, c.method, c.path, c.body); code != c.want {
			t.Errorf("%s %s %s → %d, want %d (%v)", c.tok, c.method, c.path, code, c.want, m)
		}
	}
	// the token acts for its person only
	if got := items(t, a.ok("pcx_alice_search", "GET", "/api/v1/pocket/search?q=sofa", nil)); len(got) != 1 || got[0]["title"] != "Sofa" {
		t.Fatalf("alice through the link: %v", got)
	}
	if got := items(t, a.ok("pcx_bob_search", "GET", "/api/v1/pocket/search?q=sofa", nil)); len(got) != 0 {
		t.Fatalf("bob through the link finds alice's sofa: %v", got)
	}
	if w := a.ok("pcx_alice_widget", "GET", "/api/v1/pocket/widget", nil); w["line"] != "1 thing to do for Our flat" {
		t.Fatalf("alice's widget through the link: %v", w)
	}
	if w := a.ok("pcx_bob_widget", "GET", "/api/v1/pocket/widget", nil); len(items(t, w)) != 0 {
		t.Fatalf("bob's widget through the link: %v", w)
	}
	if d := a.ok("pcx_alice_day", "GET", "/api/v1/pocket/day/"+day, nil); len(items(t, d)) != 1 {
		t.Fatalf("alice's day through the link: %v", d)
	}
	if d := a.ok("pcx_bob_day", "GET", "/api/v1/pocket/day/"+day, nil); len(items(t, d)) != 0 {
		t.Fatalf("bob's day through the link: %v", d)
	}
	// only the one allowed capture landed, as the app that sent it; a retry adds nothing
	if code, m := capture(a, "pcx_alice_capture", "req-0000000000000001", "Rug https://shop.example/rug", nil); code != 201 {
		t.Fatalf("retry through the link: %d %v", code, m)
	}
	var rugs []map[string]any
	for _, v := range a.list(a.alice, "/api/v1/pieces") {
		if p := v.(map[string]any); p["name"] != "Sofa" {
			rugs = append(rugs, p)
		}
	}
	if len(rugs) != 1 || rugs[0]["name"] != "Rug" || rugs[0]["source"] != "hub" || rugs[0]["status"] != "want" || rugs[0]["url"] != "https://shop.example/rug" {
		t.Fatalf("alice's pieces: %v", rugs)
	}
	if l := a.list(a.bob, "/api/v1/pieces"); len(l) != 0 {
		t.Fatalf("bob got %d pieces", len(l))
	}
}

func TestPocketLinkScopes(t *testing.T) {
	req := func(m, p string) *http.Request { return httptest.NewRequest(m, p, nil) }
	for _, c := range []struct {
		scope string
		r     *http.Request
		want  bool
	}{
		{core.ScopeSearch, req("GET", "/api/v1/pocket/search?q=x"), true},
		{core.ScopeDay, req("GET", "/api/v1/pocket/day/2026-01-02"), true},
		{core.ScopeWidget, req("GET", "/api/v1/pocket/widget"), true},
		{core.ScopeCapture, req("POST", "/api/v1/pocket/capture"), true},
		{core.ScopeWidget, req("GET", "/api/v1/auth/me"), true},
		{core.ScopeSearch, req("POST", "/api/v1/pocket/search"), false},
		{core.ScopeSearch, req("GET", "/api/v1/search"), false},
		{core.ScopeDay, req("GET", "/api/v1/pocket/day/today"), false},
		{core.ScopeDay, req("GET", "/api/v1/day/2026-01-02"), false},
		{core.ScopeWidget, req("GET", "/api/v1/home"), false},
		{core.ScopeCapture, req("GET", "/api/v1/pocket/capture"), false},
		{core.ScopeCapture, req("POST", "/api/v1/pieces"), false},
		{"rooms:read", req("GET", "/api/v1/pocket/widget"), false},
		{"rooms:read", req("GET", "/api/v1/pocket/search"), false},
		{"rooms:write", req("POST", "/api/v1/pocket/capture"), false},
	} {
		if got := linkAllows([]string{c.scope}, c.r); got != c.want {
			t.Errorf("%s %s %s = %v, want %v", c.scope, c.r.Method, c.r.URL.Path, got, c.want)
		}
	}
}

// Rooms tells Core what it can do, so Search, Day and Hub find it on their own.
func TestManifestCapabilities(t *testing.T) {
	a, fc := newCapApp(t)
	if err := a.s.registerWithCore(context.Background()); err != nil {
		t.Fatal(err)
	}
	fc.mu.Lock()
	m := fc.manifest
	fc.mu.Unlock()
	if m["slug"] != "rooms" || m["category"] != "Home" {
		t.Fatalf("manifest: %v %v", m["slug"], m["category"])
	}
	search, _ := m["search"].(map[string]any)
	day, _ := m["day"].(map[string]any)
	widget, _ := m["widget"].(map[string]any)
	cap, _ := m["capture"].(map[string]any)
	if search == nil || len(search["kinds"].([]any)) != 3 || day == nil || day["label"] != "Rooms" || day["one"] != "checklist item" || widget == nil ||
		widget["title"] != "Home plans" || cap == nil {
		t.Fatalf("capabilities: %v %v %v %v", search, day, widget, cap)
	}
	if b := search["bangs"].([]any); len(b) != 2 || b[0] != "rooms" || b[1] != "plan" {
		t.Fatalf("bangs: %v", b)
	}
	if k := cap["kinds"].([]any)[0].(map[string]any); k["kind"] != "piece" || len(k["fields"].([]any)) != 1 || k["fields"].([]any)[0] != "title" {
		t.Fatalf("capture kinds: %v", cap)
	}
	offered := map[string]bool{}
	for _, o := range m["offers"].([]any) {
		offered[o.(map[string]any)["scope"].(string)] = true
	}
	for _, sc := range []string{"rooms:read", "rooms:write", core.ScopeSearch, core.ScopeDay, core.ScopeWidget, core.ScopeCapture} {
		if !offered[sc] {
			t.Errorf("%s is not offered", sc)
		}
	}
}
