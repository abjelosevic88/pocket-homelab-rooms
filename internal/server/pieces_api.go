package server

import (
	"bytes"
	"context"
	"fmt"
	"image"
	_ "image/jpeg"
	_ "image/png"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/abjelosevic88/pocket-homelab-rooms/internal/store"
)

// ---- furniture: your own pieces, measured ----

// placedIn says where each piece stands on the current layouts.
func (s *Server) placedIn(ctx context.Context, uid int64) map[int64][]string {
	out := map[int64][]string{}
	plans, _ := s.store.ListPlans(ctx, uid, false)
	for _, p := range plans {
		floors, _ := s.store.ListFloors(ctx, uid, p.ID)
		layouts, _ := s.store.ListLayouts(ctx, uid, p.ID, true)
		for _, f := range floors {
			l := currentOf(f, layouts)
			if l == nil {
				continue
			}
			d, _ := store.ParseDrawing(l.Data)
			for _, it := range d.Items {
				if it.PieceID > 0 {
					out[it.PieceID] = append(out[it.PieceID], joinNonEmpty(" · ", p.Name, d.RoomOf(it)))
				}
			}
		}
	}
	return out
}

func (s *Server) decoratePieces(ctx context.Context, uid int64, list []store.Piece) {
	where := s.placedIn(ctx, uid)
	for i := range list {
		p := &list[i]
		p.PhotoURL, _ = photoURL(p.PhotoID)
		if w := where[p.ID]; w != nil {
			p.PlacedIn = w
		}
	}
}

func (s *Server) handleListPieces(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	uid := actorID(ctx)
	q := r.URL.Query()
	list, err := s.store.ListPieces(ctx, uid, store.PieceFilter{Status: q.Get("status"), Q: q.Get("q"), Limit: queryInt(r, "limit")})
	if err != nil {
		s.fail(w, err, "pieces")
		return
	}
	s.decoratePieces(ctx, uid, list)
	writeJSON(w, http.StatusOK, list)
}

type pieceIn struct {
	Name          *string  `json:"name"`
	Type          *string  `json:"type"`
	Category      *string  `json:"category"`
	Width         *float64 `json:"width"`
	Depth         *float64 `json:"depth"`
	Height        *float64 `json:"height"`
	Color         *string  `json:"color"`
	Status        *string  `json:"status"`
	Price         *int64   `json:"price"`
	Currency      *string  `json:"currency"`
	URL           *string  `json:"url"`
	Shop          *string  `json:"shop"`
	Notes         *string  `json:"notes"`
	PhotoID       *int64   `json:"photo_id"`
	InventoryID   *int64   `json:"inventory_id"`
	InventoryName *string  `json:"inventory_name"`
	WishID        *int64   `json:"wish_id"`
	WishName      *string  `json:"wish_name"`
}

func (s *Server) applyPiece(ctx context.Context, keys map[string]bool, in pieceIn, p *store.Piece) error {
	str := func(k string, dst *string, v *string, n int) {
		if keys[k] && v != nil {
			*dst = cleanText(*v, n)
		}
	}
	str("name", &p.Name, in.Name, maxName)
	str("shop", &p.Shop, in.Shop, maxName)
	str("notes", &p.Notes, in.Notes, maxNotes)
	str("inventory_name", &p.InventoryName, in.InventoryName, maxName)
	str("wish_name", &p.WishName, in.WishName, maxName)
	if keys["type"] && in.Type != nil {
		p.Type = firstNonEmpty(cleanText(*in.Type, 40), "box")
	}
	if keys["category"] && in.Category != nil {
		if !oneOf(*in.Category, store.Categories) {
			return &userErr{"unknown category"}
		}
		p.Category = *in.Category
	}
	dim := func(k string, dst *float64, v *float64) error {
		if keys[k] && v != nil {
			if *v <= 0 || *v > 5000 {
				return &userErr{"sizes are in centimetres, more than 0"}
			}
			*dst = *v
		}
		return nil
	}
	for _, x := range []struct {
		k   string
		dst *float64
		v   *float64
	}{{"width", &p.Width, in.Width}, {"depth", &p.Depth, in.Depth}, {"height", &p.Height, in.Height}} {
		if err := dim(x.k, x.dst, x.v); err != nil {
			return err
		}
	}
	if keys["color"] && in.Color != nil {
		if *in.Color != "" && !reColor.MatchString(*in.Color) {
			return &userErr{"a colour is #rrggbb"}
		}
		p.Color = *in.Color
	}
	if keys["status"] && in.Status != nil {
		if !oneOf(*in.Status, store.PieceStates) {
			return &userErr{"a piece is yours, wanted, maybe, or gone"}
		}
		p.Status = *in.Status
	}
	if keys["price"] {
		p.Price = in.Price
		if p.Price != nil && *p.Price < 0 {
			return &userErr{"a price is not negative"}
		}
	}
	if keys["currency"] && in.Currency != nil {
		p.Currency = cleanCurrency(*in.Currency)
	}
	if keys["url"] && in.URL != nil {
		u, ok := cleanURL(*in.URL)
		if !ok {
			return &userErr{"the link should start with http:// or https://"}
		}
		p.URL = u
	}
	if keys["photo_id"] {
		p.PhotoID = in.PhotoID
		if p.PhotoID != nil && (*p.PhotoID == 0 || !s.store.OwnsPhoto(ctx, p.UserID, *p.PhotoID)) {
			p.PhotoID = nil
		}
	}
	if keys["inventory_id"] {
		p.InventoryID = in.InventoryID
		if p.InventoryID != nil && *p.InventoryID == 0 {
			p.InventoryID, p.InventoryName = nil, ""
		}
	}
	if keys["wish_id"] {
		p.WishID = in.WishID
		if p.WishID != nil && *p.WishID == 0 {
			p.WishID, p.WishName = nil, ""
		}
	}
	if strings.TrimSpace(p.Name) == "" {
		return &userErr{"give the piece a name"}
	}
	return nil
}

func (s *Server) handleCreatePiece(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	uid := actorID(ctx)
	var in pieceIn
	keys, err := decodeKeys(r, &in)
	if err != nil {
		writeErr(w, http.StatusBadRequest, "invalid json")
		return
	}
	st := s.settingsOf(ctx, uid)
	p := &store.Piece{UserID: uid, Type: "box", Category: "living", Width: 100, Depth: 60, Height: 80, Status: "own", Currency: firstNonEmpty(st.Currency, "EUR"), Source: sourceOf(ctx)}
	if linkApp(ctx) != "" {
		p.Status = "want"
	}
	if err := s.applyPiece(ctx, keys, in, p); err != nil {
		s.userFail(w, err, "piece")
		return
	}
	if err := s.store.CreatePiece(ctx, p); err != nil {
		s.fail(w, err, "piece")
		return
	}
	s.publish(ctx, uid, "rooms.piece_added", fmt.Sprintf("piece:%d", p.ID), map[string]any{"piece_id": p.ID, "name": p.Name, "status": p.Status,
		"width_cm": p.Width, "depth_cm": p.Depth, "height_cm": p.Height, "url": s.link(fmt.Sprintf("/furniture/%d", p.ID))})
	p.PhotoURL, _ = photoURL(p.PhotoID)
	writeJSON(w, http.StatusCreated, p)
}

func (s *Server) handleGetPiece(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	uid := actorID(ctx)
	p, err := s.store.GetPiece(ctx, uid, idParam(r, "id"))
	if err != nil {
		s.fail(w, err, "piece")
		return
	}
	list := []store.Piece{*p}
	s.decoratePieces(ctx, uid, list)
	st := s.settingsOf(ctx, uid)
	// how it does against every way in
	routes, _ := s.store.ListRoutes(ctx, uid, 0)
	plans := map[int64]string{}
	if ps, err := s.store.ListPlans(ctx, uid, false); err == nil {
		for _, pl := range ps {
			plans[pl.ID] = pl.Name
		}
	}
	fits := []map[string]any{}
	for _, rt := range routes {
		name, ok := plans[rt.PlanID]
		if !ok || len(rt.Steps) == 0 {
			continue
		}
		fits = append(fits, map[string]any{"route_id": rt.ID, "route": rt.Name, "plan_id": rt.PlanID, "plan": name, "result": checkFit(p.Width, p.Depth, p.Height, rt.Steps, st.HandsCm, st.Units)})
	}
	out := map[string]any{"piece": list[0], "fits": fits}
	if p.InventoryID != nil {
		out["inventory_url"] = s.siblingLink(ctx, uid, appInventory, fmt.Sprintf("/items/%d", *p.InventoryID))
	}
	if p.WishID != nil {
		out["wish_url"] = s.siblingLink(ctx, uid, appWishlist, fmt.Sprintf("/wishes/%d", *p.WishID))
	}
	if p.MoneyTxID != "" {
		out["money_url"] = s.siblingLink(ctx, uid, appMoney, "/transactions/"+p.MoneyTxID)
	}
	writeJSON(w, http.StatusOK, out)
}

func (s *Server) handleUpdatePiece(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	uid := actorID(ctx)
	p, err := s.store.GetPiece(ctx, uid, idParam(r, "id"))
	if err != nil {
		s.fail(w, err, "piece")
		return
	}
	if !mayChange(ctx, p.Source) {
		writeErr(w, http.StatusForbidden, "this app may change only the pieces it added")
		return
	}
	var in pieceIn
	keys, err := decodeKeys(r, &in)
	if err != nil {
		writeErr(w, http.StatusBadRequest, "invalid json")
		return
	}
	if err := s.applyPiece(ctx, keys, in, p); err != nil {
		s.userFail(w, err, "piece")
		return
	}
	if err := s.store.UpdatePiece(ctx, p); err != nil {
		s.fail(w, err, "piece")
		return
	}
	if keys["width"] || keys["depth"] || keys["height"] || keys["name"] || keys["type"] || keys["color"] {
		s.resizePlaced(ctx, uid, p)
	}
	p.PhotoURL, _ = photoURL(p.PhotoID)
	writeJSON(w, http.StatusOK, p)
}

// resizePlaced keeps the copies of a piece on every layout in step with it.
func (s *Server) resizePlaced(ctx context.Context, uid int64, p *store.Piece) {
	layouts, _ := s.store.AllLayouts(ctx, uid)
	for _, l := range layouts {
		d, err := store.ParseDrawing(l.Data)
		if err != nil {
			continue
		}
		changed := false
		for i := range d.Items {
			it := &d.Items[i]
			if it.PieceID != p.ID {
				continue
			}
			it.W, it.D, it.H, it.Name, it.Type = p.Width, p.Depth, p.Height, p.Name, p.Type
			if p.Color != "" {
				it.Color = p.Color
			}
			changed = true
		}
		if changed {
			l.Data = mustJSON(d)
			_ = s.store.UpdateLayout(ctx, &l, 0)
		}
	}
}

func (s *Server) handleDeletePiece(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	uid := actorID(ctx)
	p, err := s.store.GetPiece(ctx, uid, idParam(r, "id"))
	if err != nil {
		s.fail(w, err, "piece")
		return
	}
	if !mayChange(ctx, p.Source) {
		writeErr(w, http.StatusForbidden, "this app may remove only the pieces it added")
		return
	}
	if err := s.store.DeletePiece(ctx, uid, p.ID); err != nil {
		s.fail(w, err, "piece")
		return
	}
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

// ---- pictures ----

const maxPhoto = 8 << 20

func (s *Server) handleUploadPhoto(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	uid := actorID(ctx)
	r.Body = http.MaxBytesReader(w, r.Body, 2*maxPhoto+(1<<20))
	if err := r.ParseMultipartForm(4 << 20); err != nil {
		writeErr(w, http.StatusBadRequest, "the picture is too big (8 MB at most)")
		return
	}
	read := func(field string) ([]byte, string, error) {
		f, h, err := r.FormFile(field)
		if err != nil {
			return nil, "", err
		}
		defer f.Close()
		b, err := io.ReadAll(io.LimitReader(f, maxPhoto+1))
		if err != nil {
			return nil, "", err
		}
		if len(b) > maxPhoto {
			return nil, "", fmt.Errorf("too big")
		}
		return b, h.Header.Get("Content-Type"), nil
	}
	data, _, err := read("file")
	if err != nil {
		writeErr(w, http.StatusBadRequest, "send the picture as “file”")
		return
	}
	p, err := s.savePhoto(ctx, uid, r.FormValue("kind"), data)
	if err != nil {
		s.userFail(w, err, "photo")
		return
	}
	if p.Width == 0 {
		p.Width, _ = strconv.Atoi(r.FormValue("width"))
		p.Height, _ = strconv.Atoi(r.FormValue("height"))
	}
	if t, tm, err := read("thumb"); err == nil && len(t) > 0 {
		if http.DetectContentType(t) == "image/jpeg" || tm == "image/jpeg" {
			_, _ = s.store.DB.ExecContext(ctx, `UPDATE photos SET thumb = ?, width = ?, height = ? WHERE id = ? AND user_id = ?`, t, p.Width, p.Height, p.ID, uid)
		}
	}
	u, t := photoURL(&p.ID)
	writeJSON(w, http.StatusCreated, map[string]any{"id": p.ID, "url": u, "thumb_url": t, "width": p.Width, "height": p.Height})
}

// savePhoto stores a picture the person sent (or one fetched from Pocket Photos).
func (s *Server) savePhoto(ctx context.Context, uid int64, kind string, data []byte) (*store.Photo, error) {
	mime := http.DetectContentType(data)
	if mime != "image/jpeg" && mime != "image/png" && mime != "image/webp" {
		return nil, &userErr{"send a JPEG, PNG or WebP picture"}
	}
	p := &store.Photo{UserID: uid, Kind: kind, Mime: mime, Data: data}
	if !oneOf(p.Kind, store.PhotoKinds) {
		p.Kind = "room"
	}
	if cfg, _, err := image.DecodeConfig(bytes.NewReader(data)); err == nil {
		p.Width, p.Height = cfg.Width, cfg.Height
	}
	return p, s.store.CreatePhoto(ctx, p)
}

func (s *Server) handlePhoto(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	p, err := s.store.GetPhoto(ctx, actorID(ctx), idParam(r, "id"), queryBool(r, "thumb"))
	if err != nil {
		s.fail(w, err, "photo")
		return
	}
	servePhoto(w, r, p)
}

func servePhoto(w http.ResponseWriter, r *http.Request, p *store.Photo) {
	w.Header().Set("Content-Type", p.Mime)
	w.Header().Set("Cache-Control", "private, max-age=31536000, immutable")
	http.ServeContent(w, r, "", time.Time{}, bytes.NewReader(p.Data))
}
