package store

import (
	"encoding/json"
	"math"
	"strings"
)

// The drawing of a layout. Everything is in centimetres on a flat plane,
// x to the right and y down, the way it is drawn on screen. Rooms are the
// inside outline of a room (what you measure with a tape); their walls are
// drawn outside the outline. Doors and windows sit on one edge of a room.

// Pt is a point on the plan.
type Pt struct {
	X float64 `json:"x"`
	Y float64 `json:"y"`
}

// DRoom is a room: its inside outline.
type DRoom struct {
	ID     string  `json:"id"`
	Name   string  `json:"name"`
	Kind   string  `json:"kind,omitempty"`
	Points []Pt    `json:"points"`
	Floor  string  `json:"floor,omitempty"` // finish: wood, tile, carpet…
	Color  string  `json:"color,omitempty"`
	Wall   float64 `json:"wall,omitempty"` // wall thickness (cm)
	Open   []int   `json:"open,omitempty"` // edges with no wall (an open kitchen)
	Height float64 `json:"height,omitempty"`
}

// DWall is a wall on its own (not around a room).
type DWall struct {
	ID    string  `json:"id"`
	A     Pt      `json:"a"`
	B     Pt      `json:"b"`
	Thick float64 `json:"thick,omitempty"`
}

// DOpening is a door, window or opening on an edge of a room (or a wall).
type DOpening struct {
	ID     string  `json:"id"`
	Kind   string  `json:"kind"`
	Room   string  `json:"room,omitempty"`
	Edge   int     `json:"edge"`
	Wall   string  `json:"wall,omitempty"`
	At     float64 `json:"at"` // centre, cm from the edge's start
	Width  float64 `json:"width"`
	Height float64 `json:"height,omitempty"`
	Sill   float64 `json:"sill,omitempty"`
	Hinge  string  `json:"hinge,omitempty"` // start | end
	Out    bool    `json:"out,omitempty"`   // opens out of the room
	Name   string  `json:"name,omitempty"`
}

// DItem is a piece of furniture standing on the plan. X, Y is its centre.
type DItem struct {
	ID      string  `json:"id"`
	Name    string  `json:"name"`
	Type    string  `json:"type"`
	Cat     string  `json:"cat,omitempty"`
	X       float64 `json:"x"`
	Y       float64 `json:"y"`
	W       float64 `json:"w"`
	D       float64 `json:"d"`
	H       float64 `json:"h,omitempty"`
	Rot     float64 `json:"rot,omitempty"`
	Elev    float64 `json:"elev,omitempty"` // off the floor (a wall cupboard)
	Color   string  `json:"color,omitempty"`
	PieceID int64   `json:"piece_id,omitempty"`
	Price   int64   `json:"price,omitempty"`
	Buy     bool    `json:"buy,omitempty"` // still to buy
	Locked  bool    `json:"locked,omitempty"`
	Note    string  `json:"note,omitempty"`
	From    string  `json:"from,omitempty"` // home:thing:12, inventory:item:4…
}

// DDim is a measurement line drawn on the plan.
type DDim struct {
	ID  string  `json:"id"`
	A   Pt      `json:"a"`
	B   Pt      `json:"b"`
	Off float64 `json:"off,omitempty"`
}

// DLabel is a note written on the plan.
type DLabel struct {
	ID   string  `json:"id"`
	At   Pt      `json:"at"`
	Text string  `json:"text"`
	Size float64 `json:"size,omitempty"`
}

// Drawing is the whole layout.
type Drawing struct {
	Rooms    []DRoom    `json:"rooms"`
	Walls    []DWall    `json:"walls"`
	Openings []DOpening `json:"openings"`
	Items    []DItem    `json:"items"`
	Dims     []DDim     `json:"dims"`
	Labels   []DLabel   `json:"labels"`
}

// ParseDrawing reads a layout's data ({} or empty = an empty drawing).
func ParseDrawing(raw []byte) (Drawing, error) {
	var d Drawing
	if len(raw) > 0 && strings.TrimSpace(string(raw)) != "" {
		if err := json.Unmarshal(raw, &d); err != nil {
			return d, err
		}
	}
	d.Normalize()
	return d, nil
}

// Normalize fills nil lists so the JSON is always arrays.
func (d *Drawing) Normalize() {
	if d.Rooms == nil {
		d.Rooms = []DRoom{}
	}
	if d.Walls == nil {
		d.Walls = []DWall{}
	}
	if d.Openings == nil {
		d.Openings = []DOpening{}
	}
	if d.Items == nil {
		d.Items = []DItem{}
	}
	if d.Dims == nil {
		d.Dims = []DDim{}
	}
	if d.Labels == nil {
		d.Labels = []DLabel{}
	}
	for i := range d.Rooms {
		if d.Rooms[i].Points == nil {
			d.Rooms[i].Points = []Pt{}
		}
	}
}

// Area is a room's floor area in cm² (shoelace).
func (r DRoom) Area() float64 { return PolyArea(r.Points) }

// Perimeter is the length of a room's outline in cm.
func (r DRoom) Perimeter() float64 {
	n := len(r.Points)
	p := 0.0
	for i := 0; i < n; i++ {
		a, b := r.Points[i], r.Points[(i+1)%n]
		p += math.Hypot(b.X-a.X, b.Y-a.Y)
	}
	return p
}

// EdgeLen is the length of edge i.
func (r DRoom) EdgeLen(i int) float64 {
	n := len(r.Points)
	if n < 2 || i < 0 || i >= n {
		return 0
	}
	a, b := r.Points[i], r.Points[(i+1)%n]
	return math.Hypot(b.X-a.X, b.Y-a.Y)
}

// PolyArea is the area of a polygon (always positive).
func PolyArea(pts []Pt) float64 {
	n := len(pts)
	if n < 3 {
		return 0
	}
	s := 0.0
	for i := 0; i < n; i++ {
		a, b := pts[i], pts[(i+1)%n]
		s += a.X*b.Y - b.X*a.Y
	}
	return math.Abs(s) / 2
}

// Contains reports whether p is inside the polygon.
func Contains(pts []Pt, p Pt) bool {
	in := false
	n := len(pts)
	for i, j := 0, n-1; i < n; j, i = i, i+1 {
		a, b := pts[i], pts[j]
		if (a.Y > p.Y) != (b.Y > p.Y) && p.X < (b.X-a.X)*(p.Y-a.Y)/(b.Y-a.Y)+a.X {
			in = !in
		}
	}
	return in
}

// RoomOf is the name of the room an item stands in ("" outside every room).
func (d Drawing) RoomOf(it DItem) string {
	for _, r := range d.Rooms {
		if Contains(r.Points, Pt{it.X, it.Y}) {
			return r.Name
		}
	}
	return ""
}

// Totals are the numbers a layout's columns keep.
func (d Drawing) Totals() (area float64, rooms, items int, budget int64) {
	for _, r := range d.Rooms {
		if r.Kind == "balcony" || r.Kind == "garden" {
			continue
		}
		area += r.Area()
	}
	for _, it := range d.Items {
		if it.Buy && it.Price > 0 {
			budget += it.Price
		}
	}
	return area, len(d.Rooms), len(d.Items), budget
}

// OpeningsOf are the openings on a room's edges.
func (d Drawing) OpeningsOf(roomID string) []DOpening {
	out := []DOpening{}
	for _, o := range d.Openings {
		if o.Room == roomID {
			out = append(out, o)
		}
	}
	return out
}
