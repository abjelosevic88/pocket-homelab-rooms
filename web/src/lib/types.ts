export interface User { id: number; email: string; name: string; role: 'admin' | 'member'; created_at: string }
export interface APIToken { id: number; user_id: number; name: string; prefix: string; last_used_at: string | null; created_at: string }

export type ConnMode = 'core' | 'direct' | 'off'
export type ConnVia = '' | 'core' | 'direct'
export type AppSlug = 'home' | 'inventory' | 'wishlist' | 'tasks' | 'notes' | 'files' | 'photos' | 'rentals' | 'money' | 'chat'

type ConnFields = { [K in AppSlug as `${K}_mode`]: ConnMode } & { [K in AppSlug as `${K}_via`]: ConnVia } & { [K in AppSlug as `${K}_detail` | `${K}_url` | `${K}_token` | `${K}_public_url` | `${K}_open_url`]: string } & { [K in AppSlug as `${K}_configured`]: boolean }

export type Settings = ConnFields & {
  core_configured: boolean
  core_registered?: boolean
  title: string; date_format: string; time_format: string; week_start: number
  units: Units; snap: number; wall_cm: number; ceiling_cm: number; door_cm: number; currency: string; hands_cm: number; show_dims: boolean; clearance: boolean
  notes_folder: string; files_folder: string; wishlist_id: number; money_account: string; money_category: string; chat_to: string; ai_help: boolean; ai_photos: boolean
  notify_todos: boolean; notify_hour: number
  notify_url: string; notify_format: string; notify_configured: boolean
  ai_provider: 'auto' | 'pocket' | 'openai'; ai_pocket_model: string; openai_api_key: string; openai_model: string; openai_base_url: string
  setup_complete: boolean; base_url: string; demo: boolean
}

export type AppStateName = 'on' | 'partly' | 'off' | 'link_off' | 'missing' | 'old' | 'no_core' | 'not_set_up' | 'error'
export interface AppState { slug: AppSlug; name: string; reason: string; mode: ConnMode; via: ConnVia; state: AppStateName; detail: string; fix: string; on: string[]; open_url: string }

// ---- plans ----

export type Units = 'metric' | 'imperial'
export interface Pt { x: number; y: number }

export interface DRoom { id: string; name: string; kind?: string; points: Pt[]; floor?: string; color?: string; wall?: number; open?: number[]; height?: number }
export interface DWall { id: string; a: Pt; b: Pt; thick?: number }
export type OpeningKind = 'door' | 'double' | 'sliding' | 'pocket' | 'window' | 'opening' | 'garage'
export interface DOpening { id: string; kind: OpeningKind; room?: string; edge: number; wall?: string; at: number; width: number; height?: number; sill?: number; hinge?: 'start' | 'end'; out?: boolean; name?: string }
export interface DItem {
  id: string; name: string; type: string; cat?: string; x: number; y: number; w: number; d: number; h?: number; rot?: number; elev?: number
  color?: string; piece_id?: number; price?: number; buy?: boolean; locked?: boolean; note?: string; from?: string
}
export interface DDim { id: string; a: Pt; b: Pt; off?: number }
export interface DLabel { id: string; at: Pt; text: string; size?: number }
export interface Drawing { rooms: DRoom[]; walls: DWall[]; openings: DOpening[]; items: DItem[]; dims: DDim[]; labels: DLabel[] }

export interface Layout {
  id: number; plan_id: number; floor_id: number; name: string; notes: string; data?: Drawing
  area_cm2: number; rooms_n: number; items_n: number; budget: number; sort: number; revision: number; created_at: string; updated_at: string; current: boolean
}
export interface Underlay { x?: number; y?: number; cm_per_px?: number; opacity?: number; rot?: number; on?: boolean; w_px?: number; h_px?: number }
export interface Floor {
  id: number; plan_id: number; name: string; level: number; ceiling_cm: number; sort: number; current_layout_id: number | null
  underlay_photo_id: number | null; underlay: Underlay; layouts?: Layout[]
}
export type PlanKind = 'apartment' | 'house' | 'office' | 'studio' | 'garden' | 'other'
export type PlanStatus = 'living' | 'moving' | 'idea' | 'done'
export interface Plan {
  id: number; name: string; kind: PlanKind; address: string; currency: string; notes: string; status: PlanStatus
  home_id: number | null; home_name: string; rental_id: number | null; rental_name: string
  share_token: string; share_on: boolean; pinned: boolean; archived: boolean; source: string; created_at: string; updated_at: string
  floors?: Floor[]; area_cm2: number; rooms_n: number; items_n: number; budget: number; open_todos: number; preview?: Layout
}
export interface PlanDoc { plan: Plan; routes: Route[]; share_url?: string; home_url?: string; rental_url?: string }

export type PieceStatus = 'own' | 'want' | 'maybe' | 'gone'
export interface Piece {
  id: number; name: string; type: string; category: string; width: number; depth: number; height: number; color: string; status: PieceStatus
  price: number | null; currency: string; url: string; shop: string; notes: string; photo_id: number | null
  inventory_id: number | null; inventory_name: string; wish_id: number | null; wish_name: string; money_tx_id: string; source: string
  created_at: string; updated_at: string; photo_url: string; placed_in: string[]
}

export type StepKind = 'door' | 'hall' | 'corner' | 'stairs' | 'lift'
export interface RouteStep { kind: StepKind; name: string; width: number; height?: number; turn?: number; depth?: number }
export interface Route { id: number; plan_id: number; name: string; steps: RouteStep[]; sort: number }
export interface FitStep { step: RouteStep; ok: boolean; how: string; spare: number; carry?: [number, number, number]; max_len?: number }
export interface FitResult { ok: boolean; steps: FitStep[]; summary: string }

export interface Todo { id: number; plan_id: number; title: string; done: boolean; due: string; task_id: string; task_url: string; sort: number; source: string; done_at: string; plan_name?: string }
export interface PlanPhoto { id: number; plan_id: number; photo_id: number | null; room: string; caption: string; pocket_id: string; taken_on: string; url: string; thumb_url: string }

export interface RoomSum { id: string; name: string; kind: string; floor: string; area_cm2: number; perimeter_cm: number; wall_cm2: number; skirting_cm: number; window_cm2: number; daylight_pct: number; size: string; items: string[]; doors: number; windows: number }
export interface FinishSum { floor: string; area_cm2: number; order_cm2: number; rooms: number; room_list: string }
export interface ItemSum { id: string; name: string; type: string; room: string; size: string; price: number; buy: boolean; piece_id: number }
export interface LayoutSum {
  layout_id: number; layout: string; floor_id: number; floor_name: string; ceiling_cm: number; area_cm2: number; outside_cm2: number; wall_cm2: number; skirting_cm: number
  rooms: RoomSum[]; finishes: FinishSum[]; items: ItemSum[]; budget: number; owned: number; to_buy: number
}
