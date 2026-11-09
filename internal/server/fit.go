package server

import (
	"fmt"
	"math"
	"net/http"
	"sort"

	"github.com/abjelosevic88/pocket-homelab-rooms/internal/store"
)

// ---- Will it fit? ----
//
// A piece of furniture (width × depth × height) has to pass every step of
// the way in: doors, hallways, corners, stairs, a lift. For each step we try
// every way of carrying it (which side is up) and say how it goes through,
// or why it does not.
//
// Straight through a door or a hallway: two of its sides must fit the
// opening (width × height); the third goes along.
//
// Round a corner (a hallway turning, or a door off a hallway): in plan the
// piece is a rectangle, long side L and short side a, carried at some height
// h that fits under the ceiling. It passes without turning when L fits the
// second leg's width; otherwise it must swing round the inner corner, and
// the longest rectangle that can is the minimum over θ of
//
//	L(θ) = p/sin θ + q/cos θ − a/(sin θ cos θ)
//
// with p, q the two legs' widths (the ladder-round-a-corner problem for a
// rectangle). A lift is a box: the piece fits lying, standing, or across
// the floor's diagonal (the rectangle-in-rectangle test).

// FitStep is the result of one step.
type FitStep struct {
	Step   store.RouteStep `json:"step"`
	OK     bool            `json:"ok"`
	How    string          `json:"how"`
	Spare  float64         `json:"spare"`             // cm to spare at the tightest point (negative = short by)
	Carry  [3]float64      `json:"carry,omitempty"`   // high, across, along
	MaxLen float64         `json:"max_len,omitempty"` // corners: the longest piece of this size that turns
}

// FitResult is the whole way in.
type FitResult struct {
	OK      bool      `json:"ok"`
	Steps   []FitStep `json:"steps"`
	Summary string    `json:"summary"`
}

// orientations are the ways to carry a piece: (high, across, along).
func orientations(w, d, h float64) [][3]float64 {
	dims := []float64{w, d, h}
	out := [][3]float64{}
	seen := map[[3]float64]bool{}
	for i := 0; i < 3; i++ {
		for j := 0; j < 3; j++ {
			if i == j {
				continue
			}
			k := 3 - i - j
			o := [3]float64{dims[i], dims[j], dims[k]}
			if !seen[o] {
				seen[o] = true
				out = append(out, o)
			}
		}
	}
	return out
}

// cornerMax is the longest rectangle of width a that turns from a hallway
// of width p into one of width q.
func cornerMax(p, q, a float64) float64 {
	if a >= p || a >= q {
		return 0
	}
	best := math.Inf(1)
	for t := 0.002; t < math.Pi/2; t += 0.002 {
		sn, cs := math.Sin(t), math.Cos(t)
		l := p/sn + q/cs - a/(sn*cs)
		if l < best {
			best = l
		}
	}
	if best < 0 {
		return 0
	}
	return best
}

// rectFits is whether a p×q rectangle fits in an a×b one, turned if needed.
func rectFits(p, q, a, b float64) bool {
	if p < q {
		p, q = q, p
	}
	if a < b {
		a, b = b, a
	}
	if p <= a && q <= b {
		return true
	}
	if q > b || p*p+q*q < a*a {
		return false
	}
	// Carver: p > a, fits diagonally when b ≥ (2pqa + (p²−q²)√(p²+q²−a²)) / (p²+q²)
	return p > a && b >= (2*p*q*a+(p*p-q*q)*math.Sqrt(p*p+q*q-a*a))/(p*p+q*q)
}

func defaultHeight(st store.RouteStep) float64 {
	if st.Height > 0 {
		return st.Height
	}
	switch st.Kind {
	case "door":
		return 200
	case "stairs":
		return 200
	case "lift":
		return 210
	}
	return 250
}

// checkStep finds the best way through one step.
func checkStep(w, d, h float64, st store.RouteStep, hands float64, units string) FitStep {
	out := FitStep{Step: st, Spare: math.Inf(-1)}
	H := defaultHeight(st) - hands
	W := st.Width - hands
	switch st.Kind {
	case "lift":
		D := st.Depth - hands
		if D <= 0 {
			D = W
		}
		for _, o := range orientations(w, d, h) {
			if o[0] <= H && rectFits(o[2], o[1], W, D) {
				spare := math.Min(H-o[0], math.Min(W, D)-math.Min(o[1], o[2]))
				if !out.OK || spare > out.Spare {
					out.OK, out.Spare, out.Carry = true, spare, o
				}
			}
		}
		if out.OK {
			out.How = fmt.Sprintf("Fits standing %s high", lenText(out.Carry[0], units))
		} else {
			out.How = "Too big for the lift in every direction"
			out.Spare = 0
		}
		return out
	case "corner", "door", "hall", "stairs":
	default:
		out.How = "Unknown step"
		return out
	}
	turn := st.Turn - hands
	if st.Kind == "corner" && turn <= 0 {
		turn = W
	}
	turning := (st.Kind == "corner" || (st.Kind == "door" && st.Turn > 0) || (st.Kind == "stairs" && st.Turn > 0)) && turn > 0
	// how high a tilted piece may reach while it turns: under the door, or the ceiling
	tiltH := H
	if st.Kind == "corner" {
		tiltH = defaultHeight(st) - hands
	}
	best := struct {
		ok          bool
		spare, tilt float64
		carry       [3]float64
		maxLen      float64
		diag        bool
	}{spare: math.Inf(-1)}
	short := math.Inf(-1) // how close the best failing try came
	take := func(spare float64, o [3]float64, tilt, mx float64, diag bool) {
		if !best.ok || spare > best.spare {
			best.ok, best.spare, best.carry, best.tilt, best.maxLen, best.diag = true, spare, o, tilt, mx, diag
		}
	}
	for _, o := range orientations(w, d, h) {
		hi, across, along := o[0], o[1], o[2]
		// the end of the piece through the opening: straight, or turned diagonally
		straight := hi <= H && across <= W
		diag := !straight && rectFits(across, hi, W, H)
		if !straight && !diag {
			if gap := math.Min(H-hi, W-across); gap > short {
				short = gap
			}
			continue
		}
		if !turning {
			spare := 0.0
			if straight {
				spare = math.Min(H-hi, W-across)
			}
			take(spare, o, 0, 0, diag)
			continue
		}
		if !straight || across > turn {
			if turn-across > short {
				short = turn - across
			}
			continue
		}
		mx := cornerMax(turn, W, across)
		// flat, then tilted up a little at a time: the length it needs in plan shrinks
		passed := false
		for deg := 0.0; deg <= 75; deg += 2.5 {
			r := deg * math.Pi / 180
			lh := along*math.Cos(r) + hi*math.Sin(r)
			vh := along*math.Sin(r) + hi*math.Cos(r)
			if vh > tiltH {
				break
			}
			if lh <= W { // no need to swing it: slide it round
				take(math.Min(math.Min(tiltH-vh, W-lh), turn-across), o, deg, mx, false)
				passed = true
				break
			}
			if lh <= mx {
				take(math.Min(tiltH-vh, mx-lh), o, deg, mx, false)
				passed = true
				break
			}
		}
		if !passed && mx-along > short {
			short = mx - along
		}
	}
	out.OK, out.Spare, out.Carry, out.MaxLen = best.ok, best.spare, best.carry, best.maxLen
	if !out.OK {
		if math.IsInf(short, -1) {
			short = 0
		}
		out.Spare = short
		if turning {
			out.How = fmt.Sprintf("Does not make the turn: about %s too long or wide, even tilted", lenText(-short, units))
		} else {
			out.How = fmt.Sprintf("Too big: about %s short whichever way you carry it", lenText(-short, units))
		}
		return out
	}
	hi, across := out.Carry[0], out.Carry[1]
	way := "flat"
	switch {
	case hi == h:
		way = "upright"
	case hi == math.Max(w, math.Max(d, h)):
		way = "on its end"
	default:
		way = "on its side"
	}
	out.How = fmt.Sprintf("Fits carried %s (%s high, %s wide)", way, lenText(hi, units), lenText(across, units))
	if best.diag {
		out.How += ", turned diagonally in the doorway"
	}
	if turning {
		if best.tilt > 0 {
			out.How += fmt.Sprintf(" — tilt it up about %.0f° to make the turn", best.tilt)
		} else {
			out.How += fmt.Sprintf(" — the turn takes up to %s long", lenText(out.MaxLen, units))
		}
	}
	return out
}

// checkFit runs a piece through every step.
func checkFit(w, d, h float64, steps []store.RouteStep, hands float64, units string) FitResult {
	res := FitResult{OK: true, Steps: []FitStep{}}
	worst := ""
	for _, st := range steps {
		r := checkStep(w, d, h, st, hands, units)
		r.Spare = math.Round(r.Spare*10) / 10
		r.MaxLen = math.Round(r.MaxLen*10) / 10
		res.Steps = append(res.Steps, r)
		if !r.OK {
			res.OK = false
			if worst == "" {
				worst = firstNonEmpty(st.Name, stepWord(st.Kind))
			}
		}
	}
	switch {
	case len(steps) == 0:
		res.OK = false
		res.Summary = "Add the doors and hallways on the way in first"
	case res.OK:
		tight := math.Inf(1)
		for _, r := range res.Steps {
			tight = math.Min(tight, r.Spare)
		}
		res.Summary = fmt.Sprintf("It fits all the way in, with %s to spare at the tightest point", lenText(math.Max(0, tight), units))
	default:
		res.Summary = "It gets stuck at " + worst
	}
	return res
}

func stepWord(k string) string {
	switch k {
	case "door":
		return "the door"
	case "hall":
		return "the hallway"
	case "corner":
		return "the corner"
	case "stairs":
		return "the stairs"
	case "lift":
		return "the lift"
	}
	return k
}

// cleanSteps keeps the steps that make sense.
func cleanSteps(in []store.RouteStep) ([]store.RouteStep, error) {
	out := []store.RouteStep{}
	for _, st := range in {
		if !oneOf(st.Kind, store.StepKinds) {
			return nil, &userErr{"a step is a door, a hallway, a corner, stairs or a lift"}
		}
		if st.Width <= 0 || st.Width > 2000 {
			return nil, &userErr{"every step needs its width in centimetres"}
		}
		st.Name = cleanText(st.Name, maxName)
		st.Height = clampF(st.Height, 0, 2000)
		st.Turn = clampF(st.Turn, 0, 2000)
		st.Depth = clampF(st.Depth, 0, 2000)
		out = append(out, st)
		if len(out) >= 30 {
			break
		}
	}
	return out, nil
}

// handleFit checks a size (or a piece) against steps (or a saved route).
func (s *Server) handleFit(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	uid := actorID(ctx)
	var in struct {
		PieceID int64             `json:"piece_id"`
		Width   float64           `json:"width"`
		Depth   float64           `json:"depth"`
		Height  float64           `json:"height"`
		RouteID int64             `json:"route_id"`
		Steps   []store.RouteStep `json:"steps"`
		Hands   *float64          `json:"hands"`
	}
	if err := decode(r, &in); err != nil {
		writeErr(w, http.StatusBadRequest, "invalid json")
		return
	}
	st := s.settingsOf(ctx, uid)
	name := ""
	if in.PieceID > 0 {
		p, err := s.store.GetPiece(ctx, uid, in.PieceID)
		if err != nil {
			s.fail(w, err, "piece")
			return
		}
		in.Width, in.Depth, in.Height, name = p.Width, p.Depth, p.Height, p.Name
	}
	if in.Width <= 0 || in.Depth <= 0 || in.Height <= 0 {
		writeErr(w, http.StatusBadRequest, "give the piece's width, depth and height")
		return
	}
	steps := in.Steps
	if in.RouteID > 0 {
		rt, err := s.store.GetRoute(ctx, uid, in.RouteID)
		if err != nil {
			s.fail(w, err, "route")
			return
		}
		steps = rt.Steps
	}
	steps, err := cleanSteps(steps)
	if err != nil {
		s.userFail(w, err, "fit")
		return
	}
	hands := st.HandsCm
	if in.Hands != nil {
		hands = clampF(*in.Hands, 0, 30)
	}
	res := checkFit(in.Width, in.Depth, in.Height, steps, hands, st.Units)
	writeJSON(w, http.StatusOK, map[string]any{"result": res, "name": name, "size": sizeText(in.Width, in.Depth, in.Height, st.Units)})
}

// fitsAll checks pieces against every route of a plan (for the furniture list).
func fitsAll(p store.Piece, routes []store.Route, hands float64, units string) map[int64]bool {
	out := map[int64]bool{}
	for _, rt := range routes {
		if len(rt.Steps) == 0 {
			continue
		}
		out[rt.ID] = checkFit(p.Width, p.Depth, p.Height, rt.Steps, hands, units).OK
	}
	return out
}

func sortedDims(w, d, h float64) []float64 {
	x := []float64{w, d, h}
	sort.Float64s(x)
	return x
}
