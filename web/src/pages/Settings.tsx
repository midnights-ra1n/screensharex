import { useState } from 'preact/hooks'
import { api, size, type Me } from '../api'
import { attempt, getSaver, setSaver } from '../ui'

const fill = (t: string, me: Me) =>
  t.replaceAll('{filename}', 'capture.png').replaceAll('{size}', size(245760)).replaceAll('{user}', me.user.username)
    .replaceAll('{date}', new Date().toLocaleString('fr-FR', { dateStyle: 'short', timeStyle: 'short' })).replaceAll('{views}', '42')

export function Settings({ me, refresh }: { me: Me; refresh: () => void }) {
  const [e, setE] = useState({ ...me.user })
  const [saver, setSaverState] = useState(getSaver())
  const set = (k: keyof typeof e) => (ev: Event) => {
    const t = ev.currentTarget as HTMLInputElement
    setE({ ...e, [k]: t.type === 'checkbox' ? t.checked : t.value })
  }

  const saveEmbed = (ev: Event) => {
    ev.preventDefault()
    attempt(async () => { await api('/me', { method: 'PATCH', body: e }); refresh() }, 'Embed enregistré')
  }
  const savePassword = (ev: Event) => {
    ev.preventDefault()
    const form = ev.currentTarget as HTMLFormElement
    const f = new FormData(form)
    attempt(async () => {
      await api('/me', { method: 'PATCH', body: { password: f.get('password'), current_password: f.get('current') } })
      form.reset()
    }, 'Mot de passe modifié')
  }

  return (
    <div class="page">
      <header class="page-head"><h1>Profil</h1><p class="muted">Connecté en tant que <strong>{me.user.username}</strong></p></header>

      <section class="card">
        <h2>Embed Discord & messageries</h2>
        <p class="muted">Variables : <code>{'{filename}'}</code> <code>{'{size}'}</code> <code>{'{date}'}</code> <code>{'{user}'}</code> <code>{'{views}'}</code></p>
        <div class="two-col">
          <form class="form" onSubmit={saveEmbed}>
            <label>Nom du site<input value={e.embed_site} onInput={set('embed_site')} maxLength={100} /></label>
            <label>Titre<input value={e.embed_title} onInput={set('embed_title')} maxLength={200} /></label>
            <label>Description<textarea value={e.embed_description} onInput={set('embed_description')} maxLength={500} rows={2} /></label>
            <label class="inline">Couleur<input type="color" value={e.embed_color} onInput={set('embed_color')} /></label>
            <label class="switch">
              <input type="checkbox" checked={e.embed_raw} onChange={set('embed_raw')} />
              <span>Embed brut : envoyer directement le fichier aux bots (image seule, sans texte)</span>
            </label>
            <button class="primary">Enregistrer</button>
          </form>
          <div class="discord" aria-label="Aperçu de l'embed">
            {e.embed_raw ? (
              <div class="discord-raw"><div class="ph" /></div>
            ) : (
              <div class="discord-embed" style={{ borderColor: e.embed_color }}>
                <span class="d-site">{e.embed_site}</span>
                <span class="d-title">{fill(e.embed_title, me)}</span>
                <span class="d-desc">{fill(e.embed_description, me)}</span>
                <div class="ph" />
              </div>
            )}
            <p class="muted small">Les vidéos sont toujours envoyées brutes aux bots pour être lues directement dans Discord.</p>
          </div>
        </div>
      </section>

      <section class="card">
        <h2>Économie de données</h2>
        <label class="switch">
          <input type="checkbox" checked={saver} onChange={() => { setSaver(!saver); setSaverState(!saver) }} />
          <span>Afficher les versions compressées (WebP / MP4 légers) sur cet appareil</span>
        </label>
        <p class="muted small">Les pages publiques servent aussi la version légère aux navigateurs qui envoient l'en-tête Save-Data.</p>
      </section>

      <section class="card">
        <h2>Mot de passe</h2>
        <form class="form narrow" onSubmit={savePassword}>
          <label>Mot de passe actuel<input name="current" type="password" autocomplete="current-password" required /></label>
          <label>Nouveau mot de passe<input name="password" type="password" autocomplete="new-password" required minLength={8} maxLength={72} /></label>
          <button class="primary">Modifier</button>
        </form>
      </section>
    </div>
  )
}
