package server

import (
	"context"
	"fmt"
	"math"
	"net/http"
	"sort"
	"strings"

	"github.com/abjelosevic88/pocket-homelab-rooms/internal/store"
)

// ---- what a layout adds up to: areas, paint, floor, the shopping list ----

type roomSum struct {
	ID        string   `json:"id"`
	Name      string   `json:"name"`
	Kind      string   `json:"kind"`
	Floor     string   `json:"floor"`
	AreaCm2   float64  `json:"area_cm2"`
	Perimeter float64  `json:"perimeter_cm"`
	WallCm2   float64  `json:"wall_cm2"`     // to paint: walls minus doors and windows
	Skirting  float64  `json:"skirting_cm"`  // the walls' length minus door openings
	Window    float64  `json:"window_cm2"`   // glass, roughly (daylight)
	Daylight  float64  `json:"daylight_pct"` // window area as a share of the floor area
	Size      string   `json:"size"`         // "4.2 × 3.5 m" for rectangles
	Items     []string `json:"items"`
	Doors     int      `json:"doors"`
	Windows   int      `json:"windows"`
}

type finishSum struct {
	Floor    string  `json:"floor"`
	AreaCm2  float64 `json:"area_cm2"`
	Order    float64 `json:"order_cm2"` // with 10 % for cuts and waste
	RoomsN   int     `json:"rooms"`
	RoomList string  `json:"room_list"`
}

type itemSum struct {
	ID      string `json:"id"`
	Name    string `json:"name"`
	Type    string `json:"type"`
	Room    string `json:"room"`
	Size    string `json:"size"`
	Price   int64  `json:"price"`
	Buy     bool   `json:"buy"`
	PieceID int64  `json:"piece_id"`
}

type layoutSum struct {
	LayoutID  int64       `json:"layout_id"`
	Layout    string      `json:"layout"`
	FloorID   int64       `json:"floor_id"`
	FloorName string      `json:"floor_name"`
	Ceiling   float64     `json:"ceiling_cm"`
	AreaCm2   float64     `json:"area_cm2"`
	OutCm2    float64     `json:"outside_cm2"` // balconies, terraces, garden
	WallCm2   float64     `json:"wall_cm2"`
	Skirting  float64     `json:"skirting_cm"`
	Rooms     []roomSum   `json:"rooms"`
	Finishes  []finishSum `json:"finishes"`
	Items     []itemSum   `json:"items"`
	Budget    int64       `json:"budget"`
	Owned     int         `json:"owned"`
	ToBuy     int         `json:"to_buy"`
}

func outside(kind string) bool { return kind == "balcony" || kind == "garden" }

func isRect(pts []store.Pt) (float64, float64, bool) {
	if len(pts) != 4 {
		return 0, 0, false
	}
	for i := 0; i < 4; i++ {
		a, b, c := pts[i], pts[(i+1)%4], pts[(i+2)%4]
		if math.Abs((b.X-a.X)*(c.X-b.X)+(b.Y-a.Y)*(c.Y-b.Y)) > 1 {
			return 0, 0, false
		}
	}
	return math.Hypot(pts[1].X-pts[0].X, pts[1].Y-pts[0].Y), math.Hypot(pts[2].X-pts[1].X, pts[2].Y-pts[1].Y), true
}

// summarize works a layout out.
func summarize(d store.Drawing, ceiling float64, pieces map[int64]store.Piece, units string) layoutSum {
	if ceiling <= 0 {
		ceiling = 250
	}
	out := layoutSum{Rooms: []roomSum{}, Finishes: []finishSum{}, Items: []itemSum{}, Ceiling: ceiling}
	fin := map[string]*finishSum{}
	for _, r := range d.Rooms {
		rs := roomSum{ID: r.ID, Name: r.Name, Kind: r.Kind, Floor: r.Floor, AreaCm2: r.Area(), Perimeter: r.Perimeter(), Items: []string{}}
		if w, dd, ok := isRect(r.Points); ok {
			rs.Size = lenText(math.Max(w, dd), units) + " × " + lenText(math.Min(w, dd), units)
		}
		h := firstPos(r.Height, ceiling)
		open := map[int]bool{}
		for _, e := range r.Open {
			open[e] = true
		}
		wallLen := 0.0
		for i := range r.Points {
			if !open[i] {
				wallLen += r.EdgeLen(i)
			}
		}
		rs.WallCm2 = wallLen * h
		rs.Skirting = wallLen
		for _, o := range d.OpeningsOf(r.ID) {
			if open[o.Edge] {
				continue
			}
			switch o.Kind {
			case "window":
				rs.Windows++
				hh := firstPos(o.Height, 140)
				rs.WallCm2 -= o.Width * hh
				rs.Window += o.Width * hh * 0.8
			default:
				rs.Doors++
				rs.WallCm2 -= o.Width * firstPos(o.Height, 205)
				rs.Skirting -= o.Width
				if o.Kind == "sliding" || o.Kind == "garage" {
					rs.Window += o.Width * firstPos(o.Height, 205) * 0.8 * boolF(o.Kind == "sliding")
				}
			}
		}
		rs.WallCm2 = math.Max(0, rs.WallCm2)
		rs.Skirting = math.Max(0, rs.Skirting)
		if outside(r.Kind) { // a balcony's railing is not painted
			rs.WallCm2, rs.Skirting = 0, 0
		}
		if rs.AreaCm2 > 0 {
			rs.Daylight = math.Round(rs.Window/rs.AreaCm2*1000) / 10
		}
		if outside(r.Kind) {
			out.OutCm2 += rs.AreaCm2
		} else {
			out.AreaCm2 += rs.AreaCm2
			out.WallCm2 += rs.WallCm2
			out.Skirting += rs.Skirting
		}
		if r.Floor != "" {
			f := fin[r.Floor]
			if f == nil {
				f = &finishSum{Floor: r.Floor}
				fin[r.Floor] = f
			}
			f.AreaCm2 += rs.AreaCm2
			f.RoomsN++
			f.RoomList = joinNonEmpty(", ", f.RoomList, r.Name)
		}
		out.Rooms = append(out.Rooms, rs)
	}
	for _, f := range fin {
		f.Order = f.AreaCm2 * 1.1
		out.Finishes = append(out.Finishes, *f)
	}
	sort.Slice(out.Finishes, func(i, j int) bool { return out.Finishes[i].AreaCm2 > out.Finishes[j].AreaCm2 })
	for _, it := range d.Items {
		room := d.RoomOf(it)
		price, buy := it.Price, it.Buy
		if p, ok := pieces[it.PieceID]; ok && it.PieceID > 0 {
			if p.Price != nil && price == 0 {
				price = *p.Price
			}
			if p.Status == "want" || p.Status == "maybe" {
				buy = true
			}
		}
		out.Items = append(out.Items, itemSum{ID: it.ID, Name: firstNonEmpty(it.Name, it.Type), Type: it.Type, Room: room, Size: sizeText(it.W, it.D, it.H, units),
			Price: price, Buy: buy, PieceID: it.PieceID})
		for i := range out.Rooms {
			if out.Rooms[i].Name == room && room != "" {
				out.Rooms[i].Items = append(out.Rooms[i].Items, firstNonEmpty(it.Name, it.Type))
				break
			}
		}
		if buy {
			out.ToBuy++
			out.Budget += price
		} else {
			out.Owned++
		}
	}
	return out
}

func boolF(b bool) float64 {
	if b {
		return 1
	}
	return 0
}

func (s *Server) piecesByID(ctx context.Context, uid int64) map[int64]store.Piece {
	out := map[int64]store.Piece{}
	ps, _ := s.store.ListPieces(ctx, uid, store.PieceFilter{Status: ""})
	for _, p := range ps {
		out[p.ID] = p
	}
	gone, _ := s.store.ListPieces(ctx, uid, store.PieceFilter{Status: "gone"})
	for _, p := range gone {
		out[p.ID] = p
	}
	return out
}

// planSummary sums the current layouts of every floor (or the given layouts).
func (s *Server) planSummary(ctx context.Context, uid, planID int64, pick map[int64]int64) ([]layoutSum, error) {
	floors, err := s.store.ListFloors(ctx, uid, planID)
	if err != nil {
		return nil, err
	}
	layouts, err := s.store.ListLayouts(ctx, uid, planID, true)
	if err != nil {
		return nil, err
	}
	st := s.settingsOf(ctx, uid)
	pieces := s.piecesByID(ctx, uid)
	out := []layoutSum{}
	for _, f := range floors {
		l := currentOf(f, layouts)
		if id, ok := pick[f.ID]; ok {
			for i := range layouts {
				if layouts[i].ID == id {
					l = &layouts[i]
				}
			}
		}
		if l == nil {
			continue
		}
		d, _ := store.ParseDrawing(l.Data)
		sum := summarize(d, f.CeilingCm, pieces, st.Units)
		sum.LayoutID, sum.Layout, sum.FloorID, sum.FloorName = l.ID, l.Name, f.ID, f.Name
		out = append(out, sum)
	}
	return out, nil
}

func (s *Server) handlePlanSummary(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	uid := actorID(ctx)
	p, err := s.store.GetPlan(ctx, uid, idParam(r, "id"))
	if err != nil {
		s.fail(w, err, "plan")
		return
	}
	pick := map[int64]int64{}
	if lid := int64(queryInt(r, "layout")); lid > 0 {
		if l, err := s.store.GetLayout(ctx, uid, lid); err == nil && l.PlanID == p.ID {
			pick[l.FloorID] = l.ID
		}
	}
	sums, err := s.planSummary(ctx, uid, p.ID, pick)
	if err != nil {
		s.fail(w, err, "summary")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"plan_id": p.ID, "currency": p.Currency, "floors": sums})
}

// planText is a plan written out (Notes, the AI, Chat).
func (s *Server) planText(ctx context.Context, uid int64, p *store.Plan, sums []layoutSum) string {
	st := s.settingsOf(ctx, uid)
	var b strings.Builder
	fmt.Fprintf(&b, "# %s\n\n", p.Name)
	if p.Address != "" {
		fmt.Fprintf(&b, "%s\n\n", p.Address)
	}
	total, out := 0.0, 0.0
	for _, f := range sums {
		total += f.AreaCm2
		out += f.OutCm2
	}
	fmt.Fprintf(&b, "Floor area: **%s**", areaText(total, st.Units))
	if out > 0 {
		fmt.Fprintf(&b, " (and %s outside)", areaText(out, st.Units))
	}
	b.WriteString("\n")
	for _, f := range sums {
		fmt.Fprintf(&b, "\n## %s — %s\n\n", f.FloorName, f.Layout)
		b.WriteString("| Room | Size | Area | Walls to paint |\n|---|---|---|---|\n")
		for _, r := range f.Rooms {
			fmt.Fprintf(&b, "| %s | %s | %s | %s |\n", r.Name, firstNonEmpty(r.Size, "—"), areaText(r.AreaCm2, st.Units), areaText(r.WallCm2, st.Units))
		}
		if len(f.Finishes) > 0 {
			b.WriteString("\nFloors to order (with 10 % for cuts): ")
			parts := []string{}
			for _, fn := range f.Finishes {
				parts = append(parts, fmt.Sprintf("%s %s", fn.Floor, areaText(fn.Order, st.Units)))
			}
			b.WriteString(strings.Join(parts, ", ") + ".\n")
		}
		fmt.Fprintf(&b, "Skirting board: %s. Ceiling height %s.\n", lenText(f.Skirting, st.Units), lenText(f.Ceiling, st.Units))
		buy := []string{}
		for _, it := range f.Items {
			if it.Buy {
				line := "- " + it.Name + " (" + it.Size + ")"
				if it.Room != "" {
					line += " — " + it.Room
				}
				if it.Price > 0 {
					line += " — " + money(it.Price, p.Currency)
				}
				buy = append(buy, line)
			}
		}
		if len(buy) > 0 {
			fmt.Fprintf(&b, "\n### To buy (%s)\n\n%s\n", money(f.Budget, p.Currency), strings.Join(buy, "\n"))
		}
	}
	if p.Notes != "" {
		b.WriteString("\n## Notes\n\n" + p.Notes + "\n")
	}
	fmt.Fprintf(&b, "\n[Open in Pocket Rooms](%s)\n", s.link(fmt.Sprintf("/plans/%d", p.ID)))
	return b.String()
}

// ---- the overview, search and the day (for Pocket AI, Search and Day) ----

func (s *Server) handleHome(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	uid := actorID(ctx)
	plans, err := s.store.ListPlans(ctx, uid, false)
	if err != nil {
		s.fail(w, err, "plans")
		return
	}
	for i := range plans {
		s.planTotals(ctx, uid, &plans[i], true)
	}
	todos, _ := s.store.ListTodos(ctx, uid, store.TodoFilter{Open: true})
	if len(todos) > 8 {
		todos = todos[:8]
	}
	want, _ := s.store.ListPieces(ctx, uid, store.PieceFilter{Status: "want", Limit: 6})
	counts := s.store.CountsFor(ctx, uid)
	writeJSON(w, http.StatusOK, map[string]any{"plans": plans, "todos": todos, "wanted": want, "pieces": counts["pieces"]})
}

func (s *Server) handleSummary(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	uid := actorID(ctx)
	st := s.settingsOf(ctx, uid)
	plans, err := s.store.ListPlans(ctx, uid, false)
	if err != nil {
		s.fail(w, err, "plans")
		return
	}
	list := []map[string]any{}
	for i := range plans {
		p := &plans[i]
		s.planTotals(ctx, uid, p, false)
		list = append(list, map[string]any{"id": p.ID, "name": p.Name, "kind": p.Kind, "status": p.Status, "area_m2": math.Round(p.AreaCm2/100) / 100, "area": areaText(p.AreaCm2, st.Units),
			"rooms": p.RoomsN, "pieces": p.ItemsN, "to_buy": money(p.Budget, p.Currency), "open_todos": p.OpenTodo, "home": p.HomeName, "url": s.link(fmt.Sprintf("/plans/%d", p.ID))})
	}
	want, _ := s.store.ListPieces(ctx, uid, store.PieceFilter{Status: "want"})
	todos, _ := s.store.ListTodos(ctx, uid, store.TodoFilter{Open: true})
	wl := []map[string]any{}
	for _, p := range want {
		wl = append(wl, map[string]any{"id": p.ID, "name": p.Name, "size": sizeText(p.Width, p.Depth, p.Height, st.Units)})
	}
	writeJSON(w, http.StatusOK, map[string]any{"plans": list, "wanted": wl, "open_todos": len(todos), "todos": todos})
}

func (s *Server) handleSearch(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	uid := actorID(ctx)
	q := strings.ToLower(strings.TrimSpace(r.URL.Query().Get("q")))
	limit := queryInt(r, "limit")
	if limit <= 0 || limit > 50 {
		limit = 20
	}
	out := []map[string]any{}
	if q == "" {
		writeJSON(w, http.StatusOK, map[string]any{"results": out})
		return
	}
	st := s.settingsOf(ctx, uid)
	plans, _ := s.store.ListPlans(ctx, uid, false)
	for _, p := range plans {
		if strings.Contains(strings.ToLower(p.Name+" "+p.Address+" "+p.Notes), q) {
			path := fmt.Sprintf("/plans/%d", p.ID)
			out = append(out, map[string]any{"kind": "plan", "id": p.ID, "title": p.Name, "name": p.Name, "subtitle": firstNonEmpty(p.Address, p.Kind), "line": firstNonEmpty(p.Address, areaText(p.AreaCm2, st.Units)), "url": s.link(path), "path": path})
		}
		layouts, _ := s.store.ListLayouts(ctx, uid, p.ID, true)
		for _, l := range layouts {
			d, _ := store.ParseDrawing(l.Data)
			for _, rm := range d.Rooms {
				if strings.Contains(strings.ToLower(rm.Name), q) {
					sub := fmt.Sprintf("%s · %s · %s", p.Name, l.Name, areaText(rm.Area(), st.Units))
					path := fmt.Sprintf("/plans/%d?layout=%d", p.ID, l.ID)
					out = append(out, map[string]any{"kind": "room", "id": rm.ID, "title": rm.Name, "name": rm.Name, "subtitle": sub, "line": sub, "url": s.link(path), "path": path})
				}
			}
		}
	}
	pieces, _ := s.store.ListPieces(ctx, uid, store.PieceFilter{Q: q, Limit: limit})
	for _, p := range pieces {
		sub := sizeText(p.Width, p.Depth, p.Height, st.Units)
		path := fmt.Sprintf("/furniture/%d", p.ID)
		out = append(out, map[string]any{"kind": "piece", "id": p.ID, "title": p.Name, "name": p.Name, "subtitle": sub, "line": sub, "url": s.link(path), "path": path})
	}
	if len(out) > limit {
		out = out[:limit]
	}
	writeJSON(w, http.StatusOK, map[string]any{"results": out})
}

// handleDay is what happened in Rooms on a day: checklist items due and done.
func (s *Server) handleDay(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	uid := actorID(ctx)
	date := chiParam(r, "date")
	if !validDate(date) {
		writeErr(w, http.StatusBadRequest, "date is YYYY-MM-DD")
		return
	}
	todos, _ := s.store.ListTodos(ctx, uid, store.TodoFilter{})
	items := []map[string]any{}
	for _, t := range todos {
		doneThatDay := t.Done && strings.HasPrefix(t.DoneAt, date)
		if t.Due == date || doneThatDay {
			items = append(items, map[string]any{"kind": "todo", "title": t.Title, "subtitle": t.PlanName, "done": t.Done, "due": t.Due,
				"url": s.link(fmt.Sprintf("/plans/%d?tab=checklist", t.PlanID))})
		}
	}
	writeJSON(w, http.StatusOK, map[string]any{"date": date, "items": items})
}
