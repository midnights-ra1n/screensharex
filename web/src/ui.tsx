import { useEffect, useState } from 'preact/hooks'

type Toast = { id: number; text: string; error?: boolean }
let push: (t: Toast) => void = () => {}
let seq = 0

export const toast = (text: string, error = false) => push({ id: ++seq, text, error })

export function Toasts() {
  const [list, setList] = useState<Toast[]>([])
  useEffect(() => {
    push = (t) => {
      setList((l) => [...l, t])
      setTimeout(() => setList((l) => l.filter((x) => x.id !== t.id)), 3500)
    }
  }, [])
  return (
    <div class="toasts" role="status" aria-live="polite">
      {list.map((t) => <div class={'toast' + (t.error ? ' error' : '')}>{t.text}</div>)}
    </div>
  )
}

export async function copy(text: string) {
  try {
    await navigator.clipboard.writeText(text)
    toast('Lien copié')
  } catch {
    prompt('Copiez le lien :', text)
  }
}

// Runs an async action and reports its error as a toast.
export const attempt = async (fn: () => Promise<unknown>, ok?: string) => {
  try {
    await fn()
    if (ok) toast(ok)
  } catch (e) {
    toast((e as Error).message, true)
  }
}

const saverKey = 'ssx-data-saver'
export function getSaver(): boolean {
  try {
    const v = localStorage.getItem(saverKey)
    if (v !== null) return v === '1'
  } catch {}
  return (navigator as any).connection?.saveData === true
}
export function setSaver(on: boolean) {
  try { localStorage.setItem(saverKey, on ? '1' : '0') } catch {}
}

const paths: Record<string, string> = {
  grid: 'M4 4h7v7H4zM13 4h7v7h-7zM4 13h7v7H4zM13 13h7v7h-7z',
  chart: 'M4 20V10M10 20V4M16 20v-7M22 20H2',
  plug: 'M9 2v6M15 2v6M6 8h12v4a6 6 0 0 1-12 0zM12 18v4',
  user: 'M12 12a4 4 0 1 0 0-8 4 4 0 0 0 0 8zM4 21a8 8 0 0 1 16 0',
  shield: 'M12 2l8 4v6c0 5-3.5 8.5-8 10-4.5-1.5-8-5-8-10V6z',
  logout: 'M15 4h4v16h-4M10 16l4-4-4-4M14 12H3',
  upload: 'M12 16V4M7 9l5-5 5 5M4 20h16',
  copy: 'M9 9h11v11H9zM5 15H4V4h11v1',
  trash: 'M4 7h16M10 11v6M14 11v6M6 7l1 13h10l1-13M9 7V4h6v3',
  external: 'M14 4h6v6M20 4l-9 9M18 14v6H4V6h6',
  download: 'M12 4v12M7 11l5 5 5-5M4 20h16',
  close: 'M6 6l12 12M18 6L6 18',
  search: 'M11 18a7 7 0 1 0 0-14 7 7 0 0 0 0 14zM21 21l-5-5',
  play: 'M8 5v14l11-7z',
  file: 'M6 2h8l4 4v16H6zM14 2v4h4',
  leaf: 'M5 21c0-9 6-15 15-16-1 9-7 15-15 16zM5 21l8-8',
}

export const Icon = ({ name }: { name: string }) => (
  <svg class="icon" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true">
    <path d={paths[name]} />
  </svg>
)

export const Meter = ({ used, total }: { used: number; total: number }) => {
  const pct = total > 0 ? Math.min(100, (used / total) * 100) : 0
  return (
    <div class="meter" role="meter" aria-valuenow={Math.round(pct)} aria-valuemin={0} aria-valuemax={100}>
      <div style={{ width: `${pct}%` }} class={pct > 90 ? 'danger' : ''} />
    </div>
  )
}
