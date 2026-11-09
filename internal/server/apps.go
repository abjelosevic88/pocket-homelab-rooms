package server

import (
	"context"
	"fmt"
	"math"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/abjelosevic88/pocket-homelab-rooms/internal/store"
)

// The other Pocket apps. None of it is needed: when an app is not there,
// the page greys the button and says what to do.

func linkTo(sib *siblingConn, path string) string {
	if sib == nil || sib.public == "" {
		return ""
	}
	return strings.TrimRight(sib.public, "/") + path
}

// plainErr is an app's error in a line.
func plainErr(a pocketApp, err error) string {
	if isNotConnected(err) {
		return err.Error()
	}
	return a.name + ": " + err.Error()
}

func (s *Server) appFail(w http.ResponseWriter, a pocketApp, err error) {
	if isNotConnected(err) {
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}
	writeErr(w, http.StatusBadGateway, plainErr(a, err))
}

// handleApps lists how the person reaches each app, in words they can act on.
func (s *Server) handleApps(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	c := s.clients(ctx, actorID(ctx))
	out := []appState{}
	for _, a := range pocketApps {
		out = append(out, c.status(ctx, a))
	}
	writeJSON(w, http.StatusOK, map[string]any{"apps": out, "core": s.core.Configured()})
}

// ---- Pocket Home: homes, appliances, house plants ----

func (s *Server) handleHomes(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	sib, err := s.clients(ctx, actorID(ctx)).conn(ctx, appHome, scopeHomeRead)
	if err != nil {
		s.appFail(w, appHome, err)
		return
	}
	v, err := getJSONFrom(ctx, sib, "/api/v1/homes", nil)
	if err != nil {
		s.appFail(w, appHome, err)
		return
	}
	out := []map[string]any{}
	for _, m := range rows(v, "homes") {
		if m["archived"] == true {
			continue
		}
		out = append(out, map[string]any{"id": num(m, "id"), "name": str(m, "name"), "kind": str(m, "kind"), "address": str(m, "address"),
			"area_m2": num(m, "area_m2"), "currency": str(m, "currency"), "url": linkTo(sib, "/homes/"+idOf(m))})
	}
	writeJSON(w, http.StatusOK, map[string]any{"homes": out})
}

// usual sizes (w, d, h in cm) for what Pocket Home keeps, so they can go on the plan
var homeSizes = []struct {
	words   []string
	typ     string
	w, d, h float64
}{
	{[]string{"fridge", "refrigerator", "freezer"}, "fridge", 60, 65, 185},
	{[]string{"dishwasher"}, "dishwasher", 60, 58, 85},
	{[]string{"washing machine", "washer", "washing"}, "washer", 60, 60, 85},
	{[]string{"dryer", "tumble"}, "washer", 60, 60, 85},
	{[]string{"oven", "stove", "cooker", "hob"}, "stove", 60, 60, 90},
	{[]string{"microwave"}, "box", 50, 40, 30},
	{[]string{"boiler", "heater", "water heater"}, "box", 45, 35, 75},
	{[]string{"radiator"}, "radiator", 100, 10, 60},
	{[]string{"air con", "air-con", "aircon", "ac unit", "heat pump"}, "box", 90, 25, 30},
	{[]string{"tv", "television"}, "tv", 145, 10, 85},
	{[]string{"sofa", "couch"}, "sofa", 210, 90, 85},
	{[]string{"bed"}, "bed_double", 160, 205, 50},
	{[]string{"desk"}, "desk", 140, 70, 75},
	{[]string{"wardrobe", "closet"}, "wardrobe", 150, 60, 200},
	{[]string{"vacuum", "robot"}, "box", 35, 35, 10},
	{[]string{"router", "modem"}, "box", 25, 15, 5},
}

func guessSize(name, category string) (string, float64, float64, float64) {
	n := strings.ToLower(name)
	for _, hs := range homeSizes {
		for _, w := range hs.words {
			if strings.Contains(n, w) {
				return hs.typ, hs.w, hs.d, hs.h
			}
		}
	}
	switch category {
	case "kitchen", "appliance":
		return "box", 60, 60, 85
	case "heating", "cooling":
		return "box", 50, 30, 70
	}
	return "box", 50, 50, 50
}

// handleHomeThings lists a home's appliances and house plants with the
// room Pocket Home has for each and a usual size, ready to place.
func (s *Server) handleHomeThings(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	sib, err := s.clients(ctx, actorID(ctx)).conn(ctx, appHome, scopeHomeRead)
	if err != nil {
		s.appFail(w, appHome, err)
		return
	}
	q := url.Values{}
	if id := r.URL.Query().Get("home_id"); id != "" {
		q.Set("home_id", id)
	}
	out := []map[string]any{}
	if v, err := getJSONFrom(ctx, sib, "/api/v1/things", q); err == nil {
		for _, m := range rows(v, "things") {
			if m["archived"] == true || str(m, "category") == "structure" || str(m, "category") == "outdoor" {
				continue
			}
			typ, wd, dp, ht := guessSize(str(m, "name"), str(m, "category"))
			out = append(out, map[string]any{"from": "home:thing:" + idOf(m), "kind": "thing", "id": num(m, "id"), "name": str(m, "name"), "room": str(m, "room"),
				"category": str(m, "category"), "type": typ, "w": wd, "d": dp, "h": ht, "subtitle": joinNonEmpty(" ", str(m, "brand"), str(m, "model")),
				"url": linkTo(sib, "/things/"+idOf(m))})
		}
	} else {
		s.appFail(w, appHome, err)
		return
	}
	if v, err := getJSONFrom(ctx, sib, "/api/v1/plants", q); err == nil {
		for _, m := range rows(v, "plants") {
			if m["archived"] == true {
				continue
			}
			size := 40.0
			if k := str(m, "kind"); k == "palm" || k == "foliage" {
				size = 50
			} else if k == "succulent" || k == "cactus" || k == "herb" || k == "orchid" {
				size = 20
			}
			out = append(out, map[string]any{"from": "home:plant:" + idOf(m), "kind": "plant", "id": num(m, "id"), "name": str(m, "name"), "room": str(m, "room"),
				"category": "plant", "type": "plant", "w": size, "d": size, "h": size * 2, "subtitle": firstNonEmpty(str(m, "species"), str(m, "light")),
				"url": linkTo(sib, "/plants/"+idOf(m))})
		}
	}
	writeJSON(w, http.StatusOK, map[string]any{"things": out})
}

// ---- Pocket Inventory ----

func (s *Server) handleInventory(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	sib, err := s.clients(ctx, actorID(ctx)).conn(ctx, appInventory, scopeItemsRead)
	if err != nil {
		s.appFail(w, appInventory, err)
		return
	}
	q := url.Values{"limit": {"60"}}
	if t := strings.TrimSpace(r.URL.Query().Get("q")); t != "" {
		q.Set("q", t)
	}
	v, err := getJSONFrom(ctx, sib, "/api/v1/items", q)
	if err != nil {
		s.appFail(w, appInventory, err)
		return
	}
	out := []map[string]any{}
	for _, m := range rows(v, "items") {
		if m["archived"] == true {
			continue
		}
		price := int64(math.Round(num(m, "price") * 100)) // Inventory keeps whole units
		typ, wd, dp, ht := guessSize(str(m, "name"), strings.ToLower(str(m, "category_name")))
		out = append(out, map[string]any{"id": num(m, "id"), "name": str(m, "name"), "location": str(m, "location_name"), "category": str(m, "category_name"),
			"brand": str(m, "brand"), "price": price, "url": linkTo(sib, "/items/"+idOf(m)), "type": typ, "w": wd, "d": dp, "h": ht})
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": out})
}

// ---- Pocket Wishlist ----

func (s *Server) handleWishes(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	sib, err := s.clients(ctx, actorID(ctx)).conn(ctx, appWishlist, scopeWishesRead)
	if err != nil {
		s.appFail(w, appWishlist, err)
		return
	}
	q := url.Values{"status": {"wanted"}, "limit": {"80"}}
	if t := strings.TrimSpace(r.URL.Query().Get("q")); t != "" {
		q.Set("q", t)
	}
	v, err := getJSONFrom(ctx, sib, "/api/v1/wishes", q)
	if err != nil {
		s.appFail(w, appWishlist, err)
		return
	}
	out := []map[string]any{}
	for _, m := range rows(v, "wishes") {
		typ, wd, dp, ht := guessSize(str(m, "title"), "")
		out = append(out, map[string]any{"id": num(m, "id"), "name": str(m, "title"), "price": num(m, "price"), "currency": str(m, "currency"), "shop": str(m, "store"),
			"link": str(m, "url"), "image_url": str(m, "image_url"), "notes": str(m, "notes"), "url": linkTo(sib, "/wishes/"+idOf(m)), "type": typ, "w": wd, "d": dp, "h": ht})
	}
	writeJSON(w, http.StatusOK, map[string]any{"wishes": out})
}

// handlePieceWish puts a piece on the wishlist (and remembers the wish).
func (s *Server) handlePieceWish(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	uid := actorID(ctx)
	p, err := s.store.GetPiece(ctx, uid, idParam(r, "id"))
	if err != nil {
		s.fail(w, err, "piece")
		return
	}
	if p.WishID != nil {
		writeJSON(w, http.StatusOK, map[string]any{"ok": true, "url": s.siblingLink(ctx, uid, appWishlist, fmt.Sprintf("/wishes/%d", *p.WishID)), "text": "Already on your wishlist"})
		return
	}
	sib, err := s.clients(ctx, uid).conn(ctx, appWishlist, scopeWishesWrite)
	if err != nil {
		s.appFail(w, appWishlist, err)
		return
	}
	st := s.settingsOf(ctx, uid)
	notes := strings.TrimSpace(fmt.Sprintf("Size: %s (w × d × h).\n%s", sizeText(p.Width, p.Depth, p.Height, st.Units), p.Notes))
	if pl := s.placedIn(ctx, uid)[p.ID]; len(pl) > 0 {
		notes += "\nPlanned for: " + strings.Join(pl, ", ")
	}
	notes += "\n" + s.link(fmt.Sprintf("/furniture/%d", p.ID))
	body := map[string]any{"title": p.Name, "notes": notes, "store": p.Shop, "currency": p.Currency, "no_lookup": true, "tags": []string{"furniture", "rooms"}}
	if p.URL != "" {
		body["url"] = p.URL
	}
	if p.Price != nil {
		body["price"] = *p.Price
	}
	if st.WishlistID > 0 {
		body["list_id"] = st.WishlistID
	}
	res, err := postJSONTo(ctx, sib, "/api/v1/wishes", body)
	if err != nil {
		s.appFail(w, appWishlist, err)
		return
	}
	id := int64(num(res, "id"))
	if id > 0 {
		p.WishID, p.WishName = &id, p.Name
		if p.Status == "own" {
			p.Status = "want"
		}
		_ = s.store.UpdatePiece(ctx, p)
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "url": linkTo(sib, "/wishes/"+idOf(res)), "text": "Added to Pocket Wishlist", "piece": p})
}

// ---- Pocket Money ----

func (s *Server) handleMoneyAccounts(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	sib, err := s.clients(ctx, actorID(ctx)).conn(ctx, appMoney, scopeAccountsRead)
	if err != nil {
		s.appFail(w, appMoney, err)
		return
	}
	v, err := getJSONFrom(ctx, sib, "/api/v1/accounts", nil)
	if err != nil {
		s.appFail(w, appMoney, err)
		return
	}
	accts := []map[string]any{}
	for _, m := range rows(v, "accounts") {
		if m["archived"] == true || str(m, "role") == "viewer" {
			continue
		}
		accts = append(accts, map[string]any{"id": idOf(m), "name": str(m, "name"), "currency": str(m, "currency"), "everyday": m["everyday"] == true})
	}
	cats := []map[string]any{}
	if v, err := getJSONFrom(ctx, sib, "/api/v1/categories", nil); err == nil {
		for _, m := range rows(v, "categories") {
			if m["archived"] == true || str(m, "kind") == "income" {
				continue
			}
			cats = append(cats, map[string]any{"id": idOf(m), "name": firstNonEmpty(str(m, "full_name"), str(m, "name"))})
		}
	}
	writeJSON(w, http.StatusOK, map[string]any{"accounts": accts, "categories": cats})
}

// handlePieceBought marks a piece bought: it becomes yours, the purchase is
// booked in Pocket Money (when asked) and the wish is ticked off.
func (s *Server) handlePieceBought(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	uid := actorID(ctx)
	p, err := s.store.GetPiece(ctx, uid, idParam(r, "id"))
	if err != nil {
		s.fail(w, err, "piece")
		return
	}
	var in struct {
		Price     *int64 `json:"price"`
		Date      string `json:"date"`
		Book      bool   `json:"book"`
		AccountID string `json:"account_id"`
	}
	_ = decode(r, &in)
	if in.Price != nil && *in.Price >= 0 {
		p.Price = in.Price
	}
	if !validDate(in.Date) || in.Date == "" {
		in.Date = today()
	}
	st := s.settingsOf(ctx, uid)
	c := s.clients(ctx, uid)
	notes := []string{}
	out := map[string]any{"ok": true}
	if in.Book && p.Price != nil && *p.Price > 0 && p.MoneyTxID == "" {
		sib, err := c.conn(ctx, appMoney, scopeAccountsRead, scopeMoneyWrite)
		if err != nil {
			s.appFail(w, appMoney, err)
			return
		}
		acct := firstNonEmpty(in.AccountID, st.MoneyAccount)
		if acct == "" {
			if v, err := getJSONFrom(ctx, sib, "/api/v1/accounts", nil); err == nil {
				for _, m := range rows(v, "accounts") {
					if m["archived"] == true || str(m, "role") == "viewer" {
						continue
					}
					if acct == "" || m["everyday"] == true {
						acct = idOf(m)
						if m["everyday"] == true {
							break
						}
					}
				}
			}
		}
		aid, _ := strconv.ParseInt(acct, 10, 64)
		if aid == 0 {
			writeErr(w, http.StatusBadRequest, "Pocket Money has no account to book it on")
			return
		}
		body := map[string]any{"kind": "expense", "account_id": aid, "date": in.Date, "amount": *p.Price, "currency": p.Currency,
			"description": p.Name, "payee": p.Shop, "tags": []string{"furniture", "rooms"}, "notes": "Pocket Rooms: " + s.link(fmt.Sprintf("/furniture/%d", p.ID)),
			"source": "rooms", "external_id": fmt.Sprintf("rooms:piece:%d", p.ID)}
		if cid, _ := strconv.ParseInt(st.MoneyCategory, 10, 64); cid > 0 {
			body["category_id"] = cid
		}
		res, err := postJSONTo(ctx, sib, "/api/v1/transactions", body)
		if err != nil {
			s.appFail(w, appMoney, err)
			return
		}
		p.MoneyTxID = idOf(res)
		out["money_url"] = linkTo(sib, "/transactions/"+p.MoneyTxID)
		notes = append(notes, "booked in Pocket Money")
	}
	if p.WishID != nil {
		if sib, err := optional(c.conn(ctx, appWishlist, scopeWishesWrite)); err == nil && sib != nil {
			body := map[string]any{"date": in.Date}
			if p.Price != nil {
				body["price"] = *p.Price
			}
			if _, err := postJSONTo(ctx, sib, fmt.Sprintf("/api/v1/wishes/%d/bought", *p.WishID), body); err == nil {
				notes = append(notes, "ticked off your wishlist")
			}
		}
	}
	p.Status = "own"
	if err := s.store.UpdatePiece(ctx, p); err != nil {
		s.fail(w, err, "piece")
		return
	}
	s.unmarkBuy(ctx, uid, p.ID)
	text := "Marked as yours"
	if len(notes) > 0 {
		text += ", " + strings.Join(notes, " and ")
	}
	out["text"], out["piece"] = text, p
	s.publish(ctx, uid, "rooms.piece_bought", fmt.Sprintf("piece:%d", p.ID), map[string]any{"piece_id": p.ID, "name": p.Name, "price": p.Price, "currency": p.Currency})
	writeJSON(w, http.StatusOK, out)
}

// unmarkBuy clears "still to buy" on a piece's copies once it is bought.
func (s *Server) unmarkBuy(ctx context.Context, uid, pieceID int64) {
	layouts, _ := s.store.AllLayouts(ctx, uid)
	for _, l := range layouts {
		d, err := store.ParseDrawing(l.Data)
		if err != nil {
			continue
		}
		changed := false
		for i := range d.Items {
			if d.Items[i].PieceID == pieceID && d.Items[i].Buy {
				d.Items[i].Buy = false
				changed = true
			}
		}
		if changed {
			l.Data = mustJSON(d)
			_ = s.store.UpdateLayout(ctx, &l, 0)
		}
	}
}

// ---- Pocket Notes ----

func (s *Server) toNotes(ctx context.Context, uid int64, title, content string) (string, error) {
	sib, err := s.clients(ctx, uid).conn(ctx, appNotes, scopeNotesWrite)
	if err != nil {
		return "", err
	}
	folder := firstNonEmpty(s.settingsOf(ctx, uid).NotesFolder, "Rooms")
	res, err := postJSONTo(ctx, sib, "/api/v1/notes", map[string]any{"title": clip(title, 200), "content": content, "category": folder})
	if err != nil {
		return "", fmt.Errorf("Pocket Notes: %w", err)
	}
	return linkTo(sib, "/notes/"+idOf(res)), nil
}

// handlePlanNote saves a plan's summary as a note.
func (s *Server) handlePlanNote(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	uid := actorID(ctx)
	p, err := s.store.GetPlan(ctx, uid, idParam(r, "id"))
	if err != nil {
		s.fail(w, err, "plan")
		return
	}
	sums, err := s.planSummary(ctx, uid, p.ID, nil)
	if err != nil {
		s.fail(w, err, "summary")
		return
	}
	u, err := s.toNotes(ctx, uid, p.Name+" — floor plan", s.planText(ctx, uid, p, sums))
	if err != nil {
		s.userFail(w, err, "notes")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "url": u, "text": "Saved in Pocket Notes"})
}

// ---- Pocket Tasks ----

// handlePlanTasks sends a plan's open checklist items to Pocket Tasks.
func (s *Server) handlePlanTasks(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	uid := actorID(ctx)
	p, err := s.store.GetPlan(ctx, uid, idParam(r, "id"))
	if err != nil {
		s.fail(w, err, "plan")
		return
	}
	var in struct {
		IDs []int64 `json:"ids"`
	}
	_ = decode(r, &in)
	want := map[int64]bool{}
	for _, id := range in.IDs {
		want[id] = true
	}
	sib, err := s.clients(ctx, uid).conn(ctx, appTasks, scopeTasksWrite)
	if err != nil {
		s.appFail(w, appTasks, err)
		return
	}
	todos, _ := s.store.ListTodos(ctx, uid, store.TodoFilter{PlanID: p.ID, Open: true})
	n := 0
	for _, t := range todos {
		if t.TaskID != "" || (len(want) > 0 && !want[t.ID]) {
			continue
		}
		task, err := sib.client.CreateTask(ctx, clip(t.Title, 200), "From the plan “"+p.Name+"” in Pocket Rooms", t.Due, s.link(fmt.Sprintf("/plans/%d?tab=checklist", p.ID)))
		if err != nil {
			if n == 0 {
				s.appFail(w, appTasks, err)
				return
			}
			break
		}
		t.TaskID = strconv.FormatInt(task.ID, 10)
		t.TaskURL = linkTo(sib, "/tasks/"+t.TaskID)
		_ = s.store.SaveTodo(ctx, &t)
		n++
	}
	text := "Nothing new to send"
	if n > 0 {
		text = fmt.Sprintf("%d %s sent to Pocket Tasks", n, plural(n, "item", "items"))
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "sent": n, "text": text, "url": linkTo(sib, "/")})
}

// ---- Pocket Files ----

// handleLayoutFile saves a picture of a layout (drawn in the browser) in Pocket Files.
func (s *Server) handleLayoutFile(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	uid := actorID(ctx)
	l, err := s.store.GetLayout(ctx, uid, idParam(r, "id"))
	if err != nil {
		s.fail(w, err, "layout")
		return
	}
	p, err := s.store.GetPlan(ctx, uid, l.PlanID)
	if err != nil {
		s.fail(w, err, "plan")
		return
	}
	kind := r.URL.Query().Get("type")
	ctype := map[string]string{"svg": "image/svg+xml", "png": "image/png", "pdf": "application/pdf"}[kind]
	if ctype == "" {
		writeErr(w, http.StatusBadRequest, "send an SVG, PNG or PDF")
		return
	}
	data, err := s.readBody(r)
	if err != nil || len(data) == 0 {
		writeErr(w, http.StatusBadRequest, "nothing to save")
		return
	}
	sib, err := s.clients(ctx, uid).conn(ctx, appFiles, scopeFilesWrite)
	if err != nil {
		s.appFail(w, appFiles, err)
		return
	}
	folder := strings.Trim(firstNonEmpty(s.settingsOf(ctx, uid).FilesFolder, "Rooms"), "/")
	name := safeFileName(p.Name+" - "+l.Name) + "." + kind
	var res map[string]any
	q := url.Values{"space": {"home"}, "path": {"/"}, "name": {folder + "/" + name}, "conflict": {"rename"}, "mtime": {strconv.FormatInt(time.Now().UnixMilli(), 10)}}
	if err := sib.client.PutRaw(ctx, "/api/v1/fs/upload", q, data, ctype, &res); err != nil {
		s.appFail(w, appFiles, err)
		return
	}
	path := firstNonEmpty(str(res, "path"), "/"+folder+"/"+name)
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "text": "Saved in Pocket Files", "url": linkTo(sib, "/f/home"+path)})
}

func safeFileName(s string) string {
	s = strings.Map(func(r rune) rune {
		if strings.ContainsRune(`/\:*?"<>|`, r) || r < 32 {
			return '-'
		}
		return r
	}, s)
	return clip(strings.TrimSpace(s), 100)
}

// ---- Pocket Photos ----

func (s *Server) handlePhotosList(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	sib, err := s.clients(ctx, actorID(ctx)).conn(ctx, appPhotos, scopePhotosRead)
	if err != nil {
		s.appFail(w, appPhotos, err)
		return
	}
	body := map[string]any{"filter": map[string]any{"trashedAt": map[string]any{"eq": nil}, "type": map[string]any{"eq": "IMAGE"}, "visibility": map[string]any{"eq": "timeline"}},
		"size": 60, "withExif": false, "orderBy": map[string]any{"field": "fileCreatedAt", "direction": "desc"}}
	path := "/api/v1/immich/search/metadata"
	if q := strings.TrimSpace(r.URL.Query().Get("q")); q != "" {
		body["query"] = q
		path = "/api/v1/immich/search/smart"
	}
	if c := r.URL.Query().Get("cursor"); c != "" {
		body["cursor"] = c
	}
	res, err := postJSONTo(ctx, sib, path, body)
	if err != nil {
		s.appFail(w, appPhotos, err)
		return
	}
	assets := obj(res, "assets")
	out := []map[string]any{}
	for _, m := range rows(assets["items"]) {
		id := str(m, "id")
		if !reUUID.MatchString(id) {
			continue
		}
		out = append(out, map[string]any{"id": id, "name": str(m, "originalFileName"), "date": day10(m["fileCreatedAt"]), "thumb_url": "/api/v1/apps/photos/" + id + "/image"})
	}
	writeJSON(w, http.StatusOK, map[string]any{"photos": out, "cursor": assets["nextCursor"]})
}

// handlePhotoImage relays a picture from Pocket Photos (the browser cannot send the token).
func (s *Server) handlePhotoImage(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	id := chiParam(r, "uuid")
	if !reUUID.MatchString(id) {
		writeErr(w, http.StatusNotFound, "not found")
		return
	}
	sib, err := s.clients(ctx, actorID(ctx)).conn(ctx, appPhotos, scopePhotosRead)
	if err != nil {
		s.appFail(w, appPhotos, err)
		return
	}
	size := r.URL.Query().Get("size")
	if size != "preview" {
		size = "thumbnail"
	}
	b, ctype, err := sib.client.Bytes(ctx, "/api/v1/immich/assets/"+id+"/thumbnail", url.Values{"size": {size}}, 16<<20)
	if err != nil {
		s.appFail(w, appPhotos, err)
		return
	}
	w.Header().Set("Content-Type", firstNonEmpty(ctype, http.DetectContentType(b)))
	w.Header().Set("Cache-Control", "private, max-age=3600")
	_, _ = w.Write(b)
}

// handlePhotoCopy keeps a copy of a Pocket Photos picture here (to trace over it).
func (s *Server) handlePhotoCopy(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	uid := actorID(ctx)
	id := chiParam(r, "uuid")
	if !reUUID.MatchString(id) {
		writeErr(w, http.StatusNotFound, "not found")
		return
	}
	sib, err := s.clients(ctx, uid).conn(ctx, appPhotos, scopePhotosRead)
	if err != nil {
		s.appFail(w, appPhotos, err)
		return
	}
	b, _, err := sib.client.Bytes(ctx, "/api/v1/immich/assets/"+id+"/thumbnail", url.Values{"size": {"preview"}}, maxPhoto)
	if err != nil {
		s.appFail(w, appPhotos, err)
		return
	}
	p, err := s.savePhoto(ctx, uid, firstNonEmpty(r.URL.Query().Get("kind"), "underlay"), b)
	if err != nil {
		s.userFail(w, err, "photo")
		return
	}
	u, t := photoURL(&p.ID)
	writeJSON(w, http.StatusCreated, map[string]any{"id": p.ID, "url": u, "thumb_url": t, "width": p.Width, "height": p.Height})
}

// ---- Pocket Rentals ----

func (s *Server) handleRentals(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	sib, err := s.clients(ctx, actorID(ctx)).conn(ctx, appRentals, scopeRentalsRead)
	if err != nil {
		s.appFail(w, appRentals, err)
		return
	}
	v, err := getJSONFrom(ctx, sib, "/api/v1/units", nil)
	if err != nil {
		s.appFail(w, appRentals, err)
		return
	}
	out := []map[string]any{}
	for _, m := range rows(v, "units") {
		if m["archived"] == true {
			continue
		}
		out = append(out, map[string]any{"id": num(m, "id"), "name": joinNonEmpty(" · ", str(m, "property_name"), str(m, "name")), "unit": str(m, "name"),
			"property": str(m, "property_name"), "bedrooms": num(m, "bedrooms"), "bathrooms": num(m, "bathrooms"), "area_m2": num(m, "area_m2"),
			"url": linkTo(sib, "/properties/"+strconv.FormatInt(int64(num(m, "property_id")), 10))})
	}
	writeJSON(w, http.StatusOK, map[string]any{"units": out})
}

// ---- Pocket Chat ----

func (s *Server) handleChats(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	sib, err := s.clients(ctx, actorID(ctx)).conn(ctx, appChat, scopeChatRead)
	if err != nil {
		s.appFail(w, appChat, err)
		return
	}
	v, err := getJSONFrom(ctx, sib, "/api/v1/conversations", nil)
	if err != nil {
		s.appFail(w, appChat, err)
		return
	}
	out := []map[string]any{}
	for _, m := range rows(v, "conversations") {
		if m["archived"] == true {
			continue
		}
		out = append(out, map[string]any{"id": idOf(m), "name": firstNonEmpty(str(m, "name"), str(m, "title")), "kind": str(m, "kind"), "emoji": str(m, "emoji")})
	}
	writeJSON(w, http.StatusOK, map[string]any{"conversations": out})
}

// handleShareChat posts a layout to a chat as a card (with the read-only link when on).
func (s *Server) handleShareChat(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	uid := actorID(ctx)
	var in struct {
		To       string `json:"to"`
		Text     string `json:"text"`
		LayoutID int64  `json:"layout_id"`
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
	p, err := s.store.GetPlan(ctx, uid, l.PlanID)
	if err != nil {
		s.fail(w, err, "plan")
		return
	}
	st := s.settingsOf(ctx, uid)
	d, _ := store.ParseDrawing(l.Data)
	floors, _ := s.store.ListFloors(ctx, uid, p.ID)
	ceiling := 250.0
	for _, f := range floors {
		if f.ID == l.FloorID {
			ceiling = f.CeilingCm
		}
	}
	sum := summarize(d, ceiling, s.piecesByID(ctx, uid), st.Units)
	lines := []string{fmt.Sprintf("%s · %d %s · %d %s", areaText(sum.AreaCm2, st.Units), len(sum.Rooms), plural(len(sum.Rooms), "room", "rooms"), len(sum.Items), plural(len(sum.Items), "piece", "pieces"))}
	if sum.ToBuy > 0 {
		lines = append(lines, fmt.Sprintf("%d to buy · %s", sum.ToBuy, money(sum.Budget, p.Currency)))
	}
	if l.Notes != "" {
		lines = append(lines, firstLines(l.Notes, 2)...)
	}
	link := s.link(fmt.Sprintf("/plans/%d?layout=%d", p.ID, l.ID))
	if p.ShareOn {
		link = s.link(fmt.Sprintf("/s/%s?layout=%d", p.ShareToken, l.ID))
	}
	card := map[string]any{"title": p.Name + " — " + l.Name, "subtitle": "Floor plan", "lines": lines, "icon": "📐", "url": link, "app": "rooms", "kind": "card"}
	sib, err := s.clients(ctx, uid).conn(ctx, appChat, scopeChatWrite)
	if err != nil {
		s.appFail(w, appChat, err)
		return
	}
	text := firstNonEmpty(strings.TrimSpace(in.Text), "What do you think of this layout?")
	body := map[string]any{"to": firstNonEmpty(strings.TrimSpace(in.To), st.ChatTo, "me"), "text": clip(text, 2000), "card": card,
		"client_id": fmt.Sprintf("rooms-%d-%d-%d", uid, l.ID, time.Now().Unix()/30)}
	res, err := postJSONTo(ctx, sib, "/api/v1/send", body)
	if err != nil {
		s.appFail(w, appChat, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "url": linkTo(sib, "/c/"+str(res, "conversation_id")), "text": "Sent to Pocket Chat"})
}
