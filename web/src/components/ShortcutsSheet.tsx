import { Modal } from './ui'

const GROUPS: { title: string; keys: [string[], string][] }[] = [
  { title: 'Go to', keys: [[['p'], 'Plans'], [['u'], 'Furniture'], [['i'], 'Will it fit?'], [[','], 'Settings']] },
  { title: 'Make', keys: [[['n'], 'New plan'], [['⇧', 'N'], 'New piece of furniture']] },
  { title: 'On a plan', keys: [[['v'], 'Select and move'], [['o'], 'Draw a room'], [['w'], 'A wall'], [['m'], 'Measure'], [['f'], 'Add furniture'], [['r'], 'Turn 90° (⇧ back)'], [['←', '→'], 'Nudge 1 cm (⇧ 10 cm)'], [['⌘', 'D'], 'Duplicate'], [['del'], 'Remove'], [['⌘', 'Z'], 'Undo (⇧ redo)'], [['0'], 'Fit the plan'], [['+', '−'], 'Zoom']] },
  { title: 'Find', keys: [[['/'], 'Search & commands'], [['⌘', 'K'], 'Search & commands']] },
  { title: 'Window', keys: [[['['], 'Show or hide the sidebar'], [['esc'], 'Close, stop drawing, choose nothing']] },
  { title: 'Help', keys: [[['?'], 'This sheet']] },
]

export function ShortcutsSheet({ onClose }: { onClose: () => void }) {
  return (
    <Modal title="Keyboard shortcuts" onClose={onClose} size="lg">
      <div className="shortcuts">
        {GROUPS.map(g => (
          <section key={g.title}>
            <h4 className="sc-h">{g.title}</h4>
            {g.keys.map(([keys, label]) => (
              <div key={label + keys.join('')} className="sc-row"><span>{label}</span><span className="sc-keys">{keys.map(k => <kbd key={k}>{k}</kbd>)}</span></div>
            ))}
          </section>
        ))}
      </div>
    </Modal>
  )
}
