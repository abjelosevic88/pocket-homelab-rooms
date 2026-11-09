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

// ---- AI help: read a floor plan picture, read sizes, tips for a layout ----

func (s *Server) aiGate(w http.ResponseWriter, r *http.Request) (int64, store.Settings, bool) {
	ctx := r.Context()
	uid := actorID(ctx)
	st := s.settingsOf(ctx, uid)
	if !st.AIHelp {
		writeErr(w, http.StatusBadRequest, "AI help is switched off (Settings → AI)")
		return 0, st, false
	}
	select {
	case s.aiSem <- struct{}{}:
	default:
		writeErr(w, http.StatusTooManyRequests, "the AI is busy with other questions; try again in a moment")
		return 0, st, false
	}
	return uid, st, true
}

func (s *Server) aiDone() { <-s.aiSem }

const planReaderSystem = `You read floor plan pictures (an estate agent's plan, a hand sketch, a photo of a printed plan) and turn them into rooms for a floor planner.
Answer with JSON only: {"scale_note": "...", "rooms": [{"name": "Living room", "kind": "living", "x": 0, "y": 0, "width": 450, "depth": 380}], "doors": [{"room": 0, "edge": "bottom", "at": 0.5, "width": 80}], "windows": [{"room": 0, "edge": "top", "at": 0.5, "width": 120}]}
Rules:
- Every length is in centimetres. Use the dimensions written on the plan when there are any (e.g. "4.50 × 3.80 m" → width 450, depth 380); otherwise estimate from a door being about 80 cm wide.
- x, y is the room's top-left corner, measured from the top-left of the whole plan, x to the right, y down. Rooms are rectangles; for an L-shaped room give its two rectangles with the same name.
- Leave about 12 cm (a wall) between rooms that touch.
- kind is one of: living, bedroom, kitchen, dining, bath, toilet, hall, office, kids, storage, laundry, balcony, garage, garden, stairs, other.
- Doors and windows: room is the index of the room in your list, edge is top | right | bottom | left, at is 0..1 along that edge from its left/top end.
- Name rooms in the language written on the plan. If the picture is not a floor plan, answer {"rooms": [], "scale_note": "This does not look like a floor plan."}.`

// handleAIReadPlan turns a picture of a floor plan into a draft drawing.
func (s *Server) handleAIReadPlan(w http.ResponseWriter, r *http.Request) {
	uid, st, ok := s.aiGate(w, r)
	if !ok {
		return
	}
	defer s.aiDone()
	ctx := r.Context()
	var in struct {
		PhotoID int64  `json:"photo_id"`
		Hint    string `json:"hint"`
	}
	if err := decode(r, &in); err != nil || in.PhotoID == 0 {
		writeErr(w, http.StatusBadRequest, "pick a picture of the floor plan")
		return
	}
	imgs, err := s.photoForAI(ctx, uid, in.PhotoID)
	if err != nil {
		s.fail(w, err, "photo")
		return
	}
	var out struct {
		ScaleNote string `json:"scale_note"`
		Rooms     []struct {
			Name  string  `json:"name"`
			Kind  string  `json:"kind"`
			X     float64 `json:"x"`
			Y     float64 `json:"y"`
			Width float64 `json:"width"`
			Depth float64 `json:"depth"`
		} `json:"rooms"`
		Doors []struct {
			Room  int     `json:"room"`
			Edge  string  `json:"edge"`
			At    float64 `json:"at"`
			Width float64 `json:"width"`
		} `json:"doors"`
		Windows []struct {
			Room  int     `json:"room"`
			Edge  string  `json:"edge"`
			At    float64 `json:"at"`
			Width float64 `json:"width"`
		} `json:"windows"`
	}
	prompt := "Read this floor plan."
	if h := strings.TrimSpace(in.Hint); h != "" {
		prompt += " The person adds: " + clip(h, 500)
	}
	if err := s.aiAsk(ctx, uid, st, planReaderSystem, prompt, imgs, &out); err != nil {
		s.userFail(w, err, "ai")
		return
	}
	b := newDrawer()
	ids := []string{}
	for _, rm := range out.Rooms {
		if rm.Width < 50 || rm.Depth < 50 || rm.Width > 5000 || rm.Depth > 5000 {
			ids = append(ids, "")
			continue
		}
		kind := rm.Kind
		if !oneOf(kind, store.RoomKinds) {
			kind = guessRoomKind(rm.Name)
		}
		floor := "wood"
		switch kind {
		case "bath", "toilet", "kitchen", "hall", "laundry":
			floor = "tile"
		case "balcony":
			floor = "deck"
		case "garden":
			floor = "grass"
		case "garage", "storage":
			floor = "concrete"
		}
		ids = append(ids, b.room(clip(firstNonEmpty(rm.Name, "Room"), maxName), kind, math.Round(rm.X), math.Round(rm.Y), math.Round(rm.Width), math.Round(rm.Depth), floor))
	}
	edgeOf := map[string]int{"top": 0, "right": 1, "bottom": 2, "left": 3}
	addOpening := func(room int, edge string, at, width float64, window bool) {
		if room < 0 || room >= len(ids) || ids[room] == "" {
			return
		}
		e, ok := edgeOf[edge]
		if !ok {
			return
		}
		var rr store.DRoom
		for _, x := range b.d.Rooms {
			if x.ID == ids[room] {
				rr = x
			}
		}
		l := rr.EdgeLen(e)
		width = clampF(firstPos(width, 80), 40, l)
		pos := clampF(at, 0, 1) * l
		// edges 2 and 3 run right-to-left / bottom-to-top
		if e >= 2 {
			pos = l - pos
		}
		pos = clampF(pos, width/2, l-width/2)
		if window {
			b.window(ids[room], e, math.Round(pos), math.Round(width))
		} else {
			b.door(ids[room], e, math.Round(pos), math.Round(width), "start", false)
		}
	}
	for _, d := range out.Doors {
		addOpening(d.Room, d.Edge, d.At, d.Width, false)
	}
	for _, d := range out.Windows {
		addOpening(d.Room, d.Edge, d.At, d.Width, true)
	}
	writeJSON(w, http.StatusOK, map[string]any{"drawing": b.d, "note": out.ScaleNote, "rooms": len(b.d.Rooms)})
}

const sizeReaderSystem = `You read a shop's product page text (or what a person wrote) about a piece of furniture and give its size for a floor planner.
Answer with JSON only: {"name": "...", "type": "...", "category": "...", "width": 0, "depth": 0, "height": 0, "price": 0, "currency": "EUR", "shop": "...", "note": "..."}
- width is side to side, depth front to back, height floor to top, all in centimetres (convert inches: 1 in = 2.54 cm). Use the overall size, not the seat or the mattress.
- type is one of: sofa, sofa_l, armchair, coffee_table, tv_unit, bookcase, shelf, bed_double, bed_single, crib, nightstand, wardrobe, dresser, desk, office_chair, chair, dining_table, table_round, bench, counter, sink_unit, stove, fridge, dishwasher, washer, toilet, bathtub, shower, basin, plant, rug, lamp, piano, radiator, box.
- category is one of: living, sleeping, dining, kitchen, bath, work, storage, kids, outdoor, other.
- price in minor units (cents), 0 when not given. note: one short line when something is unsure (e.g. "depth with the cushions"). Use 0 for a size you cannot find.`

func (s *Server) handleAISizes(w http.ResponseWriter, r *http.Request) {
	uid, st, ok := s.aiGate(w, r)
	if !ok {
		return
	}
	defer s.aiDone()
	ctx := r.Context()
	var in struct {
		Text string `json:"text"`
	}
	if err := decode(r, &in); err != nil || strings.TrimSpace(in.Text) == "" {
		writeErr(w, http.StatusBadRequest, "paste the product's description or its size")
		return
	}
	var out map[string]any
	if err := s.aiAsk(ctx, uid, st, sizeReaderSystem, clip(in.Text, 12000), nil, &out); err != nil {
		s.userFail(w, err, "ai")
		return
	}
	writeJSON(w, http.StatusOK, out)
}

const tipsSystem = `You are a calm, practical interior planner. A person shows you one floor plan layout (rooms with sizes, doors, windows, furniture with positions) and the problems the planner already found. Give them advice they can act on.
Rules:
- Plain language, no jargon. Short: at most 6 points, each one or two sentences, as a Markdown list. Start with the most useful.
- Be concrete: name the piece and the room and say where to move it or what size to look for ("move the desk to the window wall of the office: daylight from the left and 90 cm free behind the chair").
- Use usual clearances: 60–75 cm to walk past a bed, 90 cm for a main walkway, 75–90 cm behind a dining chair, 70 cm in front of a wardrobe with doors, 100–120 cm in a kitchen between counters, TV distance about 1.5–2.5 × the screen size.
- Say when something is already good. Never invent rooms or furniture that are not listed.`

func (s *Server) handleAITips(w http.ResponseWriter, r *http.Request) {
	uid, st, ok := s.aiGate(w, r)
	if !ok {
		return
	}
	defer s.aiDone()
	ctx := r.Context()
	var in struct {
		LayoutID int64    `json:"layout_id"`
		Issues   []string `json:"issues"`
		Question string   `json:"question"`
	}
	if err := decode(r, &in); err != nil {
		writeErr(w, http.StatusBadRequest, "invalid json")
		return
	}
	l, err := s.store.GetLayout(ctx, uid, in.LayoutID)
	if err != nil {
		s.fail(w, err, "layout")
		return
	}
	d, _ := store.ParseDrawing(l.Data)
	desc := describeLayout(d, st.Units)
	if len(in.Issues) > 0 {
		desc += "\nProblems the planner found:\n- " + strings.Join(in.Issues[:min(len(in.Issues), 30)], "\n- ")
	}
	if q := strings.TrimSpace(in.Question); q != "" {
		desc += "\nThe person asks: " + clip(q, 1000)
	}
	cl, _, why := s.aiFor(ctx, uid, st)
	if cl == nil {
		writeErr(w, http.StatusBadRequest, firstNonEmpty(why, "AI is not set up (Settings → AI)"))
		return
	}
	text, err := cl.Complete(ctx, tipsSystem, desc, false)
	if err != nil {
		writeErr(w, http.StatusBadGateway, "AI: "+err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"text": strings.TrimSpace(text)})
}

// describeLayout writes a layout out for the AI (x to the right, y down, cm).
func describeLayout(d store.Drawing, units string) string {
	var b strings.Builder
	b.WriteString("All lengths in cm; x to the right (east), y down (south) as on the screen.\nRooms:\n")
	for _, r := range d.Rooms {
		minX, minY, maxX, maxY := bounds(r.Points)
		fmt.Fprintf(&b, "- %s (%s): %s, spans x %.0f–%.0f, y %.0f–%.0f", r.Name, firstNonEmpty(r.Kind, "room"), areaText(r.Area(), units), minX, maxX, minY, maxY)
		ops := []string{}
		for _, o := range d.OpeningsOf(r.ID) {
			ops = append(ops, fmt.Sprintf("%s %.0f wide on the %s wall", o.Kind, o.Width, edgeWord(r, o.Edge)))
		}
		if len(ops) > 0 {
			b.WriteString("; " + strings.Join(ops, ", "))
		}
		b.WriteString("\n")
	}
	b.WriteString("Furniture (centre x, y; width × depth; turned degrees):\n")
	items := append([]store.DItem(nil), d.Items...)
	sort.Slice(items, func(i, j int) bool { return d.RoomOf(items[i]) < d.RoomOf(items[j]) })
	for _, it := range items {
		fmt.Fprintf(&b, "- %s [%s] in %s at %.0f, %.0f; %.0f × %.0f", firstNonEmpty(it.Name, it.Type), it.Type, firstNonEmpty(d.RoomOf(it), "no room"), it.X, it.Y, it.W, it.D)
		if it.Rot != 0 {
			fmt.Fprintf(&b, "; turned %.0f°", it.Rot)
		}
		b.WriteString("\n")
	}
	return b.String()
}

func bounds(pts []store.Pt) (float64, float64, float64, float64) {
	if len(pts) == 0 {
		return 0, 0, 0, 0
	}
	minX, minY, maxX, maxY := pts[0].X, pts[0].Y, pts[0].X, pts[0].Y
	for _, p := range pts {
		minX, minY, maxX, maxY = math.Min(minX, p.X), math.Min(minY, p.Y), math.Max(maxX, p.X), math.Max(maxY, p.Y)
	}
	return minX, minY, maxX, maxY
}

// edgeWord says which side of the room an edge is on.
func edgeWord(r store.DRoom, e int) string {
	n := len(r.Points)
	if n < 2 || e < 0 || e >= n {
		return "?"
	}
	a, c := r.Points[e], r.Points[(e+1)%n]
	cx, cy := 0.0, 0.0
	for _, p := range r.Points {
		cx += p.X
		cy += p.Y
	}
	cx, cy = cx/float64(n), cy/float64(n)
	mx, my := (a.X+c.X)/2-cx, (a.Y+c.Y)/2-cy
	if math.Abs(mx) > math.Abs(my) {
		if mx > 0 {
			return "east"
		}
		return "west"
	}
	if my > 0 {
		return "south"
	}
	return "north"
}

var _ = context.Background
