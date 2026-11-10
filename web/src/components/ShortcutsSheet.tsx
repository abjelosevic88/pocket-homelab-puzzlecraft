import { Modal } from './ui'

const GROUPS: { title: string; keys: [string[], string][] }[] = [
  { title: 'Go to', keys: [[['t'], 'Today'], [['p'], 'Puzzles'], [['e'], 'Escape rooms & hunts'], [['b'], 'Puzzle books'], [['w'], 'Word lists'], [['s'], 'Your times'], [[','], 'Settings']] },
  { title: 'Make', keys: [[['n'], 'New puzzle'], [['⇧', 'N'], 'New escape room or hunt']] },
  { title: 'In a crossword', keys: [[['←', '→', '↑', '↓'], 'Move'], [['space'], 'Across or down'], [['tab'], 'Next word (⇧ back)'], [['⌫'], 'Rub out'], [['.'], 'Black square (making one)']] },
  { title: 'In a sudoku', keys: [[['1', '…', '9'], 'Put a number'], [['⌫'], 'Rub out'], [['←', '→', '↑', '↓'], 'Move']] },
  { title: 'Find', keys: [[['/'], 'Search & commands'], [['⌘', 'K'], 'Search & commands']] },
  { title: 'Window', keys: [[['['], 'Show or hide the sidebar'], [['esc'], 'Close']] },
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
