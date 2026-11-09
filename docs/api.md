# API

Everything the web app does goes through this JSON API at `/api/v1`. Sign in with the session cookie, or send a personal token (Settings → API & shortcuts) as `Authorization: Bearer prms_…`. Everything is per person: another account's ids answer 404.

Lengths are centimetres, areas square centimetres (`area_cm2`), money minor units (cents) of the plan's or piece's currency.

## Overview

| Call | What it is |
|---|---|
| `GET /home` | Your plans (with a preview of the first floor's current layout), open checklist items, wanted furniture |
| `GET /summary` | Plans with their area, rooms, pieces, what is to buy and to do; wanted furniture — for Homepage and Pocket AI |
| `GET /search?q=` | Plans, rooms (by name, on any layout) and furniture |
| `GET /day/YYYY-MM-DD` | Checklist items due or done on a day (Pocket Day) |
| `GET /templates` | The ready-made starting points with their drawings |

## Plans, floors, layouts

| Call | What it is |
|---|---|
| `GET /plans` (`?archived=1`, `?lean=1` without previews), `POST /plans` | Plans. `POST` takes `name kind address currency status notes home_id home_name rental_id rental_name` plus `template` (`empty room studio one_bed two_bed house`, `width`/`depth` for `room`) or a whole `drawing`; it makes the first floor and its layout "As it is" |
| `GET/PUT/DELETE /plans/{id}` | A plan with its floors, each floor's layouts (no drawings) and the ways in |
| `POST /plans/{id}/share` | `{"on": true, "new": false}` — the view-only link `/s/{token}` |
| `POST /plans/{id}/duplicate` | A copy with every floor, layout and way in |
| `GET /plans/{id}/summary` (`?layout=ID`) | Per floor: rooms with size, area, walls to paint, skirting, daylight; floor finishes to order (+10 %); the furniture and what is to buy |
| `POST /plans/{id}/floors` | `{"name", "level", "ceiling_cm", "copy_from": LAYOUT}` — a new floor (optionally with another layout's outline) |
| `PUT/DELETE /floors/{id}` | `name level ceiling_cm current_layout_id underlay_photo_id underlay` (the picture to trace over: `{x, y, cm_per_px, opacity, rot, on, w_px, h_px}`) |
| `POST /floors/{id}/layouts` | A new option: `{"name", "copy_from": LAYOUT, "shell": true}` (shell = only the rooms, walls, doors and windows) or `{"data"}` |
| `GET/PUT/DELETE /layouts/{id}` | A layout. `PUT {"data", "revision"}` saves the drawing; a stale `revision` answers 409 |

### The drawing

```json
{
  "rooms":    [{"id": "r1", "name": "Bedroom", "kind": "bedroom", "floor": "wood", "wall": 12, "open": [2], "height": 0,
                "points": [{"x": 0, "y": 0}, {"x": 360, "y": 0}, {"x": 360, "y": 300}, {"x": 0, "y": 300}]}],
  "walls":    [{"id": "w1", "a": {"x": 0, "y": 0}, "b": {"x": 200, "y": 0}, "thick": 10}],
  "openings": [{"id": "o1", "kind": "door", "room": "r1", "edge": 2, "at": 60, "width": 80, "height": 205, "hinge": "start", "out": false}],
  "items":    [{"id": "i1", "name": "Bed", "type": "bed_double", "x": 180, "y": 150, "w": 160, "d": 205, "h": 50, "rot": 0, "elev": 0,
                "piece_id": 3, "buy": true, "price": 49900}],
  "dims":     [{"id": "m1", "a": {"x": 0, "y": 320}, "b": {"x": 360, "y": 320}}],
  "labels":   [{"id": "t1", "at": {"x": 100, "y": -40}, "text": "Morning sun"}]
}
```

x runs to the right and y down, in centimetres. A room's points are its inside outline (what you measure); its walls are drawn outside it, `open` lists the edges without a wall. Edge `i` runs from point `i` to point `i+1`; a door or window sits on it, `at` cm from its start. An item's `x, y` is its centre, `w` side to side, `d` front to back (its front is the bottom edge before it is turned), `rot` degrees clockwise.

## Furniture and the way in

| Call | What it is |
|---|---|
| `GET /pieces` (`?status=own\|want\|maybe\|gone&q=`), `POST /pieces` | Measured furniture: `name type category width depth height color status price currency url shop notes photo_id inventory_id inventory_name wish_id wish_name` |
| `GET/PUT/DELETE /pieces/{id}` | A piece; `GET` also says how it does against every way in. Changing its size updates its copies on every plan |
| `POST /pieces/{id}/wish` | Put it on Pocket Wishlist |
| `POST /pieces/{id}/bought` | `{"price", "date", "book": true}` — it becomes yours, booked in Pocket Money, the wish ticked off |
| `GET/POST /plans/{id}/routes`, `PUT/DELETE /routes/{id}` | The ways in: `{"name", "steps": [{"kind": "door\|hall\|corner\|stairs\|lift", "name", "width", "height", "turn", "depth"}]}` |
| `POST /fit` | `{"piece_id"}` or `{"width", "depth", "height"}`, with `{"route_id"}` or `{"steps"}` and optionally `{"hands": 2}` → for each step whether it gets through, how to carry it (turned diagonally, tilted up round a corner) and the cm to spare |

## Checklist and photos

`GET /todos` (`?plan_id&open=1`) · `POST /plans/{id}/todos` `{"title", "due"}` · `PUT/DELETE /todos/{id}` · `GET/POST /plans/{id}/photos` `{"photo_id"}` or `{"pocket_id"}` (a Pocket Photos picture), `{"room", "caption"}` · `PUT/DELETE /plan-photos/{id}` · `POST /photos` (multipart `file`, `thumb`, `kind room|underlay|piece`) · `GET /photos/{id}` (`?thumb=1`).

## The view-only link (no account)

`GET /public/s/{token}` — the floors and every layout of a shared plan, with the room sizes. Never the notes, prices, checklist or photos.

## The other Pocket apps (per person, through Pocket Core or a direct token)

`GET /apps` (how each app is reached) · `GET /apps/homes`, `GET /apps/home-things?home_id=` (Pocket Home appliances and house plants with their room and a usual size) · `GET /apps/inventory?q=` · `GET /apps/wishes?q=` · `GET /apps/money` (accounts and categories) · `GET /apps/photos?q=`, `GET /apps/photos/{uuid}/image`, `POST /apps/photos/{uuid}/copy` · `GET /apps/rentals` · `GET /apps/chats`, `POST /apps/chat` `{"layout_id", "to", "text"}` · `POST /plans/{id}/note` (the summary to Pocket Notes) · `POST /plans/{id}/tasks` (open checklist items to Pocket Tasks) · `POST /layouts/{id}/file?type=svg|png` (raw body: the picture to Pocket Files).

## AI (Pocket AI or your own OpenAI key)

`POST /ai/read-plan` `{"photo_id", "hint"}` → a draft drawing from a picture of a floor plan · `POST /ai/sizes` `{"text"}` → a piece's size from a shop's description · `POST /ai/tips` `{"layout_id", "issues", "question"}` → `{"text"}`. Nothing is saved until you keep it.

## Other Pocket apps acting for you

With a `pcx_` token from Pocket Core another app reaches only: `rooms:read` — `GET summary, home, search, plans, pieces, todos, day/{date}`, `GET plans/{id}, pieces/{id}, layouts/{id}`, `GET plans/{id}/summary|routes`, `POST fit`; `rooms:write` — `POST pieces, plans, todos, plans/{id}/todos`, `PUT/DELETE` of the pieces and checklist items it added itself.

Events on the Core bus: `rooms.plan_created`, `rooms.layout_saved` (at most every few minutes per layout), `rooms.piece_added`, `rooms.piece_bought`, `rooms.todo_done`.

## Data

`GET /export.json` (`?photos=0` without pictures, `?plan=ID` one plan) · `GET /export.csv?what=furniture|rooms` · `POST /import` (a Pocket Rooms export — again and again, nothing is doubled — or a Sweet Home 3D `.sh3d` file / its `Home.xml`, which becomes a new plan) · `GET /backup` (admins: the whole database).
