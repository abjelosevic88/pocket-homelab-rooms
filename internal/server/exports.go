package server

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/csv"
	"encoding/json"
	"encoding/xml"
	"fmt"
	"io"
	"math"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/abjelosevic88/pocket-homelab-rooms/internal/store"
)

// exportDoc is everything of one person, to keep or to bring to another box.
type exportDoc struct {
	App        string            `json:"app"`
	Version    string            `json:"version"`
	ExportedAt string            `json:"exported_at"`
	Plans      []store.Plan      `json:"plans"`
	Floors     []store.Floor     `json:"floors"`
	Layouts    []store.Layout    `json:"layouts"`
	Pieces     []store.Piece     `json:"pieces"`
	Routes     []store.Route     `json:"routes"`
	Todos      []store.Todo      `json:"todos"`
	PlanPhotos []store.PlanPhoto `json:"plan_photos"`
	Photos     []exportPhoto     `json:"photos"`
}

type exportPhoto struct {
	ID    int64  `json:"id"`
	Kind  string `json:"kind"`
	Mime  string `json:"mime"`
	Data  []byte `json:"data"` // base64 in JSON
	Thumb []byte `json:"thumb,omitempty"`
}

func (s *Server) export(ctx context.Context, uid int64, photos bool, only int64) (exportDoc, error) {
	doc := exportDoc{App: "pocket-rooms", Version: Version, ExportedAt: time.Now().UTC().Format(time.RFC3339),
		Plans: []store.Plan{}, Floors: []store.Floor{}, Layouts: []store.Layout{}, Pieces: []store.Piece{}, Routes: []store.Route{}, Todos: []store.Todo{},
		PlanPhotos: []store.PlanPhoto{}, Photos: []exportPhoto{}}
	plans, err := s.store.ListPlans(ctx, uid, false)
	if err != nil {
		return doc, err
	}
	arch, _ := s.store.ListPlans(ctx, uid, true)
	photoIDs := map[int64]bool{}
	for _, p := range append(plans, arch...) {
		if only > 0 && p.ID != only {
			continue
		}
		doc.Plans = append(doc.Plans, p)
		fl, _ := s.store.ListFloors(ctx, uid, p.ID)
		for _, f := range fl {
			if f.UnderlayPhotoID != nil {
				photoIDs[*f.UnderlayPhotoID] = true
			}
		}
		doc.Floors = append(doc.Floors, fl...)
		ls, _ := s.store.ListLayouts(ctx, uid, p.ID, true)
		doc.Layouts = append(doc.Layouts, ls...)
		rs, _ := s.store.ListRoutes(ctx, uid, p.ID)
		doc.Routes = append(doc.Routes, rs...)
		ts, _ := s.store.ListTodos(ctx, uid, store.TodoFilter{PlanID: p.ID})
		doc.Todos = append(doc.Todos, ts...)
		pp, _ := s.store.ListPlanPhotos(ctx, uid, p.ID)
		for _, x := range pp {
			if x.PhotoID != nil {
				photoIDs[*x.PhotoID] = true
			}
		}
		doc.PlanPhotos = append(doc.PlanPhotos, pp...)
	}
	if only == 0 {
		own, _ := s.store.ListPieces(ctx, uid, store.PieceFilter{})
		gone, _ := s.store.ListPieces(ctx, uid, store.PieceFilter{Status: "gone"})
		doc.Pieces = append(own, gone...)
		for _, p := range doc.Pieces {
			if p.PhotoID != nil {
				photoIDs[*p.PhotoID] = true
			}
		}
	}
	if photos {
		for id := range photoIDs {
			p, err := s.store.GetPhoto(ctx, uid, id, false)
			if err != nil {
				continue
			}
			var thumb []byte
			_ = s.store.DB.QueryRowContext(ctx, `SELECT thumb FROM photos WHERE id = ?`, id).Scan(&thumb)
			doc.Photos = append(doc.Photos, exportPhoto{ID: p.ID, Kind: p.Kind, Mime: p.Mime, Data: p.Data, Thumb: thumb})
		}
	}
	return doc, nil
}

func (s *Server) handleExportJSON(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	only := int64(queryInt(r, "plan"))
	doc, err := s.export(ctx, actorID(ctx), r.URL.Query().Get("photos") != "0", only)
	if err != nil {
		s.fail(w, err, "export")
		return
	}
	name := "pocket-rooms-" + time.Now().Format("2006-01-02")
	if only > 0 && len(doc.Plans) == 1 {
		name = safeFileName(doc.Plans[0].Name)
	}
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Content-Disposition", `attachment; filename="`+name+`.json"`)
	enc := json.NewEncoder(w)
	enc.SetIndent("", " ")
	_ = enc.Encode(doc)
}

// handleExportCSV is the furniture (or a plan's rooms) as a spreadsheet.
func (s *Server) handleExportCSV(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	uid := actorID(ctx)
	what := firstNonEmpty(r.URL.Query().Get("what"), "furniture")
	w.Header().Set("Content-Type", "text/csv; charset=utf-8")
	w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="pocket-rooms-%s-%s.csv"`, what, time.Now().Format("2006-01-02")))
	cw := csv.NewWriter(w)
	defer cw.Flush()
	f := func(v float64) string { return strconv.FormatFloat(math.Round(v*10)/10, 'f', -1, 64) }
	if what == "rooms" {
		_ = cw.Write([]string{"plan", "floor", "layout", "room", "kind", "floor_finish", "area_m2", "perimeter_m", "walls_to_paint_m2", "skirting_m", "doors", "windows"})
		plans, _ := s.store.ListPlans(ctx, uid, false)
		for _, p := range plans {
			sums, _ := s.planSummary(ctx, uid, p.ID, nil)
			for _, fl := range sums {
				for _, rm := range fl.Rooms {
					_ = cw.Write([]string{p.Name, fl.FloorName, fl.Layout, rm.Name, rm.Kind, rm.Floor, f(rm.AreaCm2 / 10000), f(rm.Perimeter / 100), f(rm.WallCm2 / 10000),
						f(rm.Skirting / 100), strconv.Itoa(rm.Doors), strconv.Itoa(rm.Windows)})
				}
			}
		}
		return
	}
	list, _ := s.store.ListPieces(ctx, uid, store.PieceFilter{})
	where := s.placedIn(ctx, uid)
	_ = cw.Write([]string{"name", "type", "category", "width_cm", "depth_cm", "height_cm", "status", "price", "currency", "shop", "url", "placed_in", "notes"})
	for _, p := range list {
		price := ""
		if p.Price != nil {
			price = fmt.Sprintf("%.2f", float64(*p.Price)/100)
		}
		_ = cw.Write([]string{p.Name, p.Type, p.Category, f(p.Width), f(p.Depth), f(p.Height), p.Status, price, p.Currency, p.Shop, p.URL, strings.Join(where[p.ID], "; "), p.Notes})
	}
}

func readUpload(w http.ResponseWriter, r *http.Request) ([]byte, string, error) {
	if strings.HasPrefix(r.Header.Get("Content-Type"), "multipart/") {
		r.Body = http.MaxBytesReader(w, r.Body, 512<<20)
		if err := r.ParseMultipartForm(32 << 20); err != nil {
			return nil, "", &userErr{"the file is too big"}
		}
		f, h, err := r.FormFile("file")
		if err != nil {
			return nil, "", &userErr{"no file"}
		}
		defer f.Close()
		b, err := io.ReadAll(f)
		return b, h.Filename, err
	}
	b, err := io.ReadAll(http.MaxBytesReader(w, r.Body, 512<<20))
	if err != nil {
		return nil, "", &userErr{"the file is too big"}
	}
	return b, r.URL.Query().Get("name"), nil
}

// handleImport takes a Pocket Rooms export (again and again: nothing is
// doubled) or a Sweet Home 3D file (.sh3d, or its Home.xml).
func (s *Server) handleImport(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	uid := actorID(ctx)
	data, name, err := readUpload(w, r)
	if err != nil {
		s.userFail(w, err, "import")
		return
	}
	data = bytes.TrimPrefix(data, []byte("\xef\xbb\xbf"))
	if bytes.HasPrefix(data, []byte("PK")) || bytes.HasPrefix(bytes.TrimSpace(data), []byte("<?xml")) || bytes.HasPrefix(bytes.TrimSpace(data), []byte("<home")) {
		res, err := s.importSH3D(ctx, uid, data, name)
		if err != nil {
			s.userFail(w, err, "import")
			return
		}
		writeJSON(w, http.StatusOK, res)
		return
	}
	var doc exportDoc
	if err := json.Unmarshal(data, &doc); err != nil || doc.App != "pocket-rooms" {
		writeErr(w, http.StatusBadRequest, "that is not a Pocket Rooms export or a Sweet Home 3D file")
		return
	}
	res, err := s.importPocket(ctx, uid, doc)
	if err != nil {
		s.fail(w, err, "import")
		return
	}
	writeJSON(w, http.StatusOK, res)
}

// importPocket brings back an export. What came in is remembered (export
// id + when it was made → the new id), so a second import finds it instead
// of doubling it.
func (s *Server) importPocket(ctx context.Context, uid int64, doc exportDoc) (map[string]any, error) {
	added := map[string]int{}
	key := func(kind string, id int64, at string) string {
		return fmt.Sprintf("import:%d:%s:%d:%s", uid, kind, id, at)
	}
	seen := func(k string) (int64, bool) {
		if v, _, ok := s.store.CacheGet(ctx, k); ok {
			n, _ := strconv.ParseInt(v, 10, 64)
			return n, n > 0
		}
		return 0, false
	}
	remember := func(k string, id int64) { _ = s.store.CachePut(ctx, k, strconv.FormatInt(id, 10)) }
	photos := map[int64]int64{}
	for _, p := range doc.Photos {
		k := key("photo", p.ID, doc.ExportedAt)
		if id, ok := seen(k); ok {
			photos[p.ID] = id
			continue
		}
		np := &store.Photo{UserID: uid, Kind: p.Kind, Mime: p.Mime, Data: p.Data, Thumb: p.Thumb}
		if !oneOf(np.Kind, store.PhotoKinds) {
			np.Kind = "room"
		}
		if err := s.store.CreatePhoto(ctx, np); err != nil {
			return nil, err
		}
		photos[p.ID] = np.ID
		remember(k, np.ID)
		added["photos"]++
	}
	mapPhoto := func(id *int64) *int64 {
		if id == nil {
			return nil
		}
		if n, ok := photos[*id]; ok {
			return &n
		}
		return nil
	}
	pieces := map[int64]int64{}
	for _, p := range doc.Pieces {
		k := key("piece", p.ID, doc.ExportedAt)
		if id, ok := seen(k); ok {
			pieces[p.ID] = id
			continue
		}
		old := p.ID
		p.ID, p.UserID, p.PhotoID, p.Source = 0, uid, mapPhoto(p.PhotoID), ""
		if !oneOf(p.Status, store.PieceStates) {
			p.Status = "own"
		}
		if !oneOf(p.Category, store.Categories) {
			p.Category = "other"
		}
		if err := s.store.CreatePiece(ctx, &p); err != nil {
			return nil, err
		}
		pieces[old] = p.ID
		remember(k, p.ID)
		added["pieces"]++
	}
	plans := map[int64]int64{}
	for _, p := range doc.Plans {
		k := key("plan", p.ID, doc.ExportedAt)
		if id, ok := seen(k); ok {
			plans[p.ID] = id
			continue
		}
		old := p.ID
		p.ID, p.UserID, p.ShareOn, p.ShareToken, p.Source = 0, uid, false, "", ""
		if !oneOf(p.Kind, store.PlanKinds) {
			p.Kind = "other"
		}
		if !oneOf(p.Status, store.PlanStates) {
			p.Status = "living"
		}
		if err := s.store.CreatePlan(ctx, &p); err != nil {
			return nil, err
		}
		if p.Archived {
			_ = s.store.UpdatePlan(ctx, &p)
		}
		plans[old] = p.ID
		remember(k, p.ID)
		added["plans"]++
	}
	floors := map[int64]int64{}
	floorRows := map[int64]store.Floor{}
	for _, f := range doc.Floors {
		pid, ok := plans[f.PlanID]
		if !ok {
			continue
		}
		k := key("floor", f.ID, doc.ExportedAt)
		if id, ok := seen(k); ok {
			floors[f.ID] = id
			continue
		}
		old := f
		f.ID, f.UserID, f.PlanID, f.UnderlayPhotoID, f.CurrentLayoutID = 0, uid, pid, mapPhoto(f.UnderlayPhotoID), nil
		if err := s.store.CreateFloor(ctx, &f); err != nil {
			return nil, err
		}
		floors[old.ID] = f.ID
		f.CurrentLayoutID = old.CurrentLayoutID // remapped below
		floorRows[f.ID] = f
		remember(k, f.ID)
	}
	layouts := map[int64]int64{}
	for _, l := range doc.Layouts {
		fid, ok := floors[l.FloorID]
		if !ok {
			continue
		}
		k := key("layout", l.ID, doc.ExportedAt)
		if id, ok := seen(k); ok {
			layouts[l.ID] = id
			continue
		}
		d, err := store.ParseDrawing(l.Data)
		if err != nil {
			continue
		}
		for i := range d.Items {
			if n, ok := pieces[d.Items[i].PieceID]; ok {
				d.Items[i].PieceID = n
			} else {
				d.Items[i].PieceID = 0
			}
		}
		old := l.ID
		l.ID, l.UserID, l.PlanID, l.FloorID, l.Data = 0, uid, plans[l.PlanID], fid, mustJSON(d)
		if err := s.store.CreateLayout(ctx, &l); err != nil {
			return nil, err
		}
		layouts[old] = l.ID
		remember(k, l.ID)
		added["layouts"]++
	}
	for id, f := range floorRows {
		if f.CurrentLayoutID != nil {
			if n, ok := layouts[*f.CurrentLayoutID]; ok {
				f.ID, f.CurrentLayoutID = id, &n
				_ = s.store.UpdateFloor(ctx, &f)
			}
		}
	}
	for _, rt := range doc.Routes {
		pid, ok := plans[rt.PlanID]
		k := key("route", rt.ID, doc.ExportedAt)
		if _, done := seen(k); !ok || done {
			continue
		}
		rt.ID, rt.UserID, rt.PlanID = 0, uid, pid
		if err := s.store.SaveRoute(ctx, &rt); err != nil {
			return nil, err
		}
		remember(k, rt.ID)
	}
	for _, t := range doc.Todos {
		pid, ok := plans[t.PlanID]
		k := key("todo", t.ID, doc.ExportedAt)
		if _, done := seen(k); !ok || done {
			continue
		}
		t.ID, t.UserID, t.PlanID, t.Source, t.TaskID, t.TaskURL = 0, uid, pid, "", "", ""
		if err := s.store.SaveTodo(ctx, &t); err != nil {
			return nil, err
		}
		remember(k, t.ID)
	}
	for _, pp := range doc.PlanPhotos {
		pid, ok := plans[pp.PlanID]
		k := key("plan_photo", pp.ID, doc.ExportedAt)
		if _, done := seen(k); !ok || done {
			continue
		}
		pp.ID, pp.UserID, pp.PlanID, pp.PhotoID = 0, uid, pid, mapPhoto(pp.PhotoID)
		if pp.PhotoID == nil && pp.PocketID == "" {
			continue
		}
		if err := s.store.SavePlanPhoto(ctx, &pp); err != nil {
			return nil, err
		}
		remember(k, pp.ID)
	}
	return map[string]any{"ok": true, "added": added}, nil
}

// ---- Sweet Home 3D ----
//
// A .sh3d file is a zip with a "Home.xml" (Sweet Home 3D 5.3 and newer):
// lengths in centimetres, x to the right and y down like here. We take the
// rooms, walls, doors and windows, and the furniture with its sizes.

type sh3dHome struct {
	Name   string `xml:"name,attr"`
	Levels []struct {
		ID        string  `xml:"id,attr"`
		Name      string  `xml:"name,attr"`
		Elevation float64 `xml:"elevation,attr"`
		Height    float64 `xml:"height,attr"`
	} `xml:"level"`
	Rooms []struct {
		Level  string `xml:"level,attr"`
		Name   string `xml:"name,attr"`
		Points []struct {
			X float64 `xml:"x,attr"`
			Y float64 `xml:"y,attr"`
		} `xml:"point"`
	} `xml:"room"`
	Walls []struct {
		Level     string  `xml:"level,attr"`
		XStart    float64 `xml:"xStart,attr"`
		YStart    float64 `xml:"yStart,attr"`
		XEnd      float64 `xml:"xEnd,attr"`
		YEnd      float64 `xml:"yEnd,attr"`
		Thickness float64 `xml:"thickness,attr"`
	} `xml:"wall"`
	Furniture []sh3dPiece `xml:"pieceOfFurniture"`
	Doors     []sh3dPiece `xml:"doorOrWindow"`
	Groups    []struct {
		Furniture []sh3dPiece `xml:"pieceOfFurniture"`
		Doors     []sh3dPiece `xml:"doorOrWindow"`
	} `xml:"furnitureGroup"`
}

type sh3dPiece struct {
	Level     string  `xml:"level,attr"`
	Name      string  `xml:"name,attr"`
	Catalog   string  `xml:"catalogId,attr"`
	X         float64 `xml:"x,attr"`
	Y         float64 `xml:"y,attr"`
	Elevation float64 `xml:"elevation,attr"`
	Angle     float64 `xml:"angle,attr"` // radians
	Width     float64 `xml:"width,attr"`
	Depth     float64 `xml:"depth,attr"`
	Height    float64 `xml:"height,attr"`
	Visible   string  `xml:"visible,attr"`
	Color     string  `xml:"color,attr"`
	Price     string  `xml:"price,attr"`
}

func (s *Server) importSH3D(ctx context.Context, uid int64, data []byte, fname string) (map[string]any, error) {
	xmlData := data
	if bytes.HasPrefix(data, []byte("PK")) {
		zr, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
		if err != nil {
			return nil, &userErr{"that file could not be opened"}
		}
		xmlData = nil
		for _, f := range zr.File {
			if f.Name == "Home.xml" {
				rc, err := f.Open()
				if err != nil {
					return nil, err
				}
				xmlData, err = io.ReadAll(io.LimitReader(rc, 64<<20))
				rc.Close()
				if err != nil {
					return nil, err
				}
			}
		}
		if xmlData == nil {
			return nil, &userErr{"this Sweet Home 3D file is too old (no Home.xml inside): open it in Sweet Home 3D 5.3 or newer and save it again"}
		}
	}
	var h sh3dHome
	if err := xml.Unmarshal(xmlData, &h); err != nil {
		return nil, &userErr{"that Sweet Home 3D file could not be read"}
	}
	for _, g := range h.Groups {
		h.Furniture = append(h.Furniture, g.Furniture...)
		h.Doors = append(h.Doors, g.Doors...)
	}
	st := s.settingsOf(ctx, uid)
	name := firstNonEmpty(strings.TrimSuffix(strings.TrimSuffix(fname, ".sh3d"), ".xml"), h.Name, "Sweet Home 3D plan")
	p := &store.Plan{UserID: uid, Name: clip(name, maxName), Kind: "apartment", Status: "living", Currency: firstNonEmpty(st.Currency, "EUR"),
		Notes: "Brought in from Sweet Home 3D on " + today() + "."}
	if err := s.store.CreatePlan(ctx, p); err != nil {
		return nil, err
	}
	levels := []string{""}
	lname := map[string]string{"": "Ground floor"}
	lceil := map[string]float64{"": firstPos(st.CeilingCm, 250)}
	if len(h.Levels) > 0 {
		levels = nil
		for _, l := range h.Levels {
			levels = append(levels, l.ID)
			lname[l.ID] = firstNonEmpty(l.Name, "Floor")
			lceil[l.ID] = firstPos(l.Height, st.CeilingCm, 250)
		}
	}
	counts := map[string]int{}
	for i, lv := range levels {
		b := newDrawer()
		for _, rm := range h.Rooms {
			if rm.Level != lv || len(rm.Points) < 3 {
				continue
			}
			pts := []store.Pt{}
			for _, pt := range rm.Points {
				pts = append(pts, store.Pt{X: math.Round(pt.X*10) / 10, Y: math.Round(pt.Y*10) / 10})
			}
			b.d.Rooms = append(b.d.Rooms, store.DRoom{ID: b.id("r"), Name: firstNonEmpty(rm.Name, "Room"), Kind: guessRoomKind(rm.Name), Points: pts, Wall: 0.01, Floor: "wood"})
			counts["rooms"]++
		}
		// Sweet Home 3D walls are drawn as they are (rooms then carry no wall of their own)
		for _, wl := range h.Walls {
			if wl.Level != lv {
				continue
			}
			b.d.Walls = append(b.d.Walls, store.DWall{ID: b.id("w"), A: store.Pt{X: wl.XStart, Y: wl.YStart}, B: store.Pt{X: wl.XEnd, Y: wl.YEnd}, Thick: firstPos(wl.Thickness, 10)})
			counts["walls"]++
		}
		for _, dw := range h.Doors {
			if dw.Level != lv || dw.Visible == "false" {
				continue
			}
			b.sh3dOpening(dw)
			counts["openings"]++
		}
		for _, fp := range h.Furniture {
			if fp.Level != lv || fp.Visible == "false" || fp.Width <= 0 || fp.Depth <= 0 {
				continue
			}
			it := store.DItem{ID: b.id("i"), Name: clip(fp.Name, maxName), Type: guessType(fp.Name + " " + fp.Catalog), X: fp.X, Y: fp.Y, W: fp.Width, D: fp.Depth, H: fp.Height,
				Rot: math.Mod(fp.Angle*180/math.Pi+360, 360), Elev: fp.Elevation}
			if c, err := strconv.ParseInt(fp.Color, 10, 64); err == nil && c != 0 {
				it.Color = fmt.Sprintf("#%06x", c&0xffffff)
			}
			b.d.Items = append(b.d.Items, it)
			counts["pieces"]++
		}
		if _, err := s.addFloor(ctx, uid, p.ID, lname[lv], i, lceil[lv], b.d); err != nil {
			return nil, err
		}
	}
	return map[string]any{"ok": true, "plan_id": p.ID, "added": counts}, nil
}

// sh3dOpening puts a Sweet Home 3D door or window on the nearest room edge.
func (b *drawer) sh3dOpening(dw sh3dPiece) {
	best, bestRoom, bestEdge, bestAt := math.Inf(1), "", 0, 0.0
	for _, r := range b.d.Rooms {
		n := len(r.Points)
		for i := 0; i < n; i++ {
			a, c := r.Points[i], r.Points[(i+1)%n]
			dx, dy := c.X-a.X, c.Y-a.Y
			l2 := dx*dx + dy*dy
			if l2 == 0 {
				continue
			}
			t := clampF(((dw.X-a.X)*dx+(dw.Y-a.Y)*dy)/l2, 0, 1)
			px, py := a.X+t*dx, a.Y+t*dy
			if d := math.Hypot(dw.X-px, dw.Y-py); d < best {
				best, bestRoom, bestEdge, bestAt = d, r.ID, i, t*math.Sqrt(l2)
			}
		}
	}
	kind := "door"
	low := strings.ToLower(dw.Name + " " + dw.Catalog)
	switch {
	case strings.Contains(low, "window") || dw.Elevation > 40:
		kind = "window"
	case strings.Contains(low, "sliding"):
		kind = "sliding"
	case strings.Contains(low, "double"):
		kind = "double"
	case strings.Contains(low, "garage"):
		kind = "garage"
	}
	if bestRoom == "" || best > 60 {
		return
	}
	b.d.Openings = append(b.d.Openings, store.DOpening{ID: b.id("o"), Kind: kind, Room: bestRoom, Edge: bestEdge, At: bestAt, Width: dw.Width, Height: dw.Height, Sill: dw.Elevation, Hinge: "start"})
}

func guessRoomKind(name string) string {
	n := strings.ToLower(name)
	for _, k := range []struct{ w, k string }{{"bath", "bath"}, {"wc", "toilet"}, {"toilet", "toilet"}, {"kitchen", "kitchen"}, {"bed", "bedroom"}, {"living", "living"},
		{"lounge", "living"}, {"dining", "dining"}, {"hall", "hall"}, {"entr", "hall"}, {"corridor", "hall"}, {"office", "office"}, {"study", "office"},
		{"kid", "kids"}, {"child", "kids"}, {"nursery", "kids"}, {"storage", "storage"}, {"closet", "storage"}, {"laundry", "laundry"}, {"utility", "laundry"},
		{"balcon", "balcony"}, {"terrace", "balcony"}, {"patio", "balcony"}, {"garage", "garage"}, {"garden", "garden"}, {"stair", "stairs"}} {
		if strings.Contains(n, k.w) {
			return k.k
		}
	}
	return "other"
}

// guessType picks a drawing for a piece from its name.
func guessType(name string) string {
	n := strings.ToLower(name)
	for _, k := range []struct{ w, t string }{{"corner sofa", "sofa_l"}, {"sofa", "sofa"}, {"couch", "sofa"}, {"armchair", "armchair"}, {"double bed", "bed_double"},
		{"king", "bed_double"}, {"single bed", "bed_single"}, {"bunk", "bed_single"}, {"crib", "crib"}, {"cot", "crib"}, {"bed", "bed_double"},
		{"bedside", "nightstand"}, {"night", "nightstand"}, {"wardrobe", "wardrobe"}, {"closet", "wardrobe"}, {"dresser", "dresser"}, {"chest", "dresser"},
		{"round table", "table_round"}, {"coffee", "coffee_table"}, {"dining", "dining_table"}, {"table", "dining_table"}, {"desk", "desk"},
		{"office chair", "office_chair"}, {"chair", "chair"}, {"stool", "chair"}, {"bench", "bench"}, {"bookcase", "bookcase"}, {"bookshelf", "bookcase"},
		{"shelf", "shelf"}, {"tv", "tv_unit"}, {"television", "tv_unit"}, {"fridge", "fridge"}, {"refrigerator", "fridge"}, {"dishwasher", "dishwasher"},
		{"washing", "washer"}, {"washer", "washer"}, {"dryer", "washer"}, {"oven", "stove"}, {"cooker", "stove"}, {"stove", "stove"}, {"hob", "stove"},
		{"sink", "sink_unit"}, {"counter", "counter"}, {"cabinet", "counter"}, {"toilet", "toilet"}, {"wc", "toilet"}, {"bath", "bathtub"}, {"tub", "bathtub"},
		{"shower", "shower"}, {"basin", "basin"}, {"washbasin", "basin"}, {"plant", "plant"}, {"rug", "rug"}, {"carpet", "rug"}, {"lamp", "lamp"},
		{"piano", "piano"}, {"radiator", "radiator"}, {"fireplace", "fireplace"}, {"stair", "stairs"}, {"car", "car"}, {"bike", "bike"}, {"bicycle", "bike"}} {
		if strings.Contains(n, k.w) {
			return k.t
		}
	}
	return "box"
}
