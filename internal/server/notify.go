package server

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/abjelosevic88/pocket-homelab-rooms/internal/notify"
	"github.com/abjelosevic88/pocket-homelab-rooms/internal/store"
)

// pushes runs every minute; at each person's hour it sends one message with
// the checklist items due tomorrow (once a day).
func (s *Server) pushes(ctx context.Context) {
	now := time.Now()
	if now.Minute() > 10 {
		return
	}
	ids, err := s.store.SettingsUsers(ctx)
	if err != nil {
		return
	}
	tomorrow := store.AddDays(today(), 1)
	for _, uid := range ids {
		st, err := s.store.GetSettings(ctx, uid)
		if err != nil || st.NotifyURL == "" || !st.NotifyTodos || now.Hour() != st.NotifyHour {
			continue
		}
		list, err := s.store.ListTodos(ctx, uid, store.TodoFilter{Open: true, DueFrom: tomorrow, DueTo: tomorrow})
		if err != nil || len(list) == 0 || !s.once(ctx, fmt.Sprintf("push:%d:%s", uid, today())) {
			continue
		}
		lines := []string{}
		for _, t := range list {
			lines = append(lines, "• "+t.Title+" ("+t.PlanName+")")
		}
		title := fmt.Sprintf("Tomorrow: %d %s on your plans", len(list), plural(len(list), "thing", "things"))
		s.push(ctx, uid, st, title, strings.Join(lines, "\n"), s.link(fmt.Sprintf("/plans/%d?tab=checklist", list[0].PlanID)))
	}
}

// once is true the first time it is asked for a key.
func (s *Server) once(ctx context.Context, key string) bool {
	if _, _, done := s.store.CacheGet(ctx, key); done {
		return false
	}
	_ = s.store.CachePut(ctx, key, "1")
	return true
}

// push sends one message to a person's own notification address.
func (s *Server) push(ctx context.Context, uid int64, st store.Settings, title, body, link string) bool {
	if st.NotifyURL == "" {
		return false
	}
	n := notify.New(st.NotifyURL, st.NotifyFormat)
	if err := n.Send(ctx, notify.Message{Title: title, Body: body, Priority: "info", Tags: []string{"triangular_ruler"}, Link: link}); err != nil {
		s.log.Warn("notify failed", "user", uid, "err", err)
		return false
	}
	return true
}

func (s *Server) handleNotifyTest(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	st, _ := s.store.GetSettings(ctx, actorID(ctx))
	n := notify.New(st.NotifyURL, st.NotifyFormat)
	if err := n.Send(ctx, notify.Message{Title: "Pocket Rooms", Body: notifyTestBody, Priority: "info", Tags: []string{"triangular_ruler"}}); err != nil {
		writeJSON(w, http.StatusOK, map[string]any{"ok": false, "error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

const notifyTestBody = "Notifications work. The day before a checklist item on one of your plans is due, you get one message at your hour."
