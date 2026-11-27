package server

import (
	"database/sql"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	core "github.com/abjelosevic88/pocket-homelab-core/sdk"
	"github.com/abjelosevic88/pocket-homelab-rooms/internal/store"
)

// What Pocket Search, Pocket Day and Pocket Hub ask Rooms (the capabilities
// value in links.go): one route each, in the shape every Pocket app answers
// in. They read what /search, /day and the first page read, as the request's
// person; only the capture adds something, one piece of furniture to buy.

// The words the pages use.
var (
	planKindWords   = map[string]string{"apartment": "Flat", "house": "House", "studio": "Studio", "office": "Office", "garden": "Garden"}
	planStateWords  = map[string]string{"living": "Home", "moving": "Moving in", "idea": "Idea", "done": "Done"}
	pieceStateWords = map[string]string{"want": "To buy", "maybe": "Maybe", "gone": "Gone"}
)

// checklistURL opens a plan's checklist.
func checklistURL(planID int64) string { return fmt.Sprintf("/plans/%d?tab=checklist", planID) }

// ---------- search ----------

// hasWord says whether a search has a letter or a digit in it.
func hasWord(q string) bool {
	return strings.IndexFunc(q, func(r rune) bool { return unicode.IsLetter(r) || unicode.IsDigit(r) }) >= 0
}

// around is the part of a longer text the words were found in, on one line.
func around(text, lq string) string {
	text = strings.Join(strings.Fields(text), " ")
	at := strings.Index(strings.ToLower(text), lq)
	if at < 0 {
		return ""
	}
	rs := []rune(text)
	// lower-casing keeps the number of letters, so the place is the same
	from := utf8.RuneCountInString(strings.ToLower(text)[:at]) - 40
	if from <= 0 || from >= len(rs) {
		return clip(text, 200)
	}
	return "…" + clip(strings.TrimSpace(string(rs[from:])), 200)
}

// pocketSearch answers Pocket Search with what /search finds: plans, the
// rooms on their layouts and furniture, the ones whose name has the words
// first. A plan or a piece found by its notes shows the line they are in.
func (s *Server) pocketSearch(r *http.Request, q string, limit int) ([]core.SearchItem, error) {
	if !hasWord(q) {
		return nil, nil
	}
	ctx := r.Context()
	rows, err := s.find(ctx, actorID(ctx), q, limit)
	if err != nil {
		return nil, err
	}
	lq := strings.ToLower(q)
	out := make([]core.SearchItem, 0, len(rows))
	for _, f := range rows {
		it := core.SearchItem{ID: f.key, Title: f.title, Snippet: f.about, Kind: f.kind, Date: f.changed, URL: f.path, Score: 1}
		switch {
		case strings.EqualFold(strings.TrimSpace(f.title), q):
			it.Score = 3
		case strings.Contains(strings.ToLower(f.title), lq):
			it.Score = 2
		case !strings.Contains(strings.ToLower(f.about), lq):
			it.Snippet = joinNonEmpty(" · ", f.about, around(f.more, lq))
		}
		out = append(out, it)
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].Score > out[j].Score })
	if len(out) > limit {
		out = out[:limit]
	}
	return out, nil
}

// ---------- day ----------

// pocketDay answers Pocket Day from the checklists, as /day does: what was
// ticked off that day (with the time) and what is due that day and still
// open. Something due that day and done on another one shows on the day it
// was done. The day is the caller's (?tz=), else the box's own.
func (s *Server) pocketDay(r *http.Request, date string, tz *time.Location) (*core.Day, error) {
	ctx := r.Context()
	loc := time.Local
	if tz != nil {
		loc = tz
	}
	todos, err := s.store.ListTodos(ctx, actorID(ctx), store.TodoFilter{})
	if err != nil {
		return nil, err
	}
	var done, due []core.DayItem
	for _, t := range todos {
		it := core.DayItem{ID: itoa(t.ID), Title: t.Title, Sub: t.PlanName, URL: checklistURL(t.PlanID)}
		switch {
		case t.Done:
			at, err := time.Parse(time.RFC3339, t.DoneAt)
			switch {
			case err == nil && at.In(loc).Format("2006-01-02") == date:
				it.Time = at.In(loc).Format("15:04")
			case err == nil || !strings.HasPrefix(t.DoneAt, date):
				continue
			}
			it.Kind, it.Done = "done", true
			done = append(done, it)
		case t.Due == date:
			it.Kind = "due"
			due = append(due, it)
		}
	}
	if len(done)+len(due) == 0 {
		return nil, nil
	}
	sort.SliceStable(done, func(i, j int) bool { return done[i].Time < done[j].Time })
	day := &core.Day{Date: date, Items: append(done, due...)}
	items := func(n int) string { return fmt.Sprintf("%d checklist %s", n, plural(n, "item", "items")) }
	if n := len(done); n > 0 {
		day.Summary = items(n) + " done"
		day.Metrics = append(day.Metrics, core.Metric{Label: "Done", Value: strconv.Itoa(n)})
	}
	if n := len(due); n > 0 {
		// on a day still to come nothing is "still" due
		word := "still due"
		if date > time.Now().In(loc).Format("2006-01-02") {
			word = "due"
		}
		if day.Summary == "" {
			day.Summary = items(n) + " " + word
		} else {
			day.Summary += fmt.Sprintf(", %d %s", n, word)
		}
		day.Metrics = append(day.Metrics, core.Metric{Label: "Due", Value: strconv.Itoa(n)})
	}
	return day, nil
}

// ---------- widget ----------

// dueWords says when a checklist item is due: "due today", "due tomorrow",
// "due Mon 30 Nov", "was due Mon 2 Nov".
func dueWords(due, today string) string {
	d, err := store.ParseDate(due)
	if err != nil {
		return ""
	}
	day := d.Format("Mon 2 Jan")
	if !strings.HasPrefix(today, due[:4]) {
		day = d.Format("2 Jan 2006")
	}
	switch {
	case due < today:
		return "was due " + day
	case due == today:
		return "due today"
	case due == store.AddDays(today, 1):
		return "due tomorrow"
	}
	return "due " + day
}

// pocketWidget answers Pocket Hub: what is left to do on the checklists of
// the person's plans, what is due first on top (late ones before the rest),
// then what has no day. With nothing to do it shows the plans changed last;
// someone without a plan gets nothing.
func (s *Server) pocketWidget(r *http.Request) (*core.WidgetData, error) {
	ctx := r.Context()
	uid := actorID(ctx)
	now := today()
	open, plans, late, err := s.store.OpenTodoCounts(ctx, uid, now)
	if err != nil {
		return nil, err
	}
	w := &core.WidgetData{URL: "/"}
	if open > 0 {
		todos, err := s.store.ListTodos(ctx, uid, store.TodoFilter{Open: true, Limit: 8})
		if err != nil {
			return nil, err
		}
		for _, t := range todos {
			w.Items = append(w.Items, core.WidgetItem{ID: "todo:" + itoa(t.ID), Title: t.Title, Detail: joinNonEmpty(" · ", t.PlanName, dueWords(t.Due, now)),
				URL: checklistURL(t.PlanID)})
		}
		w.Line = fmt.Sprintf("%d %s to do", open, plural(open, "thing", "things"))
		if plans == 1 && len(todos) > 0 {
			w.Line += " for " + todos[0].PlanName
			w.URL = checklistURL(todos[0].PlanID)
		} else {
			w.Line += fmt.Sprintf(" on %d plans", plans)
		}
		if late > 0 {
			w.Line += fmt.Sprintf(", %d late", late)
		}
		w.Metrics = []core.Metric{{Label: "To do", Value: strconv.Itoa(open)}, {Label: "Late", Value: strconv.Itoa(late)}}
		return w, nil
	}
	recent, err := s.store.RecentPlans(ctx, uid, 5)
	if err != nil || len(recent) == 0 {
		return nil, err
	}
	for _, p := range recent {
		w.Items = append(w.Items, core.WidgetItem{ID: "plan:" + itoa(p.ID), Title: p.Name, Detail: joinNonEmpty(" · ", planStateWords[p.Status], p.Address),
			URL: fmt.Sprintf("/plans/%d", p.ID)})
	}
	return w, nil
}

// ---------- capture ----------

var reLink = regexp.MustCompile(`(?i)\bhttps?://[^\s<>"]+`)

// linkIn finds the first web address in a line and returns it with the line
// around it: "Kivik sofa - https://shop.example/kivik" is "Kivik sofa" and
// the address.
func linkIn(line string) (rest, link string) {
	m := reLink.FindString(line)
	if m == "" {
		return line, ""
	}
	link, ok := cleanURL(strings.TrimRight(m, `.,;:!?)]}'`))
	if !ok || link == "" {
		return line, ""
	}
	rest = strings.Join(strings.Fields(strings.Replace(line, m, " ", 1)), " ")
	return strings.TrimLeft(strings.TrimRight(rest, " -–—:,;|(["), " -–—:,;|)]"), link
}

// sizeLater is written in the notes of a piece kept from a quick line: it
// starts with the size every new piece starts with, not its own.
const sizeLater = "Added from a quick line. Put in its real size before you check whether it fits."

// pocketCapture keeps a quick line from Pocket Hub as a piece of furniture
// to buy, the way POST /pieces adds one for another app: status "want", the
// starting size until the person measures it. The line is the name (the
// title when one was given, else the first line of the text); a web address
// in it becomes the piece's shop link, and a line that is only an address is
// named after the shop. The rest of the text goes to the notes. The same
// request_id again answers the same piece and adds nothing: the receipt is
// written in the transaction that adds the piece.
func (s *Server) pocketCapture(r *http.Request, in core.CaptureInput) (*core.CaptureResult, error) {
	ctx := r.Context()
	uid := actorID(ctx)
	op, err := core.NewOperation(fmt.Sprintf("rooms:user:%d:%s", uid, core.CapturePath), in.RequestID, in)
	if err != nil {
		return nil, err
	}
	var res core.CaptureResult
	replay := func() (*core.CaptureResult, error) {
		found, err := op.Replay(ctx, s.store.DB, &res)
		switch {
		case errors.Is(err, core.ErrOperationConflict):
			return nil, fmt.Errorf("%w: this was already kept with other words", core.ErrCapture)
		case err != nil || !found:
			return nil, err
		}
		return &res, nil
	}
	if kept, err := replay(); kept != nil || err != nil {
		return kept, err
	}

	line, notes := in.Title, in.Text
	if line == "" {
		line, notes, _ = strings.Cut(in.Text, "\n")
	}
	line, notes = strings.Join(strings.Fields(line), " "), strings.TrimSpace(notes)
	name, link := linkIn(line)
	if link == "" {
		_, link = linkIn(notes)
	}
	if name == "" && link != "" {
		// only an address: the shop's name stands in until the person names it
		if u, err := url.Parse(link); err == nil {
			name = strings.TrimPrefix(strings.ToLower(u.Hostname()), "www.")
		}
	}
	if !hasWord(name) {
		return nil, fmt.Errorf("%w: write what the piece is, like \"Grey sofa\"", core.ErrCapture)
	}
	if utf8.RuneCountInString(name) > maxName { // a long line: the start is the name, all of it is in the notes
		notes = joinNonEmpty("\n", line, notes)
	}
	notes = joinNonEmpty("\n\n", notes, sizeLater)
	status := "want"
	_, err = s.addPiece(ctx, uid, map[string]bool{"name": true, "status": true, "url": true, "notes": true},
		pieceIn{Name: &name, Status: &status, URL: &link, Notes: &notes}, func(tx *sql.Tx, p *store.Piece) error {
			res = core.CaptureResult{ID: "piece:" + itoa(p.ID), Title: p.Name, URL: fmt.Sprintf("/furniture/%d", p.ID)}
			return op.Save(ctx, tx, res)
		})
	if err != nil {
		// the same line sent twice at once: the other one kept it
		if kept, rerr := replay(); kept != nil || rerr != nil {
			return kept, rerr
		}
		var ue *userErr
		if errors.As(err, &ue) {
			return nil, fmt.Errorf("%w: %s", core.ErrCapture, ue.msg)
		}
		return nil, err
	}
	return &res, nil
}
