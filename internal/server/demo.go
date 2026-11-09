package server

import (
	"context"
	"encoding/json"

	"github.com/abjelosevic88/pocket-homelab-rooms/internal/store"
)

// seedDemo fills the owner's account with three plans — a furnished
// two-bedroom flat with two layouts and the way in, a two-storey house
// being moved into, and a studio idea — plus measured furniture (owned and
// wanted) and checklists: for `make run`, the CI smoke test and the
// screenshots.
func (s *Server) seedDemo(ctx context.Context, uid int64) error {
	ctx = context.WithoutCancel(ctx) // with AUTH_MODE=none this runs inside the first request
	if list, _ := s.store.ListPlans(ctx, uid, false); len(list) > 0 {
		return nil
	}
	day := func(n int) string { return store.AddDays(today(), n) }
	price := func(v int64) *int64 { return &v }

	// ---- furniture, measured ----
	mk := func(name, typ, cat string, w, d, h float64, status string, pr *int64, shop, notes string) int64 {
		p := &store.Piece{UserID: uid, Name: name, Type: typ, Category: cat, Width: w, Depth: d, Height: h, Status: status, Price: pr, Currency: "EUR", Shop: shop, Notes: notes}
		_ = s.store.CreatePiece(ctx, p)
		return p.ID
	}
	sofa := mk("Grey three-seater", "sofa", "living", 220, 95, 85, "own", price(64900), "", "Bought 2021. The legs screw off (−12 cm height).")
	table := mk("Oak dining table", "dining_table", "dining", 140, 80, 75, "own", price(45000), "", "Extends to 200 cm.")
	chair := mk("Dining chair", "chair", "dining", 45, 52, 82, "own", price(7900), "", "We have four.")
	bed := mk("Our bed (160)", "bed_double", "sleeping", 165, 210, 100, "own", price(79000), "", "Frame with headboard; the mattress is 160 × 200.")
	wardrobe := mk("Wardrobe, two doors", "wardrobe", "storage", 150, 60, 220, "own", nil, "", "Doors swing out: needs about 70 cm in front.")
	kidbed := mk("Mia's bed", "bed_single", "kids", 95, 205, 60, "own", nil, "", "")
	desk := mk("White desk", "desk", "work", 100, 55, 75, "own", price(12900), "", "")
	tv := mk("TV bench", "tv_unit", "living", 160, 40, 50, "own", nil, "", "55-inch TV on top.")
	books := mk("Bookcase", "bookcase", "storage", 100, 30, 200, "own", price(8900), "", "")
	arm := mk("Green armchair", "armchair", "living", 80, 85, 95, "own", nil, "", "From grandma.")
	fridge := mk("Fridge-freezer", "fridge", "kitchen", 60, 65, 185, "own", nil, "", "")
	corner := mk("Corner sofa Lund", "sofa_l", "living", 270, 180, 82, "want", price(129900), "Nordhaus", "Comes in two parts? Ask the shop — in one piece it does not make the hall corner.")
	reading := mk("Reading chair", "armchair", "living", 75, 80, 100, "want", price(34900), "Nordhaus", "")
	bunk := mk("Bunk bed", "bed_single", "kids", 97, 209, 160, "maybe", price(42900), "", "If Mia's cousin stays over more often.")
	piano := mk("Upright piano", "piano", "living", 150, 60, 125, "maybe", price(180000), "", "Second-hand, 230 kg: needs the lift.")
	_ = bunk
	_ = piano

	// ---- Our flat: two bedrooms, about 72 m² ----
	b := newDrawer()
	liv := b.room("Living room & kitchen", "living", 0, 0, 520, 600, "wood")
	hall := b.id("r")
	b.d.Rooms = append(b.d.Rooms, store.DRoom{ID: hall, Name: "Hall", Kind: "hall", Floor: "tile", Wall: 12,
		Points: []store.Pt{{X: 532, Y: 272}, {X: 1044, Y: 272}, {X: 1044, Y: 392}, {X: 672, Y: 392}, {X: 672, Y: 600}, {X: 532, Y: 600}}})
	bath := b.room("Bathroom", "bath", 532, 0, 200, 260, "tile")
	kids := b.room("Mia's room", "kids", 744, 0, 300, 260, "wood")
	bedr := b.room("Bedroom", "bedroom", 684, 404, 360, 366, "wood")
	balc := b.room("Balcony", "balcony", 60, -152, 400, 140, "deck", 0, 1, 3)
	_ = balc
	b.window(liv, 3, 150, 140)
	b.window(liv, 3, 420, 140)
	b.door(liv, 0, 400, 180, "start", false, "sliding")
	b.door(liv, 1, 540, 90, "end", false)
	b.door(hall, 4, 70, 90, "start", false)
	b.d.Openings[len(b.d.Openings)-1].Name = "Front door"
	b.door(bath, 2, 130, 70, "start", false)
	b.window(bath, 0, 100, 60)
	b.door(kids, 2, 60, 80, "end", false)
	b.window(kids, 0, 150, 120)
	b.door(bedr, 0, 60, 80, "start", false)
	b.window(bedr, 1, 190, 140)
	// kitchen along the top wall
	b.item("fridge", "Fridge-freezer", 30, 33, 60, 65, 185, 0, piece(fridge))
	b.item("sink_unit", "Sink", 100, 30, 80, 60, 90, 0)
	b.item("dishwasher", "Dishwasher", 170, 30, 60, 60, 85, 0)
	b.item("stove", "Cooker", 230, 30, 60, 60, 90, 0)
	b.item("counter", "Worktop", 280, 30, 40, 60, 90, 0)
	b.item("counter", "Wall cupboards", 175, 18, 230, 35, 70, 0, func(it *store.DItem) { it.Elev = 150 })
	// dining
	b.item("dining_table", "Oak dining table", 180, 250, 140, 80, 75, 0, piece(table))
	b.item("chair", "Dining chair", 145, 188, 45, 52, 82, 0, piece(chair))
	b.item("chair", "Dining chair", 215, 188, 45, 52, 82, 0, piece(chair))
	b.item("chair", "Dining chair", 145, 312, 45, 52, 82, 180, piece(chair))
	b.item("chair", "Dining chair", 215, 312, 45, 52, 82, 180, piece(chair))
	// sitting
	b.item("rug", "Rug", 200, 450, 170, 240, 1, 0, color("#c9b79c"))
	b.item("sofa", "Grey three-seater", 48, 450, 220, 95, 85, 270, piece(sofa))
	b.item("coffee_table", "Coffee table", 175, 450, 60, 110, 40, 0)
	b.item("tv_unit", "TV bench", 500, 390, 160, 40, 50, 90, piece(tv))
	b.item("armchair", "Green armchair", 310, 360, 80, 85, 95, 225, piece(arm), color("#6f8f5a"))
	b.item("bookcase", "Bookcase", 330, 585, 100, 30, 200, 180, piece(books))
	b.item("plant", "Monstera", 490, 35, 50, 50, 120, 0)
	b.item("plant", "Fig tree", 30, 300, 45, 45, 160, 0)
	// bedroom
	b.item("bed_double", "Our bed (160)", 789, 620, 165, 210, 100, 270, piece(bed))
	b.item("nightstand", "Night table", 706, 727, 45, 40, 55, 270)
	b.item("wardrobe", "Wardrobe, two doors", 969, 434, 150, 60, 220, 0, piece(wardrobe))
	b.item("dresser", "Chest of drawers", 1022, 725, 80, 45, 85, 90)
	// Mia's room
	b.item("bed_single", "Mia's bed", 792, 103, 95, 205, 60, 0, piece(kidbed))
	b.item("desk", "White desk", 950, 28, 100, 55, 75, 0, piece(desk))
	b.item("office_chair", "Desk chair", 950, 90, 50, 50, 90, 0)
	b.item("shelf", "Toy shelf", 1022, 120, 80, 45, 90, 90)
	// bathroom
	b.item("bathtub", "Bath", 694, 85, 170, 75, 60, 90)
	b.item("basin", "Basin", 555, 45, 60, 45, 85, 270)
	b.item("toilet", "Toilet", 564, 130, 40, 65, 80, 270)
	b.item("washer", "Washing machine", 702, 225, 60, 60, 85, 90)
	// hall
	b.item("wardrobe", "Hall cupboard", 1024, 332, 100, 40, 210, 90)
	b.item("shelf", "Shoe cabinet", 547, 450, 80, 30, 100, 270)
	b.d.Dims = append(b.d.Dims, store.DDim{ID: b.id("m"), A: store.Pt{X: 0, Y: 790}, B: store.Pt{X: 1044, Y: 790}, Off: 0})
	b.d.Labels = append(b.d.Labels, store.DLabel{ID: b.id("t"), At: store.Pt{X: 260, Y: -175}, Text: "Morning sun", Size: 14})
	flatA := b.d

	flat := &store.Plan{UserID: uid, Name: "Our flat", Kind: "apartment", Address: "Kralja Tvrtka 14, 2nd floor", Currency: "EUR", Status: "living", Pinned: true,
		Notes: "Rented since 2022. The landlord is fine with drilling in the living room, ask before the bedroom.\nCeilings are 2.62 m; the radiators are under the windows."}
	if err := s.store.CreatePlan(ctx, flat); err != nil {
		return err
	}
	ff, err := s.addFloor(ctx, uid, flat.ID, "The flat", 0, 262, flatA)
	if err != nil {
		return err
	}
	// Option B: a corner sofa and a reading corner
	b2 := &drawer{d: cloneDrawing(flatA), n: b.n}
	keep := b2.d.Items[:0]
	for _, it := range b2.d.Items {
		if it.PieceID == sofa || it.PieceID == arm || it.Name == "Coffee table" || it.Name == "Rug" {
			continue
		}
		keep = append(keep, it)
	}
	b2.d.Items = keep
	b2.item("sofa_l", "Corner sofa Lund", 135, 510, 270, 180, 82, 180, piece(corner), buy(129900))
	b2.item("coffee_table", "Coffee table", 190, 455, 70, 60, 42, 0, buy(14900))
	b2.item("armchair", "Reading chair", 455, 120, 75, 80, 100, 0, piece(reading), buy(34900))
	b2.item("lamp", "Floor lamp", 380, 110, 35, 35, 160, 0, buy(8900))
	for i := range b2.d.Items {
		if b2.d.Items[i].PieceID == books {
			b2.d.Items[i].X, b2.d.Items[i].Y = 445, 585 // in front of the door: the checks will say so
		}
	}
	optB := &store.Layout{UserID: uid, PlanID: flat.ID, FloorID: ff.ID, Name: "Option B: corner sofa", Data: mustJSON(b2.d), Sort: 1,
		Notes: "The corner sofa faces the TV; the reading chair goes by the balcony door. The bookcase has to move somewhere."}
	_ = s.store.CreateLayout(ctx, optB)
	// the way in
	_ = s.store.SaveRoute(ctx, &store.Route{UserID: uid, PlanID: flat.ID, Name: "Up the stairs", Steps: []store.RouteStep{
		{Kind: "door", Name: "Building door", Width: 110, Height: 210},
		{Kind: "stairs", Name: "Stairs to the 2nd floor", Width: 105, Height: 210, Turn: 105},
		{Kind: "door", Name: "Front door", Width: 86, Height: 203, Turn: 140},
		{Kind: "corner", Name: "Hall corner", Width: 120, Turn: 140},
		{Kind: "door", Name: "Bedroom door", Width: 76, Height: 200, Turn: 120},
	}})
	_ = s.store.SaveRoute(ctx, &store.Route{UserID: uid, PlanID: flat.ID, Name: "With the lift", Sort: 1, Steps: []store.RouteStep{
		{Kind: "door", Name: "Building door", Width: 110, Height: 210},
		{Kind: "lift", Name: "Lift", Width: 100, Depth: 130, Height: 210},
		{Kind: "door", Name: "Front door", Width: 86, Height: 203, Turn: 140},
	}})
	for _, t := range []store.Todo{
		{Title: "Measure the alcove next to the balcony door", Done: true, DoneAt: store.Now()},
		{Title: "Ask the landlord about drilling in the bedroom", Due: day(1)},
		{Title: "Ask Nordhaus whether the corner sofa comes in two parts", Due: day(4)},
		{Title: "Pick a paint colour for Mia's room", Due: day(12)},
		{Title: "Sell the green armchair (if we take option B)"},
	} {
		t.UserID, t.PlanID = uid, flat.ID
		_ = s.store.SaveTodo(ctx, &t)
	}

	// ---- The lake house: two floors, moving in ----
	house := &store.Plan{UserID: uid, Name: "Lake house", Kind: "house", Address: "Obala 7, Jajce", Currency: "EUR", Status: "moving",
		Notes: "Keys on the 1st of next month. The staircase turns at the top: measure the headroom."}
	if err := s.store.CreatePlan(ctx, house); err != nil {
		return err
	}
	gd := templateDrawing("house", 0, 0)
	hb := &drawer{d: gd, n: 100}
	hb.item("sofa", "Grey three-seater", 300, 420, 220, 95, 85, 180, piece(sofa))
	hb.item("tv_unit", "TV bench", 300, 40, 160, 40, 50, 0, piece(tv))
	hb.item("dining_table", "Long table (to buy)", 814, 330, 200, 95, 75, 90, buy(89000))
	hb.item("counter", "Kitchen run", 812, 31, 400, 62, 90, 0)
	hb.item("fridge", "Fridge-freezer", 999.5, 230, 60, 65, 185, 90, piece(fridge))
	hb.item("desk", "White desk", 742, 804, 100, 55, 75, 180, piece(desk))
	hb.item("bookcase", "Bookcase", 857, 650, 100, 30, 200, 90, piece(books))
	hb.item("bench", "Garden bench (to buy)", 300, -300, 150, 60, 85, 0, buy(24900))
	hb.item("toilet", "Toilet", 958, 565, 40, 65, 80, 0)
	hb.item("plant", "Olive tree", 60, -60, 60, 60, 150, 0)
	gf, err := s.addFloor(ctx, uid, house.ID, "Ground floor", 0, 255, hb.d)
	if err != nil {
		return err
	}
	_ = gf
	ub := newDrawer()
	m1 := ub.room("Main bedroom", "bedroom", 0, 0, 420, 400, "wood")
	ub.window(m1, 0, 210, 160)
	k1 := ub.room("Mia's room", "kids", 432, 0, 320, 400, "wood")
	ub.window(k1, 0, 160, 120)
	g1 := ub.room("Guest room", "bedroom", 764, 0, 260, 400, "wood")
	ub.window(g1, 1, 200, 100)
	l1 := ub.room("Landing", "hall", 0, 412, 640, 140, "wood")
	ub.door(l1, 0, 520, 80, "end", false)
	ub.door(m1, 2, 80, 80, "start", false)
	ub.door(k1, 2, 280, 80, "end", false)
	ba := ub.room("Bathroom", "bath", 652, 412, 372, 220, "tile")
	ub.door(ba, 3, 160, 80, "start", false)
	ub.window(ba, 1, 110, 80)
	ub.door(g1, 2, 60, 80, "end", false)
	ub.item("stairs", "Stairs", 120, 482, 220, 100, 0, 0)
	ub.item("bed_double", "Our bed (160)", 215, 190, 165, 210, 100, 0, piece(bed))
	ub.item("wardrobe", "Wardrobe, two doors", 30, 300, 150, 60, 220, 270, piece(wardrobe))
	ub.item("bed_single", "Mia's bed", 482, 200, 95, 205, 60, 0, piece(kidbed))
	ub.item("bed_single", "Guest bed (to buy)", 860, 200, 90, 200, 45, 0, buy(29900))
	ub.item("bathtub", "Bath", 760, 590, 170, 75, 60, 180)
	ub.item("basin", "Double basin", 960, 440, 120, 50, 85, 0)
	ub.item("toilet", "Toilet", 991, 600, 40, 65, 80, 90)
	if _, err := s.addFloor(ctx, uid, house.ID, "First floor", 1, 245, ub.d); err != nil {
		return err
	}
	_ = s.store.SaveRoute(ctx, &store.Route{UserID: uid, PlanID: house.ID, Name: "Upstairs", Steps: []store.RouteStep{
		{Kind: "door", Name: "Front door", Width: 96, Height: 210},
		{Kind: "corner", Name: "Into the staircase", Width: 95, Turn: 120},
		{Kind: "stairs", Name: "Stairs (turn at the top)", Width: 95, Height: 195, Turn: 100},
		{Kind: "door", Name: "Bedroom door", Width: 76, Height: 200, Turn: 140},
	}})
	for _, t := range []store.Todo{
		{Title: "Book the movers", Due: day(9)},
		{Title: "Measure the headroom where the stairs turn", Due: day(3)},
		{Title: "Order the long table", Due: day(20)},
		{Title: "Take photos of every room before we move in"},
	} {
		t.UserID, t.PlanID = uid, house.ID
		_ = s.store.SaveTodo(ctx, &t)
	}

	// ---- an idea: a studio for Mia, some day ----
	studio := &store.Plan{UserID: uid, Name: "Studio idea", Kind: "studio", Currency: "EUR", Status: "idea", Notes: "What a 34 m² studio could hold."}
	if err := s.store.CreatePlan(ctx, studio); err != nil {
		return err
	}
	sb := &drawer{d: templateDrawing("studio", 0, 0), n: 200}
	sb.item("bed_double", "Bed 140", 457.5, 280, 145, 205, 90, 90, buy(39900))
	sb.item("sofa", "Two-seater", 200, 360, 170, 85, 85, 180, buy(49900))
	sb.item("coffee_table", "Side table", 200, 280, 50, 50, 45, 0)
	sb.item("desk", "Desk", 70, 40, 120, 60, 75, 0, buy(15900))
	sb.item("wardrobe", "Wardrobe", 330, 30, 120, 60, 210, 0, buy(29900))
	if _, err := s.addFloor(ctx, uid, studio.ID, "The flat", 0, 250, sb.d); err != nil {
		return err
	}
	return nil
}

func cloneDrawing(d store.Drawing) store.Drawing {
	var out store.Drawing
	b, _ := json.Marshal(d)
	_ = json.Unmarshal(b, &out)
	out.Normalize()
	return out
}
