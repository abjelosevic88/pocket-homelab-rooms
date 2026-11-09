// The furniture catalogue: what each kind of piece is called, its usual
// size, the room it needs around it, and how it looks.

export interface Clear { front?: number; sides?: number; back?: number; oneSide?: boolean; all?: number; allow?: string[] }
export interface Kind { type: string; name: string; cat: string; w: number; d: number; h: number; clear?: Clear; flat?: boolean; color: string; words?: string }

export const CATEGORIES: { id: string; name: string; icon: string }[] = [
  { id: 'living', name: 'Living', icon: 'home' },
  { id: 'sleeping', name: 'Sleeping', icon: 'bed' },
  { id: 'dining', name: 'Dining', icon: 'utensils' },
  { id: 'kitchen', name: 'Kitchen', icon: 'coffee' },
  { id: 'bath', name: 'Bathroom', icon: 'droplet' },
  { id: 'work', name: 'Work', icon: 'briefcase' },
  { id: 'storage', name: 'Storage', icon: 'archive' },
  { id: 'kids', name: 'Kids', icon: 'smile' },
  { id: 'outdoor', name: 'Outdoor', icon: 'tree' },
  { id: 'other', name: 'Other', icon: 'package' },
]

const wood = '#b98a5e', fabric = '#8c97a8', white = '#e9e7e2', steel = '#b8bec6', green = '#6f9a5b', soft = '#c9b79c'

export const KINDS: Kind[] = [
  { type: 'sofa', name: 'Sofa', cat: 'living', w: 210, d: 90, h: 85, color: fabric, clear: { front: 45, allow: ['coffee_table', 'rug'] }, words: 'couch settee three-seater' },
  { type: 'sofa_l', name: 'Corner sofa', cat: 'living', w: 270, d: 180, h: 82, color: fabric, clear: { front: 45, allow: ['coffee_table', 'rug'] }, words: 'l-shaped sectional' },
  { type: 'armchair', name: 'Armchair', cat: 'living', w: 80, d: 85, h: 95, color: fabric, clear: { front: 40, allow: ['coffee_table', 'rug'] } },
  { type: 'coffee_table', name: 'Coffee table', cat: 'living', w: 110, d: 60, h: 42, color: wood },
  { type: 'tv_unit', name: 'TV bench', cat: 'living', w: 160, d: 40, h: 50, color: wood, words: 'television media' },
  { type: 'tv', name: 'TV on the wall', cat: 'living', w: 125, d: 8, h: 72, color: '#2b2f36', words: 'television screen' },
  { type: 'bookcase', name: 'Bookcase', cat: 'storage', w: 80, d: 30, h: 200, color: wood, clear: { front: 50 }, words: 'shelves bookshelf' },
  { type: 'shelf', name: 'Low shelf', cat: 'storage', w: 120, d: 35, h: 80, color: wood, words: 'sideboard cabinet shoe' },
  { type: 'rug', name: 'Rug', cat: 'living', w: 200, d: 140, h: 1, color: soft, flat: true, words: 'carpet' },
  { type: 'lamp', name: 'Floor lamp', cat: 'living', w: 35, d: 35, h: 160, color: '#d7b45a' },
  { type: 'plant', name: 'Plant', cat: 'living', w: 45, d: 45, h: 100, color: green, words: 'tree pot' },
  { type: 'piano', name: 'Upright piano', cat: 'living', w: 150, d: 60, h: 125, color: '#3a2f2a', clear: { front: 70 } },
  { type: 'fireplace', name: 'Fireplace / stove', cat: 'living', w: 90, d: 50, h: 100, color: '#5d5550', clear: { front: 60 } },
  { type: 'bed_double', name: 'Double bed', cat: 'sleeping', w: 160, d: 205, h: 50, color: white, clear: { sides: 60, front: 50, allow: ['nightstand', 'rug'] }, words: 'queen king 140 180' },
  { type: 'bed_single', name: 'Single bed', cat: 'sleeping', w: 90, d: 200, h: 50, color: white, clear: { sides: 60, oneSide: true, allow: ['nightstand', 'rug'] }, words: 'bunk' },
  { type: 'crib', name: 'Cot', cat: 'kids', w: 65, d: 125, h: 90, color: white, clear: { front: 50 }, words: 'crib baby' },
  { type: 'nightstand', name: 'Night table', cat: 'sleeping', w: 45, d: 40, h: 55, color: wood, words: 'bedside' },
  { type: 'wardrobe', name: 'Wardrobe', cat: 'storage', w: 150, d: 60, h: 210, color: white, clear: { front: 70 }, words: 'closet' },
  { type: 'dresser', name: 'Chest of drawers', cat: 'storage', w: 80, d: 45, h: 85, color: wood, clear: { front: 60 }, words: 'dresser drawers' },
  { type: 'dining_table', name: 'Dining table', cat: 'dining', w: 160, d: 90, h: 75, color: wood, clear: { all: 75, allow: ['chair', 'bench', 'rug'] }, words: 'table' },
  { type: 'table_round', name: 'Round table', cat: 'dining', w: 110, d: 110, h: 75, color: wood, clear: { all: 75, allow: ['chair', 'bench', 'rug'] } },
  { type: 'chair', name: 'Chair', cat: 'dining', w: 45, d: 52, h: 82, color: wood },
  { type: 'bench', name: 'Bench', cat: 'dining', w: 140, d: 40, h: 45, color: wood },
  { type: 'counter', name: 'Kitchen unit', cat: 'kitchen', w: 60, d: 60, h: 90, color: white, clear: { front: 100, allow: ['rug'] }, words: 'cupboard cabinet worktop' },
  { type: 'sink_unit', name: 'Kitchen sink', cat: 'kitchen', w: 80, d: 60, h: 90, color: white, clear: { front: 100, allow: ['rug'] } },
  { type: 'stove', name: 'Cooker', cat: 'kitchen', w: 60, d: 60, h: 90, color: steel, clear: { front: 100, allow: ['rug'] }, words: 'oven hob range' },
  { type: 'fridge', name: 'Fridge', cat: 'kitchen', w: 60, d: 65, h: 185, color: steel, clear: { front: 100, allow: ['rug'] }, words: 'refrigerator freezer' },
  { type: 'dishwasher', name: 'Dishwasher', cat: 'kitchen', w: 60, d: 60, h: 85, color: steel, clear: { front: 100, allow: ['rug'] } },
  { type: 'island', name: 'Kitchen island', cat: 'kitchen', w: 180, d: 90, h: 90, color: white, clear: { all: 90, allow: ['chair', 'rug'] } },
  { type: 'washer', name: 'Washing machine', cat: 'bath', w: 60, d: 60, h: 85, color: white, clear: { front: 90 }, words: 'dryer laundry' },
  { type: 'toilet', name: 'Toilet', cat: 'bath', w: 40, d: 65, h: 80, color: white, clear: { front: 60 }, words: 'wc' },
  { type: 'bathtub', name: 'Bath', cat: 'bath', w: 170, d: 75, h: 60, color: white, clear: { front: 60 }, words: 'tub' },
  { type: 'shower', name: 'Shower', cat: 'bath', w: 90, d: 90, h: 200, color: '#cfe1ea', clear: { front: 70 } },
  { type: 'basin', name: 'Basin', cat: 'bath', w: 60, d: 45, h: 85, color: white, clear: { front: 70 }, words: 'sink washbasin vanity' },
  { type: 'desk', name: 'Desk', cat: 'work', w: 140, d: 70, h: 75, color: wood, clear: { front: 75, allow: ['office_chair', 'chair', 'rug'] } },
  { type: 'office_chair', name: 'Desk chair', cat: 'work', w: 55, d: 55, h: 100, color: '#4a4f58' },
  { type: 'radiator', name: 'Radiator', cat: 'other', w: 100, d: 10, h: 60, color: white },
  { type: 'stairs', name: 'Stairs', cat: 'other', w: 220, d: 100, h: 0, color: '#bfb3a3', flat: true, words: 'staircase steps' },
  { type: 'car', name: 'Car', cat: 'outdoor', w: 180, d: 450, h: 150, color: '#7a8796', clear: { sides: 70 } },
  { type: 'bike', name: 'Bicycle', cat: 'outdoor', w: 60, d: 180, h: 100, color: '#4a4f58' },
  { type: 'garden_table', name: 'Garden table', cat: 'outdoor', w: 120, d: 80, h: 72, color: '#9aa39a', clear: { all: 60, allow: ['chair', 'bench'] } },
  { type: 'box', name: 'Box (anything)', cat: 'other', w: 60, d: 60, h: 60, color: '#b9b1a5' },
]

const BY = new Map(KINDS.map(k => [k.type, k]))
export function kindOf(type: string): Kind { return BY.get(type) || BY.get('box')! }

export const FLOORS: { id: string; name: string; light: string; dark: string }[] = [
  { id: 'wood', name: 'Wood', light: '#efe3d2', dark: '#3a3128' },
  { id: 'tile', name: 'Tiles', light: '#e6ebef', dark: '#2c3237' },
  { id: 'stone', name: 'Stone', light: '#e7e3dc', dark: '#34312d' },
  { id: 'carpet', name: 'Carpet', light: '#ebe6ee', dark: '#332f37' },
  { id: 'vinyl', name: 'Vinyl', light: '#ece9e0', dark: '#33312b' },
  { id: 'concrete', name: 'Concrete', light: '#e3e3e1', dark: '#2f2f2e' },
  { id: 'deck', name: 'Decking', light: '#e6dccb', dark: '#363026' },
  { id: 'grass', name: 'Grass', light: '#dfe9d3', dark: '#283224' },
  { id: '', name: 'Not set', light: '#f2f1ee', dark: '#2a2b2d' },
]
export function floorOf(id?: string) { return FLOORS.find(f => f.id === (id || '')) || FLOORS[FLOORS.length - 1] }

export const ROOM_KINDS: { id: string; name: string; floor: string }[] = [
  { id: 'living', name: 'Living room', floor: 'wood' }, { id: 'bedroom', name: 'Bedroom', floor: 'wood' }, { id: 'kitchen', name: 'Kitchen', floor: 'tile' },
  { id: 'dining', name: 'Dining room', floor: 'wood' }, { id: 'bath', name: 'Bathroom', floor: 'tile' }, { id: 'toilet', name: 'Toilet', floor: 'tile' },
  { id: 'hall', name: 'Hall', floor: 'tile' }, { id: 'office', name: 'Office', floor: 'wood' }, { id: 'kids', name: "Kids' room", floor: 'wood' },
  { id: 'storage', name: 'Storage', floor: 'concrete' }, { id: 'laundry', name: 'Laundry', floor: 'tile' }, { id: 'balcony', name: 'Balcony / terrace', floor: 'deck' },
  { id: 'garage', name: 'Garage', floor: 'concrete' }, { id: 'garden', name: 'Garden', floor: 'grass' }, { id: 'stairs', name: 'Stairwell', floor: 'wood' },
  { id: 'other', name: 'Other', floor: 'wood' },
]
export const roomKindName = (k?: string) => ROOM_KINDS.find(r => r.id === k)?.name || 'Room'

export const SWATCHES = ['', '#8c97a8', '#6f8f5a', '#b98a5e', '#e9e7e2', '#3a3f47', '#c9b79c', '#b5584a', '#d7b45a', '#5b7fa8', '#9a7bb0', '#d98f6b']

export const OPENING_KINDS: { id: string; name: string }[] = [
  { id: 'door', name: 'Door' }, { id: 'double', name: 'Double door' }, { id: 'sliding', name: 'Sliding door' }, { id: 'pocket', name: 'Pocket door' },
  { id: 'opening', name: 'Opening' }, { id: 'window', name: 'Window' }, { id: 'garage', name: 'Garage door' },
]
