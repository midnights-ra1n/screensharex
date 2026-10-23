import type { ComponentChildren } from 'preact'
import { useEffect, useRef, useState } from 'preact/hooks'
import { api } from './api'
import gridIcon from '@material-symbols/svg-400/rounded/grid_view.svg?raw'
import chartIcon from '@material-symbols/svg-400/rounded/bar_chart.svg?raw'
import plugIcon from '@material-symbols/svg-400/rounded/devices.svg?raw'
import shieldIcon from '@material-symbols/svg-400/rounded/admin_panel_settings.svg?raw'
import logoutIcon from '@material-symbols/svg-400/rounded/logout.svg?raw'
import uploadIcon from '@material-symbols/svg-400/rounded/upload.svg?raw'
import copyIcon from '@material-symbols/svg-400/rounded/content_copy.svg?raw'
import trashIcon from '@material-symbols/svg-400/rounded/delete.svg?raw'
import externalIcon from '@material-symbols/svg-400/rounded/open_in_new.svg?raw'
import downloadIcon from '@material-symbols/svg-400/rounded/download.svg?raw'
import closeIcon from '@material-symbols/svg-400/rounded/close.svg?raw'
import searchIcon from '@material-symbols/svg-400/rounded/search.svg?raw'
import playIcon from '@material-symbols/svg-400/rounded/play_arrow.svg?raw'
import fileIcon from '@material-symbols/svg-400/rounded/draft.svg?raw'
import leafIcon from '@material-symbols/svg-400/rounded/eco.svg?raw'

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
  } catch { // no clipboard access (plain http on a LAN address, permissions…)
    dialog({ title: 'Copiez le lien', input: { value: text, readonly: true }, ok: 'Fermer', cancel: false })
  }
}

type DialogOpts = {
  title: string
  message?: ComponentChildren
  input?: { label?: string; value?: string; type?: string; placeholder?: string; min?: number; minLength?: number; maxLength?: number; required?: boolean; readonly?: boolean }
  ok?: string
  cancel?: string | false
  danger?: boolean
}
let openDialog: (d: DialogOpts & { resolve: (v: string | null) => void }) => void = () => {}

// In-app replacement for prompt()/confirm(): resolves to the input value ('' without input), or null when cancelled.
export const dialog = (o: DialogOpts) => new Promise<string | null>((resolve) => openDialog({ ...o, resolve }))
export const confirmDialog = async (o: DialogOpts) => (await dialog(o)) !== null

export function Dialogs() {
  const [d, setD] = useState<DialogOpts & { resolve: (v: string | null) => void }>()
  const ref = useRef<HTMLDialogElement>(null)
  useEffect(() => { openDialog = setD }, [])
  useEffect(() => {
    const el = ref.current
    if (!d || !el) return
    el.showModal()
    el.querySelector('input')?.select()
  }, [d])
  if (!d) return null
  const close = (v: string | null) => { ref.current?.close(); d.resolve(v); setD(undefined) }
  const { input } = d

  return (
    <dialog ref={ref} class="dialog" aria-labelledby="dialog-title"
      onCancel={(e) => { e.preventDefault(); close(null) }}
      onClick={(e) => e.target === ref.current && close(null)}>
      <form onSubmit={(e) => { e.preventDefault(); close(input ? String(new FormData(e.currentTarget).get('v') ?? '') : '') }}>
        <h2 id="dialog-title">{d.title}</h2>
        {d.message && <p class="muted">{d.message}</p>}
        {input && (
          <label>{input.label}
            <input name="v" type={input.type ?? 'text'} value={input.value} placeholder={input.placeholder} min={input.min}
              minLength={input.minLength} maxLength={input.maxLength} required={input.required} readOnly={input.readonly} autofocus />
          </label>
        )}
        <div class="dialog-actions">
          {d.cancel !== false && <button type="button" class="ghost" onClick={() => close(null)}>{d.cancel ?? 'Annuler'}</button>}
          <button class={d.danger ? 'danger-solid' : 'primary'} autofocus={!input}>{d.ok ?? 'OK'}</button>
        </div>
      </form>
    </dialog>
  )
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

// Material Symbols (rounded), bundled from the npm package: only the icons imported here ship.
const icons: Record<string, string> = {
  github: '<svg viewBox="0 0 24 24"><path d="M12 .5C5.73.5.5 5.73.5 12a11.5 11.5 0 0 0 7.86 10.92c.58.1.79-.25.79-.56v-2.17c-3.2.7-3.87-1.37-3.87-1.37-.52-1.33-1.28-1.69-1.28-1.69-1.04-.71.08-.7.08-.7 1.16.08 1.77 1.19 1.77 1.19 1.03 1.76 2.7 1.25 3.36.96.1-.75.4-1.25.73-1.54-2.56-.29-5.24-1.28-5.24-5.68 0-1.26.45-2.28 1.19-3.09-.12-.29-.52-1.46.11-3.05 0 0 .97-.31 3.17 1.18a11 11 0 0 1 5.77 0c2.2-1.49 3.17-1.18 3.17-1.18.63 1.59.23 2.76.11 3.05.74.81 1.19 1.83 1.19 3.09 0 4.41-2.69 5.39-5.25 5.67.41.36.78 1.06.78 2.14v3.17c0 .31.21.67.8.56A11.5 11.5 0 0 0 23.5 12C23.5 5.73 18.27.5 12 .5z"/></svg>', // brand mark, not in Material Symbols
  grid: gridIcon, chart: chartIcon, plug: plugIcon, shield: shieldIcon, logout: logoutIcon, upload: uploadIcon, copy: copyIcon, trash: trashIcon, external: externalIcon, download: downloadIcon, close: closeIcon, search: searchIcon, play: playIcon, file: fileIcon, leaf: leafIcon,
}

export const Icon = ({ name }: { name: string }) => (
  <span class="icon" aria-hidden="true" dangerouslySetInnerHTML={{ __html: icons[name] }} />
)

type ServerInfo = { version: string; repo: string }
let serverInfo: Promise<ServerInfo> | undefined

// Running version and GitHub repo, fetched once per page load.
export function useServerInfo() {
  const [info, setInfo] = useState<ServerInfo>()
  useEffect(() => { (serverInfo ??= api<ServerInfo>('/public')).then(setInfo, () => {}) }, [])
  return info
}

export const repoURL = (i: ServerInfo) => `https://github.com/${i.repo}`

export function Footer() {
  const info = useServerInfo()
  if (!info) return null
  return (
    <footer class="page-foot muted small">
      <a href={repoURL(info)} target="_blank" rel="noopener"><Icon name="github" /> {info.repo}</a>
      <span>ScreenShareX {info.version}</span>
    </footer>
  )
}

export const Meter = ({ used, total }: { used: number; total: number }) => {
  const pct = total > 0 ? Math.min(100, (used / total) * 100) : 0
  return (
    <div class="meter" role="meter" aria-valuenow={Math.round(pct)} aria-valuemin={0} aria-valuemax={100}>
      <div style={{ width: `${pct}%` }} class={pct > 90 ? 'danger' : ''} />
    </div>
  )
}
