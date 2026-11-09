package server

import (
	"net/http"

	"github.com/abjelosevic88/pocket-homelab-rooms/internal/store"
)

// handlePublicPlan is the read-only view behind a plan's share link: the
// floors and their layouts, the areas — never the notes, prices or checklist.
func (s *Server) handlePublicPlan(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	p, err := s.store.PlanByShare(ctx, chiParam(r, "token"))
	if err != nil {
		writeErr(w, http.StatusNotFound, "this link is not shared (any more)")
		return
	}
	floors, _ := s.store.ListFloors(ctx, p.UserID, p.ID)
	layouts, _ := s.store.ListLayouts(ctx, p.UserID, p.ID, true)
	st := s.settingsOf(ctx, p.UserID)
	outFloors := []map[string]any{}
	for _, f := range floors {
		cur := currentOf(f, layouts)
		ls := []map[string]any{}
		for _, l := range layouts {
			if l.FloorID != f.ID {
				continue
			}
			d, _ := store.ParseDrawing(l.Data)
			for i := range d.Items {
				d.Items[i].Price, d.Items[i].Note, d.Items[i].PieceID, d.Items[i].From = 0, "", 0, ""
			}
			ls = append(ls, map[string]any{"id": l.ID, "name": l.Name, "notes": l.Notes, "data": d, "area_cm2": l.AreaCm2, "rooms_n": l.RoomsN, "items_n": l.ItemsN,
				"current": cur != nil && cur.ID == l.ID})
		}
		outFloors = append(outFloors, map[string]any{"id": f.ID, "name": f.Name, "level": f.Level, "ceiling_cm": f.CeilingCm, "layouts": ls})
	}
	writeJSON(w, http.StatusOK, map[string]any{"name": p.Name, "kind": p.Kind, "units": st.Units, "floors": outFloors})
}
