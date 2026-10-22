import { useEffect, useRef, useState } from 'preact/hooks'
import { api, date, size, type User } from '../api'
import { attempt, Icon, Meter, toast } from '../ui'

type Settings = { registration: number; default_quota: number; max_upload: number; max_upload_api: number; uploads_per_hour: number; uploads_per_hour_api: number; compress: number }
type Row = { user: User; count: number; used: number; quota: number }
type Update = { current: string; available: boolean; mode: 'deb' | 'docker' | 'manual'; latest: { tag_name: string; body: string; html_url: string; published_at: string } }

const MB = 1 << 20
// Byte values are edited in Mo; 0 stays 0 (unlimited).
const toMB = (b: number) => Math.round(b / MB)

export function Admin() {
  return (
    <div class="page">
      <header class="page-head"><h1>Administration</h1></header>
      <ServerSettings />
      <Users />
      <Backup />
      <Updates />
    </div>
  )
}

function ServerSettings() {
  const [s, setS] = useState<Settings>()
  useEffect(() => { api<Settings>('/admin/settings').then(setS) }, [])
  if (!s) return null

  const num = (k: keyof Settings, mult = 1) => ({
    value: s[k] / mult,
    onInput: (e: Event) => setS({ ...s, [k]: Math.max(0, Number((e.currentTarget as HTMLInputElement).value)) * mult }),
  })
  const save = (e: Event) => {
    e.preventDefault()
    attempt(async () => setS(await api('/admin/settings', { method: 'PUT', body: s })), 'Paramètres enregistrés')
  }

  return (
    <section class="card">
      <h2>Paramètres du serveur</h2>
      <form class="form grid-form" onSubmit={save}>
        <label class="switch"><input type="checkbox" checked={s.registration === 1} onChange={(e) => setS({ ...s, registration: e.currentTarget.checked ? 1 : 0 })} />
          <span>Inscriptions ouvertes au public</span></label>
        <label class="switch"><input type="checkbox" checked={s.compress === 1} onChange={(e) => setS({ ...s, compress: e.currentTarget.checked ? 1 : 0 })} />
          <span>Créer des versions légères (ffmpeg)</span></label>
        <label>Quota par défaut (Mo, 0 = illimité)<input type="number" min={0} {...num('default_quota', MB)} /></label>
        <label>Taille max par fichier depuis le site web (Mo)<input type="number" min={1} {...num('max_upload', MB)} /></label>
        <label>Taille max par fichier depuis les applications (Mo)<input type="number" min={1} {...num('max_upload_api', MB)} /></label>
        <label>Envois max par heure depuis le site web (0 = illimité)<input type="number" min={0} {...num('uploads_per_hour')} /></label>
        <label>Envois max par heure depuis les applications (ShareX, scripts…) (0 = illimité)<input type="number" min={0} {...num('uploads_per_hour_api')} /></label>
        <button class="primary">Enregistrer</button>
      </form>
    </section>
  )
}

function Users() {
  const [rows, setRows] = useState<Row[]>([])
  const load = () => api<Row[]>('/admin/users').then(setRows)
  useEffect(() => { load() }, [])

  const create = (e: Event) => {
    e.preventDefault()
    const form = e.currentTarget as HTMLFormElement
    const f = new FormData(form)
    attempt(async () => {
      await api('/admin/users', { body: { username: f.get('username'), password: f.get('password'), is_admin: f.get('admin') === 'on' } })
      form.reset()
      load()
    }, 'Utilisateur créé')
  }
  const patch = (u: User, body: object, ok: string) =>
    attempt(async () => { await api(`/admin/users/${u.id}`, { method: 'PATCH', body }); load() }, ok)
  const setQuota = (u: User) => {
    const v = prompt(`Quota de ${u.username} en Mo (vide = défaut du serveur, 0 = illimité)`, u.quota >= 0 ? String(toMB(u.quota)) : '')
    if (v === null) return
    patch(u, { quota: v.trim() === '' ? -1 : Math.max(0, Number(v)) * MB }, 'Quota modifié')
  }
  const resetPassword = (u: User) => {
    const v = prompt(`Nouveau mot de passe pour ${u.username} (8 caractères min.)`)
    if (v) patch(u, { password: v }, 'Mot de passe réinitialisé')
  }
  const remove = (u: User) => {
    if (confirm(`Supprimer ${u.username} et tous ses médias ?`))
      attempt(async () => { await api(`/admin/users/${u.id}`, { method: 'DELETE' }); load() }, 'Utilisateur supprimé')
  }

  return (
    <section class="card">
      <h2>Utilisateurs</h2>
      <ul class="users">
        {rows.map(({ user: u, count, used, quota }) => (
          <li>
            <div class="user-head">
              <strong>{u.username}</strong>
              {u.is_admin && <span class="pill">admin</span>}
              <span class="muted small">depuis le {date(u.created_at)}</span>
            </div>
            <span class="muted small">{count} fichiers · {size(used)} / {quota > 0 ? size(quota) : 'illimité'}{u.quota < 0 && ' (défaut)'}</span>
            {quota > 0 && <Meter used={used} total={quota} />}
            <div class="actions">
              <button onClick={() => setQuota(u)}>Quota</button>
              <button onClick={() => patch(u, { is_admin: !u.is_admin }, 'Rôle modifié')}>{u.is_admin ? 'Retirer admin' : 'Rendre admin'}</button>
              <button onClick={() => resetPassword(u)}>Mot de passe</button>
              <button class="danger" onClick={() => remove(u)}><Icon name="trash" /></button>
            </div>
          </li>
        ))}
      </ul>
      <form class="form inline-form" onSubmit={create}>
        <input name="username" placeholder="Nom d'utilisateur" required minLength={3} maxLength={32} aria-label="Nom d'utilisateur" />
        <input name="password" type="password" placeholder="Mot de passe" required minLength={8} maxLength={72} aria-label="Mot de passe" autocomplete="new-password" />
        <label class="switch"><input type="checkbox" name="admin" /><span>Admin</span></label>
        <button class="primary">Ajouter</button>
      </form>
    </section>
  )
}

// Polls until the server answers again (after an import or update restart), then reloads.
const waitForRestart = (from?: string) => {
  const tick = async () => {
    try {
      const p = await api('/public')
      if (!from || p.version !== from) return location.reload()
    } catch {}
    setTimeout(tick, 2000)
  }
  setTimeout(tick, 2000)
}

function Backup() {
  const input = useRef<HTMLInputElement>(null)
  const [busy, setBusy] = useState(false)

  const importFile = async (file: File) => {
    if (!confirm('Importer remplace TOUTE la base de données (utilisateurs, médias, réglages). Continuer ?')) return
    setBusy(true)
    try {
      const res = await fetch('/api/admin/import', { method: 'POST', body: file, headers: { 'Content-Type': 'application/gzip' } })
      const data = await res.json()
      if (!res.ok) throw new Error(data.error)
      toast(data.message)
      waitForRestart()
    } catch (e) {
      toast((e as Error).message, true)
      setBusy(false)
    }
  }

  return (
    <section class="card">
      <h2>Sauvegarde</h2>
      <p class="muted">L'export contient la base SQLite et tous les fichiers (.tar.gz). L'import remplace la base et ajoute les fichiers, puis redémarre le serveur.</p>
      <div class="actions">
        <a class="button primary" href="/api/admin/export" download><Icon name="download" /> Exporter</a>
        <button disabled={busy} onClick={() => input.current?.click()}><Icon name="upload" /> {busy ? 'Import en cours…' : 'Importer'}</button>
        <input ref={input} type="file" accept=".tar.gz,.tgz,application/gzip" hidden
          onChange={(e) => { const f = e.currentTarget.files?.[0]; f && importFile(f); e.currentTarget.value = '' }} />
      </div>
    </section>
  )
}

function Updates() {
  const [u, setU] = useState<Update>()
  const [busy, setBusy] = useState(false)
  const check = () => { setBusy(true); attempt(async () => setU(await api('/admin/update'))).finally(() => setBusy(false)) }
  const install = () => {
    if (!u || !confirm(`Installer ${u.latest.tag_name} et redémarrer le serveur ?`)) return
    setBusy(true)
    attempt(async () => { await api('/admin/update', { body: { tag: u.latest.tag_name } }); waitForRestart(u.current) }, 'Mise à jour lancée')
  }

  return (
    <section class="card">
      <h2>Mises à jour</h2>
      {!u ? (
        <button disabled={busy} onClick={check}>{busy ? 'Recherche…' : 'Rechercher des mises à jour'}</button>
      ) : !u.available ? (
        <p>Vous utilisez la dernière version ({u.current}).</p>
      ) : (
        <div class="update">
          <p><strong>{u.latest.tag_name}</strong> est disponible (actuelle : {u.current}) · <a href={u.latest.html_url} target="_blank" rel="noopener">notes de version</a></p>
          {u.latest.body && <pre class="notes">{u.latest.body}</pre>}
          {u.mode === 'deb' && <button class="primary" disabled={busy} onClick={install}>{busy ? 'Installation…' : 'Installer et redémarrer'}</button>}
          {u.mode === 'docker' && <pre>docker compose pull && docker compose up -d</pre>}
          {u.mode === 'manual' && <p class="muted">Téléchargez le binaire depuis la page de la release puis redémarrez le service.</p>}
        </div>
      )}
    </section>
  )
}
