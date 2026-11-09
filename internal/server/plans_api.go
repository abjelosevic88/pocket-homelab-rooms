package server

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/abjelosevic88/pocket-homelab-rooms/internal/store"
)

// ---- plans ----

// planTotals fills a plan's worked-out numbers from its current layouts.
func (s *Server) planTotals(ctx context.Context, uid int64, p *store.Plan, withPreview bool) {
	floors, _ := s.store.ListFloors(ctx, uid, p.ID)
	layouts, _ := s.store.ListLayouts(ctx, uid, p.ID, false)
	byID := map[int64]store.Layout{}
	first := map[int64]int64{}
	for _, l := range layouts {
		byID[l.ID] = l
		if _, ok := first[l.FloorID]; !ok {
			first[l.FloorID] = l.ID
		}
	}
	p.AreaCm2, p.RoomsN, p.ItemsN, p.Budget = 0, 0, 0, 0
	for i, f := range floors {
		cur := first[f.ID]
		if f.CurrentLayoutID != nil {
			if _, ok := byID[*f.CurrentLayoutID]; ok {
				cur = *f.CurrentLayoutID
			}
		}
		l, ok := byID[cur]
		if !ok {
			continue
		}
		p.AreaCm2 += l.AreaCm2
		p.RoomsN += l.RoomsN
		p.ItemsN += l.ItemsN
		p.Budget += l.Budget
		if withPreview && i == 0 {
			if full, err := s.store.GetLayout(ctx, uid, l.ID); err == nil {
				p.Preview = full
			}
		}
	}
	p.OpenTodo = s.store.OpenTodos(ctx, uid, p.ID)
}

// currentLayout is a floor's "as it is" layout (else its first).
func currentOf(f store.Floor, layouts []store.Layout) *store.Layout {
	var first *store.Layout
	for i := range layouts {
		l := &layouts[i]
		if l.FloorID != f.ID {
			continue
		}
		if first == nil {
			first = l
		}
		if f.CurrentLayoutID != nil && *f.CurrentLayoutID == l.ID {
			return l
		}
	}
	return first
}

func (s *Server) handleListPlans(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	uid := actorID(ctx)
	plans, err := s.store.ListPlans(ctx, uid, queryBool(r, "archived"))
	if err != nil {
		s.fail(w, err, "plans")
		return
	}
	preview := !queryBool(r, "lean") && linkApp(ctx) == ""
	for i := range plans {
		s.planTotals(ctx, uid, &plans[i], preview)
	}
	writeJSON(w, http.StatusOK, plans)
}

type planIn struct {
	Name       *string          `json:"name"`
	Kind       *string          `json:"kind"`
	Address    *string          `json:"address"`
	Currency   *string          `json:"currency"`
	Notes      *string          `json:"notes"`
	Status     *string          `json:"status"`
	HomeID     *int64           `json:"home_id"`
	HomeName   *string          `json:"home_name"`
	RentalID   *int64           `json:"rental_id"`
	RentalName *string          `json:"rental_name"`
	Pinned     *bool            `json:"pinned"`
	Archived   *bool            `json:"archived"`
	Template   string           `json:"template"`
	Width      float64          `json:"width"`
	Depth      float64          `json:"depth"`
	FloorName  string           `json:"floor_name"`
	Ceiling    float64          `json:"ceiling_cm"`
	Drawing    *json.RawMessage `json:"drawing"`
}

func (in planIn) apply(keys map[string]bool, p *store.Plan) error {
	str := func(k string, dst *string, v *string, n int) {
		if keys[k] && v != nil {
			*dst = cleanText(*v, n)
		}
	}
	str("name", &p.Name, in.Name, maxName)
	str("address", &p.Address, in.Address, maxLine)
	str("notes", &p.Notes, in.Notes, maxNotes)
	str("home_name", &p.HomeName, in.HomeName, maxName)
	str("rental_name", &p.RentalName, in.RentalName, maxName)
	if keys["kind"] && in.Kind != nil {
		if !oneOf(*in.Kind, store.PlanKinds) {
			return &userErr{"a plan is a flat, a house, an office, a studio, a garden or something else"}
		}
		p.Kind = *in.Kind
	}
	if keys["status"] && in.Status != nil {
		if !oneOf(*in.Status, store.PlanStates) {
			return &userErr{"unknown status"}
		}
		p.Status = *in.Status
	}
	if keys["currency"] && in.Currency != nil {
		p.Currency = cleanCurrency(*in.Currency)
	}
	if keys["home_id"] {
		p.HomeID = in.HomeID
		if p.HomeID != nil && *p.HomeID == 0 {
			p.HomeID = nil
		}
		if p.HomeID == nil {
			p.HomeName = ""
		}
	}
	if keys["rental_id"] {
		p.RentalID = in.RentalID
		if p.RentalID != nil && *p.RentalID == 0 {
			p.RentalID = nil
		}
		if p.RentalID == nil {
			p.RentalName = ""
		}
	}
	if keys["pinned"] && in.Pinned != nil {
		p.Pinned = *in.Pinned
	}
	if keys["archived"] && in.Archived != nil {
		p.Archived = *in.Archived
	}
	if strings.TrimSpace(p.Name) == "" {
		return &userErr{"give the plan a name"}
	}
	return nil
}

func (s *Server) handleCreatePlan(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	uid := actorID(ctx)
	var in planIn
	keys, err := decodeKeys(r, &in)
	if err != nil {
		writeErr(w, http.StatusBadRequest, "invalid json")
		return
	}
	st := s.settingsOf(ctx, uid)
	p := &store.Plan{UserID: uid, Kind: "apartment", Status: "living", Currency: firstNonEmpty(st.Currency, "EUR"), Source: sourceOf(ctx)}
	if err := in.apply(keys, p); err != nil {
		s.userFail(w, err, "plan")
		return
	}
	var drawing store.Drawing
	if in.Drawing != nil {
		drawing, err = store.ParseDrawing(*in.Drawing)
		if err != nil {
			writeErr(w, http.StatusBadRequest, "the drawing is not valid")
			return
		}
	} else {
		drawing = templateDrawing(in.Template, in.Width, in.Depth)
	}
	if err := s.store.CreatePlan(ctx, p); err != nil {
		s.fail(w, err, "create plan")
		return
	}
	ceiling := in.Ceiling
	if ceiling <= 0 {
		ceiling = firstPos(st.CeilingCm, 250)
	}
	fname := firstNonEmpty(cleanText(in.FloorName, maxName), "Ground floor")
	if p.Kind == "apartment" || p.Kind == "studio" || p.Kind == "office" {
		fname = firstNonEmpty(cleanText(in.FloorName, maxName), "The flat")
		if p.Kind == "office" {
			fname = firstNonEmpty(cleanText(in.FloorName, maxName), "The office")
		}
	}
	if _, err := s.addFloor(ctx, uid, p.ID, fname, 0, ceiling, drawing); err != nil {
		s.fail(w, err, "create floor")
		return
	}
	s.publish(ctx, uid, "rooms.plan_created", fmt.Sprintf("plan:%d", p.ID), map[string]any{"plan_id": p.ID, "name": p.Name, "kind": p.Kind, "url": s.link(fmt.Sprintf("/plans/%d", p.ID))})
	s.writePlan(w, r, uid, p.ID, http.StatusCreated)
}

func firstPos(v ...float64) float64 {
	for _, x := range v {
		if x > 0 {
			return x
		}
	}
	return 0
}

// addFloor adds a floor with one layout ("As it is") holding the drawing.
func (s *Server) addFloor(ctx context.Context, uid, planID int64, name string, level int, ceiling float64, d store.Drawing) (*store.Floor, error) {
	f := &store.Floor{UserID: uid, PlanID: planID, Name: name, Level: level, CeilingCm: ceiling, Sort: level}
	if err := s.store.CreateFloor(ctx, f); err != nil {
		return nil, err
	}
	d.Normalize()
	raw, _ := json.Marshal(d)
	l := &store.Layout{UserID: uid, PlanID: planID, FloorID: f.ID, Name: "As it is", Data: raw}
	if err := s.store.CreateLayout(ctx, l); err != nil {
		return nil, err
	}
	f.CurrentLayoutID = &l.ID
	return f, s.store.UpdateFloor(ctx, f)
}

// planDoc is a plan with its floors, their layouts (without drawings) and routes.
func (s *Server) planDoc(ctx context.Context, uid, id int64) (map[string]any, error) {
	p, err := s.store.GetPlan(ctx, uid, id)
	if err != nil {
		return nil, err
	}
	s.planTotals(ctx, uid, p, false)
	floors, err := s.store.ListFloors(ctx, uid, id)
	if err != nil {
		return nil, err
	}
	layouts, err := s.store.ListLayouts(ctx, uid, id, false)
	if err != nil {
		return nil, err
	}
	for i := range floors {
		f := &floors[i]
		cur := currentOf(*f, layouts)
		f.Layouts = []store.Layout{}
		for _, l := range layouts {
			if l.FloorID == f.ID {
				l.Current = cur != nil && cur.ID == l.ID
				f.Layouts = append(f.Layouts, l)
			}
		}
	}
	p.Floors = floors
	routes, _ := s.store.ListRoutes(ctx, uid, id)
	out := map[string]any{"plan": p, "routes": routes}
	if p.ShareOn {
		out["share_url"] = s.link("/s/" + p.ShareToken)
	}
	if p.HomeID != nil {
		out["home_url"] = s.siblingLink(ctx, uid, appHome, fmt.Sprintf("/homes/%d", *p.HomeID))
	}
	if p.RentalID != nil {
		out["rental_url"] = s.siblingLink(ctx, uid, appRentals, "/")
	}
	return out, nil
}

func (s *Server) writePlan(w http.ResponseWriter, r *http.Request, uid, id int64, status int) {
	doc, err := s.planDoc(r.Context(), uid, id)
	if err != nil {
		s.fail(w, err, "plan")
		return
	}
	writeJSON(w, status, doc)
}

func (s *Server) handleGetPlan(w http.ResponseWriter, r *http.Request) {
	s.writePlan(w, r, actorID(r.Context()), idParam(r, "id"), http.StatusOK)
}

func (s *Server) handleUpdatePlan(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	uid := actorID(ctx)
	p, err := s.store.GetPlan(ctx, uid, idParam(r, "id"))
	if err != nil {
		s.fail(w, err, "plan")
		return
	}
	var in planIn
	keys, err := decodeKeys(r, &in)
	if err != nil {
		writeErr(w, http.StatusBadRequest, "invalid json")
		return
	}
	if err := in.apply(keys, p); err != nil {
		s.userFail(w, err, "plan")
		return
	}
	if err := s.store.UpdatePlan(ctx, p); err != nil {
		s.fail(w, err, "save plan")
		return
	}
	s.writePlan(w, r, uid, p.ID, http.StatusOK)
}

func (s *Server) handleDeletePlan(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	if err := s.store.DeletePlan(ctx, actorID(ctx), idParam(r, "id")); err != nil {
		s.fail(w, err, "delete plan")
		return
	}
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

// handleSharePlan switches the read-only link on or off (or makes a new one).
func (s *Server) handleSharePlan(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	uid := actorID(ctx)
	p, err := s.store.GetPlan(ctx, uid, idParam(r, "id"))
	if err != nil {
		s.fail(w, err, "plan")
		return
	}
	var in struct {
		On  bool `json:"on"`
		New bool `json:"new"`
	}
	_ = decode(r, &in)
	p.ShareOn = in.On
	if p.ShareToken == "" || in.New {
		p.ShareToken = newToken()
	}
	if err := s.store.UpdatePlan(ctx, p); err != nil {
		s.fail(w, err, "share")
		return
	}
	s.writePlan(w, r, uid, p.ID, http.StatusOK)
}

// handleDuplicatePlan copies a plan with its floors, layouts and routes.
func (s *Server) handleDuplicatePlan(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	uid := actorID(ctx)
	p, err := s.store.GetPlan(ctx, uid, idParam(r, "id"))
	if err != nil {
		s.fail(w, err, "plan")
		return
	}
	floors, _ := s.store.ListFloors(ctx, uid, p.ID)
	layouts, _ := s.store.ListLayouts(ctx, uid, p.ID, true)
	routes, _ := s.store.ListRoutes(ctx, uid, p.ID)
	np := *p
	np.ID, np.Name, np.ShareOn, np.ShareToken, np.Pinned, np.Source = 0, clip(p.Name+" (copy)", maxName), false, "", false, ""
	if err := s.store.CreatePlan(ctx, &np); err != nil {
		s.fail(w, err, "copy plan")
		return
	}
	for _, f := range floors {
		nf := f
		nf.ID, nf.PlanID, nf.CurrentLayoutID = 0, np.ID, nil
		if err := s.store.CreateFloor(ctx, &nf); err != nil {
			s.fail(w, err, "copy floor")
			return
		}
		for _, l := range layouts {
			if l.FloorID != f.ID {
				continue
			}
			nl := l
			nl.ID, nl.PlanID, nl.FloorID = 0, np.ID, nf.ID
			if err := s.store.CreateLayout(ctx, &nl); err != nil {
				s.fail(w, err, "copy layout")
				return
			}
			if f.CurrentLayoutID != nil && *f.CurrentLayoutID == l.ID {
				nf.CurrentLayoutID = &nl.ID
			}
		}
		_ = s.store.UpdateFloor(ctx, &nf)
	}
	for _, rt := range routes {
		nr := rt
		nr.ID, nr.PlanID = 0, np.ID
		_ = s.store.SaveRoute(ctx, &nr)
	}
	s.writePlan(w, r, uid, np.ID, http.StatusCreated)
}

// ---- floors ----

func (s *Server) handleCreateFloor(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	uid := actorID(ctx)
	p, err := s.store.GetPlan(ctx, uid, idParam(r, "id"))
	if err != nil {
		s.fail(w, err, "plan")
		return
	}
	var in struct {
		Name     string  `json:"name"`
		Level    *int    `json:"level"`
		Ceiling  float64 `json:"ceiling_cm"`
		CopyFrom int64   `json:"copy_from"` // a layout whose outline (rooms, walls) to start from
	}
	if err := decode(r, &in); err != nil {
		writeErr(w, http.StatusBadRequest, "invalid json")
		return
	}
	floors, _ := s.store.ListFloors(ctx, uid, p.ID)
	level := len(floors)
	for _, f := range floors {
		if f.Level >= level {
			level = f.Level + 1
		}
	}
	if in.Level != nil {
		level = *in.Level
	}
	d := store.Drawing{}
	d.Normalize()
	if in.CopyFrom > 0 {
		l, err := s.store.GetLayout(ctx, uid, in.CopyFrom)
		if err != nil || l.PlanID != p.ID {
			writeErr(w, http.StatusNotFound, "not found")
			return
		}
		src, _ := store.ParseDrawing(l.Data)
		d.Rooms, d.Walls = src.Rooms, src.Walls
	}
	name := firstNonEmpty(cleanText(in.Name, maxName), floorName(level))
	st := s.settingsOf(ctx, uid)
	if _, err := s.addFloor(ctx, uid, p.ID, name, level, firstPos(in.Ceiling, st.CeilingCm, 250), d); err != nil {
		s.fail(w, err, "floor")
		return
	}
	s.store.TouchPlan(ctx, uid, p.ID)
	s.writePlan(w, r, uid, p.ID, http.StatusCreated)
}

func floorName(level int) string {
	switch {
	case level == 0:
		return "Ground floor"
	case level == 1:
		return "First floor"
	case level == 2:
		return "Second floor"
	case level == -1:
		return "Basement"
	case level < 0:
		return fmt.Sprintf("Basement %d", -level)
	}
	return fmt.Sprintf("Floor %d", level)
}

func (s *Server) handleUpdateFloor(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	uid := actorID(ctx)
	f, err := s.store.GetFloor(ctx, uid, idParam(r, "id"))
	if err != nil {
		s.fail(w, err, "floor")
		return
	}
	var in struct {
		Name            *string          `json:"name"`
		Level           *int             `json:"level"`
		Ceiling         *float64         `json:"ceiling_cm"`
		CurrentLayoutID *int64           `json:"current_layout_id"`
		UnderlayPhotoID *int64           `json:"underlay_photo_id"`
		Underlay        *json.RawMessage `json:"underlay"`
	}
	keys, err := decodeKeys(r, &in)
	if err != nil {
		writeErr(w, http.StatusBadRequest, "invalid json")
		return
	}
	if in.Name != nil {
		f.Name = firstNonEmpty(cleanText(*in.Name, maxName), f.Name)
	}
	if in.Level != nil {
		f.Level = *in.Level
		f.Sort = *in.Level
	}
	if in.Ceiling != nil && *in.Ceiling > 100 && *in.Ceiling < 2000 {
		f.CeilingCm = *in.Ceiling
	}
	if in.CurrentLayoutID != nil {
		l, err := s.store.GetLayout(ctx, uid, *in.CurrentLayoutID)
		if err != nil || l.FloorID != f.ID {
			writeErr(w, http.StatusBadRequest, "that layout is not on this floor")
			return
		}
		f.CurrentLayoutID = &l.ID
	}
	if keys["underlay_photo_id"] {
		f.UnderlayPhotoID = in.UnderlayPhotoID
		if f.UnderlayPhotoID != nil && (*f.UnderlayPhotoID == 0 || !s.store.OwnsPhoto(ctx, uid, *f.UnderlayPhotoID)) {
			f.UnderlayPhotoID = nil
		}
	}
	if in.Underlay != nil {
		var u map[string]any
		if json.Unmarshal(*in.Underlay, &u) != nil {
			writeErr(w, http.StatusBadRequest, "invalid underlay")
			return
		}
		f.Underlay = *in.Underlay
	}
	if err := s.store.UpdateFloor(ctx, f); err != nil {
		s.fail(w, err, "save floor")
		return
	}
	s.store.TouchPlan(ctx, uid, f.PlanID)
	s.writePlan(w, r, uid, f.PlanID, http.StatusOK)
}

func (s *Server) handleDeleteFloor(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	uid := actorID(ctx)
	f, err := s.store.GetFloor(ctx, uid, idParam(r, "id"))
	if err != nil {
		s.fail(w, err, "floor")
		return
	}
	floors, _ := s.store.ListFloors(ctx, uid, f.PlanID)
	if len(floors) <= 1 {
		writeErr(w, http.StatusBadRequest, "a plan keeps at least one floor")
		return
	}
	if err := s.store.DeleteFloor(ctx, uid, f.ID); err != nil {
		s.fail(w, err, "delete floor")
		return
	}
	s.writePlan(w, r, uid, f.PlanID, http.StatusOK)
}

// ---- layouts ----

func (s *Server) handleGetLayout(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	l, err := s.store.GetLayout(ctx, actorID(ctx), idParam(r, "id"))
	if err != nil {
		s.fail(w, err, "layout")
		return
	}
	writeJSON(w, http.StatusOK, l)
}

// handleCreateLayout adds an option to a floor: empty, or a copy of another.
func (s *Server) handleCreateLayout(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	uid := actorID(ctx)
	f, err := s.store.GetFloor(ctx, uid, idParam(r, "id"))
	if err != nil {
		s.fail(w, err, "floor")
		return
	}
	var in struct {
		Name     string           `json:"name"`
		CopyFrom int64            `json:"copy_from"`
		Shell    bool             `json:"shell"` // copy only rooms, walls, doors and windows
		Data     *json.RawMessage `json:"data"`
	}
	if err := decode(r, &in); err != nil {
		writeErr(w, http.StatusBadRequest, "invalid json")
		return
	}
	layouts, _ := s.store.ListLayouts(ctx, uid, f.PlanID, false)
	n := 0
	for _, l := range layouts {
		if l.FloorID == f.ID {
			n++
		}
	}
	d := store.Drawing{}
	d.Normalize()
	if in.CopyFrom > 0 {
		src, err := s.store.GetLayout(ctx, uid, in.CopyFrom)
		if err != nil || src.FloorID != f.ID {
			writeErr(w, http.StatusNotFound, "not found")
			return
		}
		d, _ = store.ParseDrawing(src.Data)
		if in.Shell {
			d.Items, d.Labels = []store.DItem{}, []store.DLabel{}
		}
	} else if in.Data != nil {
		if d, err = store.ParseDrawing(*in.Data); err != nil {
			writeErr(w, http.StatusBadRequest, "the drawing is not valid")
			return
		}
	}
	raw, _ := json.Marshal(d)
	name := firstNonEmpty(cleanText(in.Name, maxName), fmt.Sprintf("Option %c", 'A'+rune(n%26)))
	l := &store.Layout{UserID: uid, PlanID: f.PlanID, FloorID: f.ID, Name: name, Data: raw, Sort: n}
	if err := s.store.CreateLayout(ctx, l); err != nil {
		s.fail(w, err, "layout")
		return
	}
	s.store.TouchPlan(ctx, uid, f.PlanID)
	writeJSON(w, http.StatusCreated, l)
}

// handleUpdateLayout saves a layout; with "revision" a stale save is refused (409).
func (s *Server) handleUpdateLayout(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	uid := actorID(ctx)
	l, err := s.store.GetLayout(ctx, uid, idParam(r, "id"))
	if err != nil {
		s.fail(w, err, "layout")
		return
	}
	var in struct {
		Name     *string          `json:"name"`
		Notes    *string          `json:"notes"`
		Data     *json.RawMessage `json:"data"`
		Sort     *int             `json:"sort"`
		Revision int              `json:"revision"`
	}
	if err := decode(r, &in); err != nil {
		writeErr(w, http.StatusBadRequest, "invalid json")
		return
	}
	if in.Name != nil {
		l.Name = firstNonEmpty(cleanText(*in.Name, maxName), l.Name)
	}
	if in.Notes != nil {
		l.Notes = cleanText(*in.Notes, maxNotes)
	}
	if in.Sort != nil {
		l.Sort = *in.Sort
	}
	if in.Data != nil {
		d, err := store.ParseDrawing(*in.Data)
		if err != nil {
			writeErr(w, http.StatusBadRequest, "the drawing is not valid")
			return
		}
		if err := validDrawing(d); err != nil {
			s.userFail(w, err, "layout")
			return
		}
		raw, _ := json.Marshal(d)
		l.Data = raw
	}
	if err := s.store.UpdateLayout(ctx, l, in.Revision); err != nil {
		if err == store.ErrStale {
			writeErr(w, http.StatusConflict, "this layout was changed in another window; reload to see the newest version")
			return
		}
		s.fail(w, err, "save layout")
		return
	}
	s.store.TouchPlan(ctx, uid, l.PlanID)
	if in.Data != nil && s.once(ctx, fmt.Sprintf("evt:layout:%d:%s", l.ID, l.UpdatedAt[:min(len(l.UpdatedAt), 15)])) {
		s.publish(ctx, uid, "rooms.layout_saved", fmt.Sprintf("layout:%d", l.ID), map[string]any{"plan_id": l.PlanID, "layout_id": l.ID, "name": l.Name,
			"rooms": l.RoomsN, "items": l.ItemsN, "area_m2": l.AreaCm2 / 10000, "url": s.link(fmt.Sprintf("/plans/%d?layout=%d", l.PlanID, l.ID))})
	}
	l.Data = nil
	writeJSON(w, http.StatusOK, l)
}

// validDrawing keeps a drawing within sane bounds.
func validDrawing(d store.Drawing) error {
	if len(d.Rooms) > 400 || len(d.Items) > 3000 || len(d.Openings) > 2000 || len(d.Walls) > 2000 || len(d.Dims) > 1000 || len(d.Labels) > 1000 {
		return &userErr{"this layout has too many things on it"}
	}
	for _, r := range d.Rooms {
		if len(r.Points) > 200 {
			return &userErr{"a room has too many corners"}
		}
		for _, p := range r.Points {
			if p.X < -1e6 || p.X > 1e6 || p.Y < -1e6 || p.Y > 1e6 {
				return &userErr{"a room is too far away"}
			}
		}
	}
	for _, it := range d.Items {
		if it.W <= 0 || it.D <= 0 || it.W > 5000 || it.D > 5000 {
			return &userErr{"every piece needs a width and a depth"}
		}
	}
	return nil
}

func (s *Server) handleDeleteLayout(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	uid := actorID(ctx)
	l, err := s.store.GetLayout(ctx, uid, idParam(r, "id"))
	if err != nil {
		s.fail(w, err, "layout")
		return
	}
	layouts, _ := s.store.ListLayouts(ctx, uid, l.PlanID, false)
	n := 0
	for _, x := range layouts {
		if x.FloorID == l.FloorID {
			n++
		}
	}
	if n <= 1 {
		writeErr(w, http.StatusBadRequest, "a floor keeps at least one layout")
		return
	}
	if err := s.store.DeleteLayout(ctx, uid, l.ID); err != nil {
		s.fail(w, err, "delete layout")
		return
	}
	if f, err := s.store.GetFloor(ctx, uid, l.FloorID); err == nil && f.CurrentLayoutID != nil && *f.CurrentLayoutID == l.ID {
		f.CurrentLayoutID = nil
		_ = s.store.UpdateFloor(ctx, f)
	}
	s.writePlan(w, r, uid, l.PlanID, http.StatusOK)
}
