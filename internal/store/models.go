package store

import (
	"encoding/json"
	"strings"
)

// User is an account on this box.
type User struct {
	ID           int64  `json:"id"`
	Email        string `json:"email"`
	Name         string `json:"name"`
	PasswordHash string `json:"-"`
	Role         string `json:"role"` // admin | member
	CreatedAt    string `json:"created_at"`
}

// APIToken is a personal access token (hash stored, plaintext shown once).
type APIToken struct {
	ID         int64   `json:"id"`
	UserID     int64   `json:"user_id"`
	Name       string  `json:"name"`
	Prefix     string  `json:"prefix"`
	LastUsedAt *string `json:"last_used_at"`
	CreatedAt  string  `json:"created_at"`
}

// Photo is a picture: a room, a floor plan to trace over, a piece of furniture.
type Photo struct {
	ID        int64  `json:"id"`
	UserID    int64  `json:"-"`
	Kind      string `json:"kind"`
	Mime      string `json:"mime"`
	Data      []byte `json:"-"`
	Thumb     []byte `json:"-"`
	Width     int    `json:"width"`
	Height    int    `json:"height"`
	Bytes     int    `json:"bytes"`
	CreatedAt string `json:"created_at"`
}

// Plan is a flat, a house, an office or a garden.
type Plan struct {
	ID         int64  `json:"id"`
	UserID     int64  `json:"-"`
	Name       string `json:"name"`
	Kind       string `json:"kind"`
	Address    string `json:"address"`
	Currency   string `json:"currency"`
	Notes      string `json:"notes"`
	Status     string `json:"status"`
	HomeID     *int64 `json:"home_id"`
	HomeName   string `json:"home_name"`
	RentalID   *int64 `json:"rental_id"`
	RentalName string `json:"rental_name"`
	ShareToken string `json:"share_token"`
	ShareOn    bool   `json:"share_on"`
	Pinned     bool   `json:"pinned"`
	Archived   bool   `json:"archived"`
	Source     string `json:"source"`
	CreatedAt  string `json:"created_at"`
	UpdatedAt  string `json:"updated_at"`

	// worked out on read
	Floors   []Floor `json:"floors,omitempty"`
	AreaCm2  float64 `json:"area_cm2"` // the current layouts' rooms, every floor
	RoomsN   int     `json:"rooms_n"`
	ItemsN   int     `json:"items_n"`
	Budget   int64   `json:"budget"` // pieces still to buy, current layouts
	OpenTodo int     `json:"open_todos"`
	Preview  *Layout `json:"preview,omitempty"` // the first floor's current layout (for the card)
}

// Floor is a storey of a plan.
type Floor struct {
	ID              int64           `json:"id"`
	UserID          int64           `json:"-"`
	PlanID          int64           `json:"plan_id"`
	Name            string          `json:"name"`
	Level           int             `json:"level"`
	CeilingCm       float64         `json:"ceiling_cm"`
	Sort            int             `json:"sort"`
	CurrentLayoutID *int64          `json:"current_layout_id"`
	UnderlayPhotoID *int64          `json:"underlay_photo_id"`
	Underlay        json.RawMessage `json:"underlay"`
	CreatedAt       string          `json:"created_at"`

	Layouts []Layout `json:"layouts,omitempty"`
}

// Layout is one arrangement of a floor; Data is the drawing (see Drawing).
type Layout struct {
	ID        int64           `json:"id"`
	UserID    int64           `json:"-"`
	PlanID    int64           `json:"plan_id"`
	FloorID   int64           `json:"floor_id"`
	Name      string          `json:"name"`
	Notes     string          `json:"notes"`
	Data      json.RawMessage `json:"data,omitempty"`
	AreaCm2   float64         `json:"area_cm2"`
	RoomsN    int             `json:"rooms_n"`
	ItemsN    int             `json:"items_n"`
	Budget    int64           `json:"budget"`
	Sort      int             `json:"sort"`
	Revision  int             `json:"revision"`
	CreatedAt string          `json:"created_at"`
	UpdatedAt string          `json:"updated_at"`

	Current bool `json:"current"`
}

// Piece is a piece of furniture you own or are thinking of buying.
type Piece struct {
	ID            int64   `json:"id"`
	UserID        int64   `json:"-"`
	Name          string  `json:"name"`
	Type          string  `json:"type"`
	Category      string  `json:"category"`
	Width         float64 `json:"width"`
	Depth         float64 `json:"depth"`
	Height        float64 `json:"height"`
	Color         string  `json:"color"`
	Status        string  `json:"status"`
	Price         *int64  `json:"price"`
	Currency      string  `json:"currency"`
	URL           string  `json:"url"`
	Shop          string  `json:"shop"`
	Notes         string  `json:"notes"`
	PhotoID       *int64  `json:"photo_id"`
	InventoryID   *int64  `json:"inventory_id"`
	InventoryName string  `json:"inventory_name"`
	WishID        *int64  `json:"wish_id"`
	WishName      string  `json:"wish_name"`
	MoneyTxID     string  `json:"money_tx_id"`
	Source        string  `json:"source"`
	CreatedAt     string  `json:"created_at"`
	UpdatedAt     string  `json:"updated_at"`

	PhotoURL string   `json:"photo_url"`
	PlacedIn []string `json:"placed_in"` // "Flat · Living room" where it stands on a current layout
}

// RouteStep is one thing a piece has to pass on the way in.
type RouteStep struct {
	Kind   string  `json:"kind"` // door | hall | corner | stairs | lift
	Name   string  `json:"name"`
	Width  float64 `json:"width"`            // the door's clear width, the hallway's width
	Height float64 `json:"height,omitempty"` // the door's clear height, the lift's or stairs' headroom
	Turn   float64 `json:"turn,omitempty"`   // corner / door off a hallway: the width of the hallway you turn out of
	Depth  float64 `json:"depth,omitempty"`  // a lift's depth
}

// Route is the way in to a plan (or one of its rooms).
type Route struct {
	ID        int64       `json:"id"`
	UserID    int64       `json:"-"`
	PlanID    int64       `json:"plan_id"`
	Name      string      `json:"name"`
	Steps     []RouteStep `json:"steps"`
	Sort      int         `json:"sort"`
	CreatedAt string      `json:"created_at"`
	UpdatedAt string      `json:"updated_at"`
}

// Todo is one thing on a plan's checklist.
type Todo struct {
	ID        int64  `json:"id"`
	UserID    int64  `json:"-"`
	PlanID    int64  `json:"plan_id"`
	Title     string `json:"title"`
	Done      bool   `json:"done"`
	Due       string `json:"due"`
	TaskID    string `json:"task_id"`
	TaskURL   string `json:"task_url"`
	Sort      int    `json:"sort"`
	Source    string `json:"source"`
	DoneAt    string `json:"done_at"`
	CreatedAt string `json:"created_at"`

	PlanName string `json:"plan_name,omitempty"`
}

// PlanPhoto is a photo of one of a plan's rooms.
type PlanPhoto struct {
	ID        int64  `json:"id"`
	UserID    int64  `json:"-"`
	PlanID    int64  `json:"plan_id"`
	PhotoID   *int64 `json:"photo_id"`
	Room      string `json:"room"`
	Caption   string `json:"caption"`
	PocketID  string `json:"pocket_id"`
	PocketURL string `json:"pocket_url"`
	TakenOn   string `json:"taken_on"`
	CreatedAt string `json:"created_at"`

	URL      string `json:"url"`
	ThumbURL string `json:"thumb_url"`
}

// Allowed values.
var (
	PlanKinds     = []string{"apartment", "house", "office", "studio", "garden", "other"}
	PlanStates    = []string{"living", "moving", "idea", "done"}
	PieceStates   = []string{"own", "want", "maybe", "gone"}
	Categories    = []string{"living", "sleeping", "dining", "kitchen", "bath", "work", "storage", "kids", "outdoor", "other"}
	StepKinds     = []string{"door", "hall", "corner", "stairs", "lift"}
	PhotoKinds    = []string{"room", "underlay", "piece"}
	OpeningKinds  = []string{"door", "double", "sliding", "pocket", "window", "opening", "garage"}
	RoomKinds     = []string{"living", "bedroom", "kitchen", "dining", "bath", "toilet", "hall", "office", "kids", "storage", "laundry", "balcony", "garage", "garden", "stairs", "other"}
	FloorFinishes = []string{"", "wood", "tile", "carpet", "stone", "concrete", "vinyl", "grass", "deck"}
)

// Conn is one person's connection to a sibling Pocket app.
type Conn struct {
	Mode      string `json:"mode,omitempty"`
	URL       string `json:"url,omitempty"`
	Token     string `json:"token,omitempty"`
	PublicURL string `json:"public_url,omitempty"`
}

// Settings are the box-wide values plus one person's preferences and their
// own connections.
type Settings struct {
	Title         string `json:"title"`
	SetupComplete bool   `json:"setup_complete"`

	DateFormat string `json:"date_format"`
	TimeFormat string `json:"time_format"`
	WeekStart  int    `json:"week_start"` // 1 = Monday, 7 = Sunday

	// Drawing.
	Units     string  `json:"units"`      // metric | imperial
	Snap      float64 `json:"snap"`       // grid step in cm (0 = off)
	WallCm    float64 `json:"wall_cm"`    // a new room's walls
	CeilingCm float64 `json:"ceiling_cm"` // a new floor's ceiling height
	DoorCm    float64 `json:"door_cm"`    // a new door's width
	Currency  string  `json:"currency"`
	HandsCm   float64 `json:"hands_cm"`  // room to leave for hands and blankets when checking a fit
	ShowDims  bool    `json:"show_dims"` // lengths on walls
	Clearance bool    `json:"clearance"` // check the room in front of beds, wardrobes, doors

	// Other apps.
	NotesFolder   string `json:"notes_folder"`   // where plan summaries go in Pocket Notes
	FilesFolder   string `json:"files_folder"`   // where pictures of plans go in Pocket Files
	WishlistID    int64  `json:"wishlist_id"`    // Pocket Wishlist list for furniture (0 = the default one)
	MoneyAccount  string `json:"money_account"`  // Pocket Money account furniture is paid from
	MoneyCategory string `json:"money_category"` // and its category
	ChatTo        string `json:"chat_to"`        // the household's Pocket Chat conversation
	AIHelp        bool   `json:"ai_help"`
	AIPhotos      bool   `json:"ai_photos"` // the AI may look at pictures of floor plans

	// Notifications.
	NotifyTodos  bool   `json:"notify_todos"` // a checklist item is due tomorrow
	NotifyHour   int    `json:"notify_hour"`
	NotifyURL    string `json:"notify_url"`
	NotifyFormat string `json:"notify_format"` // apprise | ntfy | json

	// Each sibling Pocket app, by slug.
	Conns map[string]Conn `json:"conns"`

	// AI: Pocket AI through Core, or an OpenAI-compatible API with the
	// person's own key.
	AIProvider    string `json:"ai_provider"` // auto | pocket | openai
	AIPocketModel string `json:"ai_pocket_model"`
	OpenAIKey     string `json:"openai_api_key"`
	OpenAIModel   string `json:"openai_model"`
	OpenAIBaseURL string `json:"openai_base_url"`
}

func toJSON(v any) string {
	b, _ := json.Marshal(v)
	return string(b)
}

// CleanTags trims, de-duplicates and caps a list of tags.
func CleanTags(in []string) []string {
	out := []string{}
	seen := map[string]bool{}
	for _, t := range in {
		t = strings.TrimSpace(strings.TrimPrefix(t, "#"))
		k := strings.ToLower(t)
		if t == "" || seen[k] || len(out) >= 20 {
			continue
		}
		seen[k] = true
		out = append(out, t)
	}
	return out
}
