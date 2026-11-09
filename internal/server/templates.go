package server

import (
	"fmt"
	"net/http"

	"github.com/abjelosevic88/pocket-homelab-rooms/internal/store"
)

// Ready-made starting points for a new plan, and the pieces the demo uses.
// Rooms are drawn by their inside size with a wall's thickness between them.

const wallGap = 12

type drawer struct {
	d store.Drawing
	n int
}

func newDrawer() *drawer {
	d := store.Drawing{}
	d.Normalize()
	return &drawer{d: d}
}

func (b *drawer) id(p string) string { b.n++; return fmt.Sprintf("%s%d", p, b.n) }

// room adds a rectangular room; edges: 0 top, 1 right, 2 bottom, 3 left.
func (b *drawer) room(name, kind string, x, y, w, d float64, floor string, open ...int) string {
	id := b.id("r")
	b.d.Rooms = append(b.d.Rooms, store.DRoom{ID: id, Name: name, Kind: kind, Floor: floor, Wall: wallGap, Open: open,
		Points: []store.Pt{{X: x, Y: y}, {X: x + w, Y: y}, {X: x + w, Y: y + d}, {X: x, Y: y + d}}})
	return id
}

func (b *drawer) door(room string, edge int, at, width float64, hinge string, out bool, kind ...string) {
	k := "door"
	if len(kind) > 0 {
		k = kind[0]
	}
	b.d.Openings = append(b.d.Openings, store.DOpening{ID: b.id("o"), Kind: k, Room: room, Edge: edge, At: at, Width: width, Height: 205, Hinge: hinge, Out: out})
}

func (b *drawer) window(room string, edge int, at, width float64) {
	b.d.Openings = append(b.d.Openings, store.DOpening{ID: b.id("o"), Kind: "window", Room: room, Edge: edge, At: at, Width: width, Height: 140, Sill: 90})
}

func (b *drawer) item(typ, name string, x, y, w, d, h, rot float64, opts ...func(*store.DItem)) {
	it := store.DItem{ID: b.id("i"), Type: typ, Name: name, X: x, Y: y, W: w, D: d, H: h, Rot: rot}
	for _, o := range opts {
		o(&it)
	}
	b.d.Items = append(b.d.Items, it)
}

func buy(price int64) func(*store.DItem) {
	return func(it *store.DItem) { it.Buy, it.Price = true, price }
}

func color(c string) func(*store.DItem) { return func(it *store.DItem) { it.Color = c } }

func piece(id int64) func(*store.DItem) { return func(it *store.DItem) { it.PieceID = id } }

var templates = []map[string]string{
	{"key": "empty", "name": "Start empty", "about": "A blank floor: draw your own rooms"},
	{"key": "room", "name": "One room", "about": "A single room of the size you give"},
	{"key": "studio", "name": "Studio flat", "about": "About 34 m²: one room with a kitchen corner, a bathroom and a hall"},
	{"key": "one_bed", "name": "One-bedroom flat", "about": "About 55 m²: living room, bedroom, kitchen, bathroom, hall"},
	{"key": "two_bed", "name": "Two-bedroom flat", "about": "About 75 m²: living room with an open kitchen, two bedrooms, bathroom, toilet, hall"},
	{"key": "house", "name": "House, ground floor", "about": "About 90 m²: living room, kitchen with dining, office, toilet, hall, a terrace"},
}

// templateDrawing builds a starting point (w, d for "room").
func templateDrawing(key string, w, d float64) store.Drawing {
	b := newDrawer()
	g := float64(wallGap)
	switch key {
	case "room":
		if w <= 0 {
			w = 400
		}
		if d <= 0 {
			d = 350
		}
		r := b.room("Room", "living", 0, 0, w, d, "wood")
		b.door(r, 2, w-60, 80, "end", false)
		b.window(r, 0, w/2, 120)
	case "studio":
		main := b.room("Living & sleeping", "living", 0, 0, 560, 420, "wood")
		b.window(main, 0, 150, 140)
		b.window(main, 0, 420, 140)
		hall := b.room("Hall", "hall", 0, 420+g, 200, 160, "tile")
		b.door(hall, 0, 100, 90, "start", false, "opening")
		b.door(hall, 2, 100, 90, "end", true)
		bath := b.room("Bathroom", "bath", 200+g, 420+g, 200, 160, "tile")
		b.door(bath, 3, 80, 70, "start", false)
		kit := b.room("Kitchen", "kitchen", 400+2*g, 420+g, 160-2*g, 160, "tile")
		b.door(kit, 0, 74, 120, "start", false, "opening")
	case "one_bed":
		liv := b.room("Living room", "living", 0, 0, 460, 500, "wood")
		b.window(liv, 0, 230, 180)
		bed := b.room("Bedroom", "bedroom", 460+g, 0, 360, 400, "wood")
		b.window(bed, 0, 180, 140)
		hall := b.room("Hall", "hall", 460+g, 400+g, 360, 100-g+80, "tile")
		b.door(hall, 0, 300, 80, "end", false)
		b.door(hall, 3, 130, 80, "start", false)
		b.door(hall, 2, 60, 90, "start", true)
		kit := b.room("Kitchen", "kitchen", 0, 500+g, 280, 260, "tile")
		b.window(kit, 2, 140, 120)
		b.door(kit, 0, 220, 80, "end", false)
		bath := b.room("Bathroom", "bath", 280+g, 500+g, 180-g, 260, "tile")
		b.door(bath, 1, 60, 70, "start", false)
		_ = hall
	case "two_bed":
		liv := b.room("Living room & kitchen", "living", 0, 0, 620, 480, "wood")
		b.window(liv, 0, 180, 200)
		b.window(liv, 3, 240, 140)
		b1 := b.room("Bedroom", "bedroom", 620+g, 0, 380, 380, "wood")
		b.window(b1, 0, 190, 140)
		b2 := b.room("Kids' room", "kids", 620+g, 380+g, 380, 300, "wood")
		b.window(b2, 1, 150, 120)
		hall := b.room("Hall", "hall", 0, 480+g, 620, 140, "tile")
		b.door(hall, 0, 400, 120, "start", false, "opening")
		b.door(hall, 1, 70, 80, "start", false)
		b.door(hall, 2, 80, 90, "start", true)
		bath := b.room("Bathroom", "bath", 160+g, 620+2*g, 240, 220, "tile")
		b.door(bath, 0, 120, 70, "start", false)
		wc := b.room("Toilet", "toilet", 400+2*g, 620+2*g, 120, 160, "tile")
		b.door(wc, 0, 60, 70, "end", false)
		b.door(b1, 2, 60, 80, "end", false)
	case "house":
		liv := b.room("Living room", "living", 0, 0, 600, 520, "wood")
		b.window(liv, 0, 200, 240)
		b.window(liv, 3, 260, 160)
		kit := b.room("Kitchen & dining", "kitchen", 600+g, 0, 420, 520, "tile")
		b.window(kit, 0, 210, 160)
		b.door(kit, 3, 260, 140, "start", false, "opening")
		hall := b.room("Hall", "hall", 0, 520+g, 600, 200, "tile")
		b.door(hall, 0, 300, 90, "start", false)
		b.door(hall, 2, 300, 100, "end", true)
		off := b.room("Office", "office", 600+g, 520+g, 260, 300, "wood")
		b.door(off, 3, 100, 80, "start", false)
		b.window(off, 2, 130, 120)
		wc := b.room("Toilet", "toilet", 860+2*g, 520+g, 160-g, 200, "tile")
		b.door(wc, 2, 70, 70, "start", false)
		terr := b.room("Terrace", "balcony", 0, -360-g, 600, 360, "deck", 2)
		b.door(liv, 0, 460, 160, "start", false, "sliding")
		_ = terr
	}
	return b.d
}

func (s *Server) handleTemplates(w http.ResponseWriter, r *http.Request) {
	out := []map[string]any{}
	for _, t := range templates {
		out = append(out, map[string]any{"key": t["key"], "name": t["name"], "about": t["about"], "drawing": templateDrawing(t["key"], 0, 0)})
	}
	writeJSON(w, http.StatusOK, out)
}
