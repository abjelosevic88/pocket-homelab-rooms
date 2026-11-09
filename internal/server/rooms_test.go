package server

import (
	"bytes"
	"encoding/json"
	"fmt"
	"math"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/abjelosevic88/pocket-homelab-rooms/internal/config"
	"github.com/abjelosevic88/pocket-homelab-rooms/internal/store"
)

// flat sets up a plan with a layout, a piece, a route and a checklist item for tok.
func flat(a *app, tok string) map[string]string {
	a.t.Helper()
	out := map[string]string{}
	doc := a.expect(201, tok, "POST", "/api/v1/plans", map[string]any{"name": "Our flat", "template": "one_bed", "notes": "Private notes"})
	plan := doc["plan"].(map[string]any)
	out["plan"] = id(plan)
	fl := plan["floors"].([]any)[0].(map[string]any)
	out["floor"] = id(fl)
	out["layout"] = id(fl["layouts"].([]any)[0].(map[string]any))
	p := a.expect(201, tok, "POST", "/api/v1/pieces", map[string]any{"name": "Sofa", "type": "sofa", "width": 220, "depth": 95, "height": 85, "price": 50000})
	out["piece"] = id(p)
	rt := a.expect(201, tok, "POST", "/api/v1/plans/"+out["plan"]+"/routes", map[string]any{"name": "Way in", "steps": []map[string]any{{"kind": "door", "width": 90, "height": 205}}})
	out["route"] = id(rt)
	td := a.expect(201, tok, "POST", "/api/v1/plans/"+out["plan"]+"/todos", map[string]any{"title": "Measure the hall", "due": dayAfter(1)})
	out["todo"] = id(td)
	return out
}

func dayAfter(n int) string { return store.AddDays(today(), n) }

func TestEverythingIsPerPerson(t *testing.T) {
	a := newApp(t, nil)
	ids := flat(a, a.alice)
	if l := a.list(a.bob, "/api/v1/plans"); len(l) != 0 {
		t.Fatalf("bob sees %d plans", len(l))
	}
	if l := a.list(a.bob, "/api/v1/pieces"); len(l) != 0 {
		t.Fatalf("bob sees %d pieces", len(l))
	}
	if l := a.list(a.bob, "/api/v1/todos"); len(l) != 0 {
		t.Fatalf("bob sees %d todos", len(l))
	}
	for _, c := range []struct{ method, path string }{
		{"GET", "/api/v1/plans/" + ids["plan"]}, {"PUT", "/api/v1/plans/" + ids["plan"]}, {"DELETE", "/api/v1/plans/" + ids["plan"]},
		{"POST", "/api/v1/plans/" + ids["plan"] + "/share"}, {"POST", "/api/v1/plans/" + ids["plan"] + "/duplicate"},
		{"GET", "/api/v1/plans/" + ids["plan"] + "/summary"}, {"POST", "/api/v1/plans/" + ids["plan"] + "/floors"},
		{"GET", "/api/v1/plans/" + ids["plan"] + "/routes"}, {"POST", "/api/v1/plans/" + ids["plan"] + "/routes"},
		{"POST", "/api/v1/plans/" + ids["plan"] + "/todos"}, {"GET", "/api/v1/plans/" + ids["plan"] + "/photos"},
		{"PUT", "/api/v1/floors/" + ids["floor"]}, {"DELETE", "/api/v1/floors/" + ids["floor"]}, {"POST", "/api/v1/floors/" + ids["floor"] + "/layouts"},
		{"GET", "/api/v1/layouts/" + ids["layout"]}, {"PUT", "/api/v1/layouts/" + ids["layout"]}, {"DELETE", "/api/v1/layouts/" + ids["layout"]},
		{"GET", "/api/v1/pieces/" + ids["piece"]}, {"PUT", "/api/v1/pieces/" + ids["piece"]}, {"DELETE", "/api/v1/pieces/" + ids["piece"]},
		{"PUT", "/api/v1/routes/" + ids["route"]}, {"DELETE", "/api/v1/routes/" + ids["route"]},
		{"PUT", "/api/v1/todos/" + ids["todo"]}, {"DELETE", "/api/v1/todos/" + ids["todo"]},
	} {
		a.expect(404, a.bob, c.method, c.path, map[string]any{"name": "Mine", "title": "x"})
	}
	a.expect(404, a.bob, "POST", "/api/v1/fit", map[string]any{"piece_id": n(ids["piece"]), "steps": []map[string]any{{"kind": "door", "width": 80}}})
	a.expect(404, a.bob, "POST", "/api/v1/fit", map[string]any{"width": 10, "depth": 10, "height": 10, "route_id": n(ids["route"])})
	a.expect(404, a.bob, "POST", "/api/v1/todos", map[string]any{"plan_id": n(ids["plan"]), "title": "spy"})
	a.expect(404, a.bob, "GET", "/api/v1/fit?piece_id="+ids["piece"]+"&door_width=80", nil)
	if m := a.ok(a.alice, "GET", "/api/v1/fit?piece_id="+ids["piece"]+"&door_width=80&door_height=200", nil); m["result"].(map[string]any)["ok"] != false {
		t.Fatalf("a 95 × 85 sofa end through an 80 cm door: %v", m)
	}
	if d := a.ok(a.alice, "GET", "/api/v1/plans/"+ids["plan"], nil); d["plan"].(map[string]any)["name"] != "Our flat" {
		t.Fatalf("alice's plan: %v", d)
	}
}

func TestLayoutSaveAndTotals(t *testing.T) {
	a := newApp(t, nil)
	ids := flat(a, a.alice)
	l := a.ok(a.alice, "GET", "/api/v1/layouts/"+ids["layout"], nil)
	rev := num(l, "revision")
	d := map[string]any{
		"rooms": []map[string]any{{"id": "r1", "name": "Room", "points": []map[string]any{{"x": 0, "y": 0}, {"x": 400, "y": 0}, {"x": 400, "y": 300}, {"x": 0, "y": 300}}}},
		"items": []map[string]any{{"id": "i1", "name": "Sofa", "type": "sofa", "x": 200, "y": 150, "w": 220, "d": 95, "buy": true, "price": 50000}},
	}
	saved := a.ok(a.alice, "PUT", "/api/v1/layouts/"+ids["layout"], map[string]any{"data": d, "revision": rev})
	if num(saved, "area_cm2") != 120000 || num(saved, "budget") != 50000 || num(saved, "items_n") != 1 {
		t.Fatalf("totals: %v", saved)
	}
	// a stale save is refused
	a.expect(409, a.alice, "PUT", "/api/v1/layouts/"+ids["layout"], map[string]any{"data": d, "revision": rev})
	// nonsense is refused
	a.expect(400, a.alice, "PUT", "/api/v1/layouts/"+ids["layout"], map[string]any{"data": map[string]any{"items": []map[string]any{{"id": "x", "w": 0, "d": 0}}}})
	sum := a.ok(a.alice, "GET", "/api/v1/plans/"+ids["plan"]+"/summary", nil)
	f := sum["floors"].([]any)[0].(map[string]any)
	room := f["rooms"].([]any)[0].(map[string]any)
	// walls to paint: 14 m × 2.5 m = 35 m²
	if num(room, "wall_cm2") != 1400*250 || num(f, "to_buy") != 1 {
		t.Fatalf("summary: %v", f)
	}
	// an option copied from it, then made the current one
	opt := a.expect(201, a.alice, "POST", "/api/v1/floors/"+ids["floor"]+"/layouts", map[string]any{"copy_from": n(ids["layout"]), "shell": true})
	if num(opt, "items_n") != 0 || num(opt, "rooms_n") != 1 {
		t.Fatalf("option: %v", opt)
	}
	a.ok(a.alice, "PUT", "/api/v1/floors/"+ids["floor"], map[string]any{"current_layout_id": num(opt, "id")})
	plans := a.list(a.alice, "/api/v1/plans")
	if num(plans[0].(map[string]any), "items_n") != 0 {
		t.Fatalf("current layout not used: %v", plans[0])
	}
	a.expect(400, a.alice, "DELETE", "/api/v1/floors/"+ids["floor"], nil) // the last floor stays
}

func TestShareLinkHidesPrivateBits(t *testing.T) {
	a := newApp(t, nil)
	ids := flat(a, a.alice)
	doc := a.ok(a.alice, "POST", "/api/v1/plans/"+ids["plan"]+"/share", map[string]any{"on": true})
	tok := doc["plan"].(map[string]any)["share_token"].(string)
	code, m, _ := a.call("", "GET", "/api/v1/public/s/"+tok, nil)
	if code != 200 || m["name"] != "Our flat" {
		t.Fatalf("public: %d %v", code, m)
	}
	raw, _ := json.Marshal(m)
	if bytes.Contains(raw, []byte("Private notes")) {
		t.Fatal("the share link shows the plan's notes")
	}
	a.ok(a.alice, "POST", "/api/v1/plans/"+ids["plan"]+"/share", map[string]any{"on": false})
	a.expect(404, "", "GET", "/api/v1/public/s/"+tok, nil)
}

func TestWillItFit(t *testing.T) {
	door := store.RouteStep{Kind: "door", Width: 80, Height: 200}
	// a 220 × 95 × 85 sofa goes through an 80 cm door on its back? no: 85 > 80; but standing on its end, 95 × 85 doesn't fit either → it must go through 85 high, 95 wide: no.
	if r := checkFit(220, 95, 85, []store.RouteStep{door}, 0, "metric"); r.OK {
		t.Fatalf("a 85 × 95 cross-section passed an 80 cm door: %+v", r)
	}
	// with legs off (75 cm deep) it passes upright on its side
	if r := checkFit(220, 75, 85, []store.RouteStep{door}, 0, "metric"); !r.OK {
		t.Fatalf("a 75 cm deep sofa did not pass an 80 cm door: %+v", r)
	}
	// round a corner between two 100 cm hallways: the ladder bound for a 50 cm wide piece
	mx := cornerMax(100, 100, 50)
	want := 2*math.Sqrt2*100 - 2*50 // at 45°
	if math.Abs(mx-want) > 1 {
		t.Fatalf("corner max %v, want %v", mx, want)
	}
	corner := store.RouteStep{Kind: "corner", Width: 100, Turn: 100}
	if r := checkFit(300, 50, 60, []store.RouteStep{corner}, 0, "metric"); r.OK {
		t.Fatalf("a 3 m piece turned a 1 m corner: %+v", r)
	}
	if r := checkFit(150, 50, 60, []store.RouteStep{corner}, 0, "metric"); !r.OK {
		t.Fatalf("a 1.5 m piece did not turn a 1 m corner: %+v", r)
	}
	// round a corner, tilted up: a 200 × 40 × 40 board does not swing flat round a 70 / 70 corner but goes tilted
	if r := checkFit(200, 40, 40, []store.RouteStep{{Kind: "corner", Width: 70, Turn: 70, Height: 250}}, 0, "metric"); !r.OK {
		t.Fatalf("a board did not tilt round a corner: %+v", r)
	}
	// a lift: a 230 cm piece stands up in a 210 cm lift? no; lies across the diagonal of 100 × 130 (164)? no
	lift := store.RouteStep{Kind: "lift", Width: 100, Depth: 130, Height: 210}
	if r := checkFit(230, 40, 40, []store.RouteStep{lift}, 0, "metric"); r.OK {
		t.Fatalf("a 230 cm piece fit in the lift: %+v", r)
	}
	if r := checkFit(200, 40, 40, []store.RouteStep{lift}, 0, "metric"); !r.OK {
		t.Fatalf("a 200 cm piece did not stand up in the lift: %+v", r)
	}
	if !rectFits(140, 10, 100, 130) { // across the floor's diagonal (164 cm)
		t.Fatal("a 140 × 10 board does not fit across a 100 × 130 floor")
	}
	if rectFits(170, 20, 100, 130) {
		t.Fatal("a 170 cm board fits across a 164 cm diagonal")
	}
}

func TestExportImportTwice(t *testing.T) {
	a := newApp(t, nil)
	flat(a, a.alice)
	req, _ := http.NewRequest("GET", a.url+"/api/v1/export.json", nil)
	req.Header.Set("Authorization", "Bearer "+a.alice)
	res, err := http.DefaultClient.Do(req)
	if err != nil || res.StatusCode != 200 {
		t.Fatalf("export: %v", err)
	}
	var buf bytes.Buffer
	_, _ = buf.ReadFrom(res.Body)
	res.Body.Close()
	for i := 0; i < 2; i++ { // into Bob's account, twice
		req, _ = http.NewRequest("POST", a.url+"/api/v1/import", bytes.NewReader(buf.Bytes()))
		req.Header.Set("Authorization", "Bearer "+a.bob)
		req.Header.Set("Content-Type", "application/json")
		res, err = http.DefaultClient.Do(req)
		if err != nil || res.StatusCode != 200 {
			t.Fatalf("import %d: %v %v", i, err, res.StatusCode)
		}
		res.Body.Close()
	}
	plans := a.list(a.bob, "/api/v1/plans")
	if len(plans) != 1 || len(a.list(a.bob, "/api/v1/pieces")) != 1 || len(a.list(a.bob, "/api/v1/todos")) != 1 {
		t.Fatalf("after two imports bob has %d plans", len(plans))
	}
	pid := id(plans[0].(map[string]any))
	doc := a.ok(a.bob, "GET", "/api/v1/plans/"+pid, nil)
	fl := doc["plan"].(map[string]any)["floors"].([]any)[0].(map[string]any)
	if len(fl["layouts"].([]any)) != 1 || fl["current_layout_id"] == nil || len(doc["routes"].([]any)) != 1 {
		t.Fatalf("imported plan: %v", doc)
	}
}

func TestSweetHome3D(t *testing.T) {
	a := newApp(t, nil)
	xml := `<?xml version="1.0"?><home name="Test">
<wall id="w1" xStart="0" yStart="0" xEnd="400" yEnd="0" thickness="10"/>
<room name="Kitchen"><point x="0" y="0"/><point x="400" y="0"/><point x="400" y="300"/><point x="0" y="300"/></room>
<doorOrWindow name="Door" x="200" y="300" width="80" depth="10" height="210" angle="0"/>
<pieceOfFurniture name="Fridge" x="50" y="40" width="60" depth="65" height="185" angle="1.5707963"/>
</home>`
	req, _ := http.NewRequest("POST", a.url+"/api/v1/import?name=Test.sh3d", bytes.NewReader([]byte(xml)))
	req.Header.Set("Authorization", "Bearer "+a.alice)
	res, err := http.DefaultClient.Do(req)
	if err != nil || res.StatusCode != 200 {
		t.Fatalf("import: %v %v", err, res.StatusCode)
	}
	var out map[string]any
	_ = json.NewDecoder(res.Body).Decode(&out)
	res.Body.Close()
	doc := a.ok(a.alice, "GET", fmt.Sprintf("/api/v1/plans/%d", int64(num(out, "plan_id"))), nil)
	fl := doc["plan"].(map[string]any)["floors"].([]any)[0].(map[string]any)
	l := a.ok(a.alice, "GET", "/api/v1/layouts/"+id(fl["layouts"].([]any)[0].(map[string]any)), nil)
	d := l["data"].(map[string]any)
	items := d["items"].([]any)
	if len(d["rooms"].([]any)) != 1 || len(items) != 1 || len(d["openings"].([]any)) != 1 || items[0].(map[string]any)["type"] != "fridge" || math.Round(num(items[0].(map[string]any), "rot")) != 90 {
		t.Fatalf("drawing: %v", d)
	}
}

func TestSettingsArePersonal(t *testing.T) {
	a := newApp(t, nil)
	a.ok(a.alice, "PUT", "/api/v1/settings", map[string]any{"units": "imperial", "notes_folder": "Home/Plans", "snap": 10})
	if s := a.ok(a.bob, "GET", "/api/v1/settings", nil); s["units"] != "metric" || s["notes_folder"] != "Rooms" {
		t.Fatalf("bob's settings changed: %v", s)
	}
	if s := a.ok(a.alice, "GET", "/api/v1/settings", nil); s["units"] != "imperial" || num(s, "snap") != 10 {
		t.Fatalf("alice's settings: %v", s)
	}
	a.expect(400, a.alice, "PUT", "/api/v1/settings", map[string]any{"units": "cubits"})
	a.expect(400, a.alice, "PUT", "/api/v1/settings", map[string]any{"ceiling_cm": 20})
}

func TestLinkScopes(t *testing.T) {
	r := func(method, path string) *http.Request { return httptest.NewRequest(method, path, nil) }
	read, write := []string{"rooms:read"}, []string{"rooms:write"}
	for _, c := range []struct {
		scopes []string
		req    *http.Request
		want   bool
	}{
		{read, r("GET", "/api/v1/summary"), true},
		{read, r("GET", "/api/v1/plans/3"), true},
		{read, r("GET", "/api/v1/plans/3/summary"), true},
		{read, r("GET", "/api/v1/layouts/3"), true},
		{read, r("POST", "/api/v1/fit"), true},
		{read, r("GET", "/api/v1/fit"), true},
		{read, r("GET", "/api/v1/day/2026-11-13"), true},
		{read, r("POST", "/api/v1/pieces"), false},
		{read, r("GET", "/api/v1/settings"), false},
		{write, r("POST", "/api/v1/pieces"), true},
		{write, r("PUT", "/api/v1/pieces/4"), true},
		{write, r("POST", "/api/v1/plans/4/todos"), true},
		{write, r("PUT", "/api/v1/layouts/4"), false},
		{write, r("DELETE", "/api/v1/plans/4"), false},
		{write, r("POST", "/api/v1/plans/4/share"), false},
		{nil, r("GET", "/api/v1/auth/me"), true},
	} {
		if got := linkAllows(c.scopes, c.req); got != c.want {
			t.Errorf("%v %s %s = %v, want %v", c.scopes, c.req.Method, c.req.URL.Path, got, c.want)
		}
	}
}

func TestDemo(t *testing.T) {
	a := newApp(t, func(c *config.Config) { c.DemoData = true })
	plans := a.list(a.alice, "/api/v1/plans")
	if len(plans) != 3 {
		t.Fatalf("%d plans", len(plans))
	}
	if err := a.s.seedDemo(t.Context(), a.ids["Alice"]); err != nil {
		t.Fatal(err)
	}
	if l := a.list(a.alice, "/api/v1/plans"); len(l) != 3 {
		t.Fatalf("%d plans after a second seed", len(l))
	}
	our := plans[0].(map[string]any)
	if our["name"] != "Our flat" || num(our, "area_cm2") < 600000 {
		t.Fatalf("our flat: %v", our)
	}
	// the corner sofa does not make it up the stairs
	pieces := a.list(a.alice, "/api/v1/pieces")
	for _, p := range pieces {
		pm := p.(map[string]any)
		if pm["name"] != "Corner sofa Lund" {
			continue
		}
		d := a.ok(a.alice, "GET", "/api/v1/pieces/"+id(pm), nil)
		fits := d["fits"].([]any)
		if len(fits) < 2 || fits[0].(map[string]any)["result"].(map[string]any)["ok"] == true {
			t.Fatalf("corner sofa fits: %v", fits)
		}
	}
}
