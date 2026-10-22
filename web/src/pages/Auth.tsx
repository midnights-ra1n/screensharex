import { useEffect, useState } from 'preact/hooks'
import { api } from '../api'

export function Auth({ onDone }: { onDone: () => void }) {
  const [info, setInfo] = useState<{ registration: boolean; setup: boolean; version: string }>()
  const [mode, setMode] = useState<'login' | 'register'>('login')
  const [error, setError] = useState('')
  const [busy, setBusy] = useState(false)

  useEffect(() => {
    api('/public').then((i) => { setInfo(i); if (i.setup) setMode('register') })
  }, [])

  const submit = async (e: Event) => {
    e.preventDefault()
    const f = new FormData(e.target as HTMLFormElement)
    setBusy(true)
    setError('')
    try {
      await api(`/auth/${mode}`, { body: { username: f.get('username'), password: f.get('password') } })
      onDone()
    } catch (err) {
      setError((err as Error).message)
    } finally {
      setBusy(false)
    }
  }

  return (
    <div class="center auth">
      <form class="card auth-card" onSubmit={submit}>
        <img src="/favicon.svg" alt="" width={56} height={56} />
        <h1>ScreenShareX</h1>
        <p class="muted">
          {info?.setup ? 'Bienvenue ! Créez le compte administrateur.' : 'Hébergement de captures, GIF et vidéos.'}
        </p>
        {info?.registration && !info.setup && (
          <div class="tabs" role="tablist">
            <button type="button" role="tab" aria-selected={mode === 'login'} onClick={() => setMode('login')}>Connexion</button>
            <button type="button" role="tab" aria-selected={mode === 'register'} onClick={() => setMode('register')}>Inscription</button>
          </div>
        )}
        <label>Nom d'utilisateur<input name="username" autocomplete="username" required minLength={3} maxLength={32} /></label>
        <label>Mot de passe
          <input name="password" type="password" autocomplete={mode === 'login' ? 'current-password' : 'new-password'} required minLength={8} maxLength={72} />
        </label>
        {error && <p class="error" role="alert">{error}</p>}
        <button class="primary" disabled={busy}>{mode === 'login' ? 'Se connecter' : 'Créer le compte'}</button>
        <p class="muted small">{info?.version}</p>
      </form>
    </div>
  )
}
