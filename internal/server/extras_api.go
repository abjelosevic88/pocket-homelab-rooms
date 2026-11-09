package server

import (
	"fmt"
	"net/http"
	"strings"

	"github.com/abjelosevic88/pocket-homelab-rooms/internal/store"
)

// ---- the way in ----

func (s *Server) handleListRoutes(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	uid := actorID(ctx)
	if _, err := s.store.GetPlan(ctx, uid, idParam(r, "id")); err != nil {
		s.fail(w, err, "plan")
		return
	}
	list, err := s.store.ListRoutes(ctx, uid, idParam(r, "id"))
	if err != nil {
		s.fail(w, err, "routes")
		return
	}
	writeJSON(w, http.StatusOK, list)
}

type routeIn struct {
	Name  *string            `json:"name"`
	Steps *[]store.RouteStep `json:"steps"`
	Sort  *int               `json:"sort"`
}

func (in routeIn) apply(rt *store.Route) error {
	if in.Name != nil {
		rt.Name = firstNonEmpty(cleanText(*in.Name, maxName), "The way in")
	}
	if in.Steps != nil {
		steps, err := cleanSteps(*in.Steps)
		if err != nil {
			return err
		}
		rt.Steps = steps
	}
	if in.Sort != nil {
		rt.Sort = *in.Sort
	}
	return nil
}

func (s *Server) handleCreateRoute(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	uid := actorID(ctx)
	p, err := s.store.GetPlan(ctx, uid, idParam(r, "id"))
	if err != nil {
		s.fail(w, err, "plan")
		return
	}
	var in routeIn
	if err := decode(r, &in); err != nil {
		writeErr(w, http.StatusBadRequest, "invalid json")
		return
	}
	rt := &store.Route{UserID: uid, PlanID: p.ID, Name: "The way in", Steps: []store.RouteStep{}}
	if err := in.apply(rt); err != nil {
		s.userFail(w, err, "route")
		return
	}
	if err := s.store.SaveRoute(ctx, rt); err != nil {
		s.fail(w, err, "route")
		return
	}
	writeJSON(w, http.StatusCreated, rt)
}

func (s *Server) handleUpdateRoute(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	uid := actorID(ctx)
	rt, err := s.store.GetRoute(ctx, uid, idParam(r, "id"))
	if err != nil {
		s.fail(w, err, "route")
		return
	}
	var in routeIn
	if err := decode(r, &in); err != nil {
		writeErr(w, http.StatusBadRequest, "invalid json")
		return
	}
	if err := in.apply(rt); err != nil {
		s.userFail(w, err, "route")
		return
	}
	if err := s.store.SaveRoute(ctx, rt); err != nil {
		s.fail(w, err, "route")
		return
	}
	writeJSON(w, http.StatusOK, rt)
}

func (s *Server) handleDeleteRoute(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	if err := s.store.DeleteRoute(ctx, actorID(ctx), idParam(r, "id")); err != nil {
		s.fail(w, err, "route")
		return
	}
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

// ---- the checklist ----

func (s *Server) handleListTodos(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	uid := actorID(ctx)
	list, err := s.store.ListTodos(ctx, uid, store.TodoFilter{PlanID: int64(queryInt(r, "plan_id")), Open: queryBool(r, "open")})
	if err != nil {
		s.fail(w, err, "checklist")
		return
	}
	writeJSON(w, http.StatusOK, list)
}

type todoIn struct {
	Title  *string `json:"title"`
	Done   *bool   `json:"done"`
	Due    *string `json:"due"`
	Sort   *int    `json:"sort"`
	PlanID int64   `json:"plan_id"`
}

func (in todoIn) apply(t *store.Todo) error {
	if in.Title != nil {
		t.Title = cleanText(*in.Title, maxLine)
	}
	if in.Due != nil {
		if !validDate(*in.Due) {
			return &userErr{"a date is YYYY-MM-DD"}
		}
		t.Due = *in.Due
	}
	if in.Done != nil && *in.Done != t.Done {
		t.Done = *in.Done
		t.DoneAt = ""
		if t.Done {
			t.DoneAt = store.Now()
		}
	}
	if in.Sort != nil {
		t.Sort = *in.Sort
	}
	if strings.TrimSpace(t.Title) == "" {
		return &userErr{"write what needs doing"}
	}
	return nil
}

func (s *Server) handleCreateTodo(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	uid := actorID(ctx)
	var in todoIn
	if err := decode(r, &in); err != nil {
		writeErr(w, http.StatusBadRequest, "invalid json")
		return
	}
	pid := idParam(r, "id")
	if pid == 0 {
		pid = in.PlanID
	}
	p, err := s.store.GetPlan(ctx, uid, pid)
	if err != nil {
		s.fail(w, err, "plan")
		return
	}
	t := &store.Todo{UserID: uid, PlanID: p.ID, Source: sourceOf(ctx)}
	if err := in.apply(t); err != nil {
		s.userFail(w, err, "checklist")
		return
	}
	if err := s.store.SaveTodo(ctx, t); err != nil {
		s.fail(w, err, "checklist")
		return
	}
	t.PlanName = p.Name
	writeJSON(w, http.StatusCreated, t)
}

func (s *Server) handleUpdateTodo(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	uid := actorID(ctx)
	t, err := s.store.GetTodo(ctx, uid, idParam(r, "id"))
	if err != nil {
		s.fail(w, err, "checklist")
		return
	}
	if !mayChange(ctx, t.Source) {
		writeErr(w, http.StatusForbidden, "this app may change only the checklist items it added")
		return
	}
	var in todoIn
	if err := decode(r, &in); err != nil {
		writeErr(w, http.StatusBadRequest, "invalid json")
		return
	}
	wasDone := t.Done
	if err := in.apply(t); err != nil {
		s.userFail(w, err, "checklist")
		return
	}
	if err := s.store.SaveTodo(ctx, t); err != nil {
		s.fail(w, err, "checklist")
		return
	}
	if t.Done && !wasDone {
		s.publish(ctx, uid, "rooms.todo_done", fmt.Sprintf("todo:%d", t.ID), map[string]any{"todo_id": t.ID, "plan_id": t.PlanID, "title": t.Title, "plan": t.PlanName})
	}
	writeJSON(w, http.StatusOK, t)
}

func (s *Server) handleDeleteTodo(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	uid := actorID(ctx)
	t, err := s.store.GetTodo(ctx, uid, idParam(r, "id"))
	if err != nil {
		s.fail(w, err, "checklist")
		return
	}
	if !mayChange(ctx, t.Source) {
		writeErr(w, http.StatusForbidden, "this app may remove only the checklist items it added")
		return
	}
	if err := s.store.DeleteTodo(ctx, uid, t.ID); err != nil {
		s.fail(w, err, "checklist")
		return
	}
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

// ---- room photos ----

func (s *Server) decoratePhotos(list []store.PlanPhoto) {
	for i := range list {
		p := &list[i]
		if p.PhotoID != nil {
			p.URL, p.ThumbURL = photoURL(p.PhotoID)
		} else if p.PocketID != "" {
			p.URL = "/api/v1/apps/photos/" + p.PocketID + "/image?size=preview"
			p.ThumbURL = "/api/v1/apps/photos/" + p.PocketID + "/image"
		}
	}
}

func (s *Server) handleListPlanPhotos(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	uid := actorID(ctx)
	if _, err := s.store.GetPlan(ctx, uid, idParam(r, "id")); err != nil {
		s.fail(w, err, "plan")
		return
	}
	list, err := s.store.ListPlanPhotos(ctx, uid, idParam(r, "id"))
	if err != nil {
		s.fail(w, err, "photos")
		return
	}
	s.decoratePhotos(list)
	writeJSON(w, http.StatusOK, list)
}

func (s *Server) handleAddPlanPhoto(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	uid := actorID(ctx)
	p, err := s.store.GetPlan(ctx, uid, idParam(r, "id"))
	if err != nil {
		s.fail(w, err, "plan")
		return
	}
	var in struct {
		PhotoID  int64  `json:"photo_id"`
		PocketID string `json:"pocket_id"`
		Room     string `json:"room"`
		Caption  string `json:"caption"`
		TakenOn  string `json:"taken_on"`
	}
	if err := decode(r, &in); err != nil {
		writeErr(w, http.StatusBadRequest, "invalid json")
		return
	}
	pp := &store.PlanPhoto{UserID: uid, PlanID: p.ID, Room: cleanText(in.Room, maxName), Caption: cleanText(in.Caption, maxLine), TakenOn: in.TakenOn}
	switch {
	case in.PhotoID > 0:
		if !s.store.OwnsPhoto(ctx, uid, in.PhotoID) {
			writeErr(w, http.StatusNotFound, "not found")
			return
		}
		pp.PhotoID = &in.PhotoID
	case reUUID.MatchString(in.PocketID):
		pp.PocketID = in.PocketID
	default:
		writeErr(w, http.StatusBadRequest, "send a picture or pick one from Pocket Photos")
		return
	}
	if !validDate(pp.TakenOn) {
		pp.TakenOn = ""
	}
	if pp.TakenOn == "" {
		pp.TakenOn = today()
	}
	if err := s.store.SavePlanPhoto(ctx, pp); err != nil {
		s.fail(w, err, "photo")
		return
	}
	list := []store.PlanPhoto{*pp}
	s.decoratePhotos(list)
	writeJSON(w, http.StatusCreated, list[0])
}

func (s *Server) handleUpdatePlanPhoto(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	uid := actorID(ctx)
	pp, err := s.store.GetPlanPhoto(ctx, uid, idParam(r, "id"))
	if err != nil {
		s.fail(w, err, "photo")
		return
	}
	var in struct {
		Room    *string `json:"room"`
		Caption *string `json:"caption"`
	}
	if err := decode(r, &in); err != nil {
		writeErr(w, http.StatusBadRequest, "invalid json")
		return
	}
	if in.Room != nil {
		pp.Room = cleanText(*in.Room, maxName)
	}
	if in.Caption != nil {
		pp.Caption = cleanText(*in.Caption, maxLine)
	}
	if err := s.store.SavePlanPhoto(ctx, pp); err != nil {
		s.fail(w, err, "photo")
		return
	}
	list := []store.PlanPhoto{*pp}
	s.decoratePhotos(list)
	writeJSON(w, http.StatusOK, list[0])
}

func (s *Server) handleDeletePlanPhoto(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	if err := s.store.DeletePlanPhoto(ctx, actorID(ctx), idParam(r, "id")); err != nil {
		s.fail(w, err, "photo")
		return
	}
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}
