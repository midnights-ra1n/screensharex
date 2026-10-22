import { render } from 'preact'
import { useEffect, useState } from 'preact/hooks'
import { api, type Me } from './api'
import { Auth } from './pages/Auth'
import { Gallery } from './pages/Gallery'
import { Stats } from './pages/Stats'
import { Integrations } from './pages/Integrations'
import { Settings } from './pages/Settings'
import { Admin } from './pages/Admin'
import { Icon, Toasts } from './ui'
import './style.css'

const routes = [
  { path: 'gallery', label: 'Médias', icon: 'grid' },
  { path: 'stats', label: 'Stats', icon: 'chart' },
  { path: 'clients', label: 'Clients', icon: 'plug' },
  { path: 'settings', label: 'Profil', icon: 'user' },
  { path: 'admin', label: 'Admin', icon: 'shield', admin: true },
] as const

const currentRoute = () => location.hash.replace(/^#\/?/, '') || 'gallery'

function App() {
  const [me, setMe] = useState<Me | null | undefined>(undefined)
  const [route, setRoute] = useState(currentRoute())

  const refresh = () => api<Me>('/me').then(setMe, () => setMe(null))
  useEffect(() => {
    refresh()
    const onHash = () => setRoute(currentRoute())
    addEventListener('hashchange', onHash)
    return () => removeEventListener('hashchange', onHash)
  }, [])

  if (me === undefined) return <div class="center"><div class="spinner" /></div>
  if (me === null) return <Auth onDone={refresh} />

  const logout = async () => { await api('/auth/logout', { method: 'POST' }); setMe(null) }
  const nav = routes.filter((r) => !('admin' in r) || me.user.is_admin)
  const page = {
    gallery: <Gallery me={me} refresh={refresh} />,
    stats: <Stats me={me} />,
    clients: <Integrations me={me} refresh={refresh} />,
    settings: <Settings me={me} refresh={refresh} />,
    admin: me.user.is_admin ? <Admin /> : null,
  }[route] ?? <Gallery me={me} refresh={refresh} />

  return (
    <div class="shell">
      <aside class="sidebar">
        <a class="brand" href="#/gallery"><img src="/favicon.svg" alt="" /> ScreenShareX</a>
        <nav>
          {nav.map((r) => (
            <a href={`#/${r.path}`} class={route === r.path ? 'active' : ''} aria-current={route === r.path ? 'page' : undefined}>
              <Icon name={r.icon} /> <span>{r.label}</span>
            </a>
          ))}
        </nav>
        <button class="ghost logout" onClick={logout}><Icon name="logout" /> <span>Déconnexion</span></button>
      </aside>
      <main>{page}</main>
      <Toasts />
    </div>
  )
}

render(<App />, document.getElementById('app')!)
