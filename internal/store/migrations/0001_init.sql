CREATE TABLE users (
  id            INTEGER PRIMARY KEY AUTOINCREMENT,
  email         TEXT NOT NULL UNIQUE,
  name          TEXT NOT NULL DEFAULT '',
  password_hash TEXT NOT NULL DEFAULT '',
  role          TEXT NOT NULL DEFAULT 'member',
  created_at    TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ','now')),
  updated_at    TEXT
);

CREATE TABLE sessions (
  id         TEXT PRIMARY KEY,
  user_id    INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  expires_at TEXT NOT NULL,
  created_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ','now'))
);

CREATE TABLE api_tokens (
  id           INTEGER PRIMARY KEY AUTOINCREMENT,
  user_id      INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  name         TEXT NOT NULL DEFAULT '',
  token_hash   TEXT NOT NULL UNIQUE,
  prefix       TEXT NOT NULL DEFAULT '',
  last_used_at TEXT,
  created_at   TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ','now'))
);

-- Box-wide settings (title, setup flag); everything else is personal.
CREATE TABLE settings (
  key   TEXT PRIMARY KEY,
  value TEXT NOT NULL DEFAULT ''
);

CREATE TABLE user_settings (
  user_id INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  key     TEXT NOT NULL,
  value   TEXT NOT NULL DEFAULT '',
  PRIMARY KEY (user_id, key)
);

CREATE TABLE cache (
  key        TEXT PRIMARY KEY,
  value      TEXT NOT NULL DEFAULT '',
  fetched_at TEXT NOT NULL
);


-- ---------- Pocket Rooms: everything below is per person ----------
-- Lengths are centimetres (REAL), money is minor units (cents) of the plan's
-- currency, dates YYYY-MM-DD.

-- Pictures: a room photo, a floor plan picture to trace over, a piece of furniture.
-- The browser shrinks them first (2000 px and a 480 px thumbnail, JPEG).
CREATE TABLE photos (
  id         INTEGER PRIMARY KEY AUTOINCREMENT,
  user_id    INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  kind       TEXT NOT NULL DEFAULT 'room',          -- room | underlay | piece
  mime       TEXT NOT NULL DEFAULT 'image/jpeg',
  data       BLOB NOT NULL,
  thumb      BLOB,
  width      INTEGER NOT NULL DEFAULT 0,
  height     INTEGER NOT NULL DEFAULT 0,
  bytes      INTEGER NOT NULL DEFAULT 0,
  created_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ','now'))
);
CREATE INDEX photos_user ON photos(user_id, kind);

-- A plan: a flat, a house, an office, a garden — one or more floors.
CREATE TABLE plans (
  id          INTEGER PRIMARY KEY AUTOINCREMENT,
  user_id     INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  name        TEXT NOT NULL,
  kind        TEXT NOT NULL DEFAULT 'apartment',     -- apartment | house | office | studio | garden | other
  address     TEXT NOT NULL DEFAULT '',
  currency    TEXT NOT NULL DEFAULT 'EUR',
  notes       TEXT NOT NULL DEFAULT '',
  status      TEXT NOT NULL DEFAULT 'living',        -- living (where I live) | moving (I am moving in) | idea | done
  home_id     INTEGER,                               -- the home in Pocket Home, if tied to one
  home_name   TEXT NOT NULL DEFAULT '',
  rental_id   INTEGER,                               -- the unit in Pocket Rentals, if tied to one
  rental_name TEXT NOT NULL DEFAULT '',
  share_token TEXT NOT NULL DEFAULT '',
  share_on    INTEGER NOT NULL DEFAULT 0,
  pinned      INTEGER NOT NULL DEFAULT 0,
  archived    INTEGER NOT NULL DEFAULT 0,
  source      TEXT NOT NULL DEFAULT '',              -- the app that added it through a link ('' = here)
  created_at  TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ','now')),
  updated_at  TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ','now'))
);
CREATE INDEX plans_user ON plans(user_id, archived, updated_at);
CREATE INDEX plans_share ON plans(share_token) WHERE share_token <> '';

-- A floor (storey) of a plan; its drawing lives in its layouts.
CREATE TABLE floors (
  id                INTEGER PRIMARY KEY AUTOINCREMENT,
  user_id           INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  plan_id           INTEGER NOT NULL REFERENCES plans(id) ON DELETE CASCADE,
  name              TEXT NOT NULL DEFAULT 'Ground floor',
  level             INTEGER NOT NULL DEFAULT 0,
  ceiling_cm        REAL NOT NULL DEFAULT 250,
  sort              INTEGER NOT NULL DEFAULT 0,
  current_layout_id INTEGER,                         -- the layout that is "how it is now"
  underlay_photo_id INTEGER REFERENCES photos(id) ON DELETE SET NULL,
  underlay          TEXT NOT NULL DEFAULT '{}',      -- {x, y, cm_per_px, opacity, rot, on}
  created_at        TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ','now'))
);
CREATE INDEX floors_user ON floors(user_id, plan_id, sort);

-- A layout is one way of arranging a floor: "As it is", "Option B: desk by
-- the window". Data is the whole drawing (rooms, walls, doors, windows,
-- furniture, measurements, labels); the columns are worked out from it.
CREATE TABLE layouts (
  id         INTEGER PRIMARY KEY AUTOINCREMENT,
  user_id    INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  plan_id    INTEGER NOT NULL REFERENCES plans(id) ON DELETE CASCADE,
  floor_id   INTEGER NOT NULL REFERENCES floors(id) ON DELETE CASCADE,
  name       TEXT NOT NULL DEFAULT 'As it is',
  notes      TEXT NOT NULL DEFAULT '',
  data       TEXT NOT NULL DEFAULT '{}',
  area_cm2   REAL NOT NULL DEFAULT 0,
  rooms_n    INTEGER NOT NULL DEFAULT 0,
  items_n    INTEGER NOT NULL DEFAULT 0,
  budget     INTEGER NOT NULL DEFAULT 0,             -- what the pieces still to buy cost
  sort       INTEGER NOT NULL DEFAULT 0,
  revision   INTEGER NOT NULL DEFAULT 1,             -- bumped on every save (stale writes are refused)
  created_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ','now')),
  updated_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ','now'))
);
CREATE INDEX layouts_user ON layouts(user_id, floor_id, sort);

-- A piece of furniture of your own (measured), or one you are thinking of buying.
CREATE TABLE pieces (
  id             INTEGER PRIMARY KEY AUTOINCREMENT,
  user_id        INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  name           TEXT NOT NULL,
  type           TEXT NOT NULL DEFAULT 'box',         -- a catalogue shape: sofa, bed, table_round…
  category       TEXT NOT NULL DEFAULT 'living',      -- living | sleeping | dining | kitchen | bath | work | storage | kids | outdoor | other
  width          REAL NOT NULL DEFAULT 100,           -- side to side
  depth          REAL NOT NULL DEFAULT 60,            -- front to back
  height         REAL NOT NULL DEFAULT 80,
  color          TEXT NOT NULL DEFAULT '',
  status         TEXT NOT NULL DEFAULT 'own',         -- own | want | maybe | gone
  price          INTEGER,                             -- minor units
  currency       TEXT NOT NULL DEFAULT 'EUR',
  url            TEXT NOT NULL DEFAULT '',
  shop           TEXT NOT NULL DEFAULT '',
  notes          TEXT NOT NULL DEFAULT '',
  photo_id       INTEGER REFERENCES photos(id) ON DELETE SET NULL,
  inventory_id   INTEGER,                             -- the item in Pocket Inventory
  inventory_name TEXT NOT NULL DEFAULT '',
  wish_id        INTEGER,                             -- the wish in Pocket Wishlist
  wish_name      TEXT NOT NULL DEFAULT '',
  money_tx_id    TEXT NOT NULL DEFAULT '',            -- the purchase booked in Pocket Money
  source         TEXT NOT NULL DEFAULT '',
  created_at     TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ','now')),
  updated_at     TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ','now'))
);
CREATE INDEX pieces_user ON pieces(user_id, status, name);

-- The way in: the doors, hallways, corners and stairs a piece has to pass
-- to get into a room. Steps: [{kind, name, width, height, turn_width, ...}].
CREATE TABLE routes (
  id         INTEGER PRIMARY KEY AUTOINCREMENT,
  user_id    INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  plan_id    INTEGER NOT NULL REFERENCES plans(id) ON DELETE CASCADE,
  name       TEXT NOT NULL DEFAULT 'The way in',
  steps      TEXT NOT NULL DEFAULT '[]',
  sort       INTEGER NOT NULL DEFAULT 0,
  created_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ','now')),
  updated_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ','now'))
);
CREATE INDEX routes_user ON routes(user_id, plan_id, sort);

-- A plan's checklist: measure the niche, buy the rug, book the movers.
CREATE TABLE todos (
  id         INTEGER PRIMARY KEY AUTOINCREMENT,
  user_id    INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  plan_id    INTEGER NOT NULL REFERENCES plans(id) ON DELETE CASCADE,
  title      TEXT NOT NULL,
  done       INTEGER NOT NULL DEFAULT 0,
  due        TEXT NOT NULL DEFAULT '',
  task_id    TEXT NOT NULL DEFAULT '',               -- the task in Pocket Tasks
  task_url   TEXT NOT NULL DEFAULT '',
  sort       INTEGER NOT NULL DEFAULT 0,
  source     TEXT NOT NULL DEFAULT '',
  done_at    TEXT NOT NULL DEFAULT '',
  created_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ','now'))
);
CREATE INDEX todos_user ON todos(user_id, plan_id, done, sort);

-- Photos of a plan's rooms (local pictures or ones from Pocket Photos).
CREATE TABLE plan_photos (
  id          INTEGER PRIMARY KEY AUTOINCREMENT,
  user_id     INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  plan_id     INTEGER NOT NULL REFERENCES plans(id) ON DELETE CASCADE,
  photo_id    INTEGER REFERENCES photos(id) ON DELETE CASCADE,
  room        TEXT NOT NULL DEFAULT '',              -- the room's name on the plan
  caption     TEXT NOT NULL DEFAULT '',
  pocket_id   TEXT NOT NULL DEFAULT '',              -- a photo in Pocket Photos
  pocket_url  TEXT NOT NULL DEFAULT '',
  taken_on    TEXT NOT NULL DEFAULT '',
  created_at  TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ','now'))
);
CREATE INDEX plan_photos_user ON plan_photos(user_id, plan_id);
