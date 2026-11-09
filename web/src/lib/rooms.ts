// Names and icons of the Pocket apps Rooms reaches, and app states in words.
import type { AppSlug, AppStateName } from './types'

export const APP_INFO: Record<AppSlug, { name: string; icon: string }> = {
  home: { name: 'Pocket Home', icon: 'home' },
  inventory: { name: 'Pocket Inventory', icon: 'package' },
  wishlist: { name: 'Pocket Wishlist', icon: 'gift' },
  tasks: { name: 'Pocket Tasks', icon: 'check-square' },
  notes: { name: 'Pocket Notes', icon: 'note' },
  files: { name: 'Pocket Files', icon: 'file' },
  photos: { name: 'Pocket Photos', icon: 'image' },
  rentals: { name: 'Pocket Rentals', icon: 'building' },
  money: { name: 'Pocket Money', icon: 'wallet' },
  chat: { name: 'Pocket Chat', icon: 'message' },
}

export const STATE_WORDS: Record<AppStateName, string> = {
  on: 'connected', partly: 'partly connected', off: 'switched off', link_off: 'switched off in Pocket Core', missing: 'not on this box',
  old: 'needs an update', no_core: 'no Pocket Core', not_set_up: 'not set up', error: 'not reachable',
}
