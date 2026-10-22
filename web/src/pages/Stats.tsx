import { useEffect, useState } from 'preact/hooks'
import { api, size, type Me } from '../api'
import { Meter, toast } from '../ui'

type StatsData = {
  count: number
  size: number
  views: number
  types: Record<string, [number, number]>
  days: { day: string; count: number; size: number }[]
  server?: {
    users: number; stored: number; dedup_saved: number; disk_free: number; disk_total: number
    uptime: number; version: string; go: string; memory: number; goroutines: number; cache: string; ffmpeg: boolean
  }
}

const typeLabels: Record<string, string> = { image: 'Images', gif: 'GIF', video: 'Vidéos', audio: 'Audio', other: 'Autres' }

const duration = (s: number) => {
  const d = Math.floor(s / 86400), h = Math.floor((s % 86400) / 3600), m = Math.floor((s % 3600) / 60)
  return d ? `${d} j ${h} h` : h ? `${h} h ${m} min` : `${m} min`
}

const dayLabel = (d: string) =>
  new Date(d + 'T12:00:00').toLocaleDateString('fr-FR', { weekday: 'short', day: 'numeric', month: 'short' })

export function Stats({ me }: { me: Me }) {
  const [scope, setScope] = useState<'me' | 'server'>(me.user.is_admin ? 'server' : 'me')
  const [s, setS] = useState<StatsData>()
  const [hover, setHover] = useState<number | null>(null)

  useEffect(() => {
    api<StatsData>(`/stats?scope=${scope}`).then(setS, (e) => toast(e.message, true))
  }, [scope])

  if (!s) return <div class="page"><div class="spinner" /></div>

  // Last 30 days, including days without uploads.
  const days = Array.from({ length: 30 }, (_, i) => {
    const d = new Date(Date.now() - (29 - i) * 86400000).toISOString().slice(0, 10)
    return s.days.find((x) => x.day === d) ?? { day: d, count: 0, size: 0 }
  })
  const peak = Math.max(1, ...days.map((d) => d.count))

  return (
    <div class="page">
      <header class="page-head">
        <h1>Statistiques</h1>
        {me.user.is_admin && (
          <div class="chips">
            <button class={scope === 'server' ? 'chip active' : 'chip'} onClick={() => setScope('server')}>Serveur</button>
            <button class={scope === 'me' ? 'chip active' : 'chip'} onClick={() => setScope('me')}>Moi</button>
          </div>
        )}
      </header>

      <div class="kpis">
        <div class="card kpi"><span class="muted">Fichiers</span><strong>{s.count.toLocaleString('fr-FR')}</strong></div>
        <div class="card kpi"><span class="muted">Volume</span><strong>{size(s.size)}</strong></div>
        <div class="card kpi"><span class="muted">Vues</span><strong>{s.views.toLocaleString('fr-FR')}</strong></div>
        {scope === 'me' ? (
          <div class="card kpi"><span class="muted">Quota</span><strong>{me.quota > 0 ? size(me.quota) : 'Illimité'}</strong>
            {me.quota > 0 && <Meter used={me.used} total={me.quota} />}</div>
        ) : s.server && (
          <div class="card kpi"><span class="muted">Utilisateurs</span><strong>{s.server.users}</strong></div>
        )}
      </div>

      <section class="card">
        <h2>Envois sur 30 jours</h2>
        <div class="bars" aria-label="Nombre d'envois par jour sur 30 jours" onMouseLeave={() => setHover(null)}>
          {days.map((d, i) => (
            <div class={'bar' + (hover === i ? ' hover' : '')} tabIndex={0}
              aria-label={`${dayLabel(d.day)} : ${d.count} envoi(s), ${size(d.size)}`}
              onMouseEnter={() => setHover(i)} onFocus={() => setHover(i)} onBlur={() => setHover(null)} onClick={() => setHover(i)}>
              <div style={{ height: `${(d.count / peak) * 100}%` }} />
              {hover === i && (
                // Sits just above the bar top (kept inside the chart); edge bars flip their alignment so the tooltip stays on screen.
                <div class={'tooltip' + (i < 4 ? ' left' : i > 25 ? ' right' : '')} role="tooltip" style={{ '--h': `${(d.count / peak) * 100}%` }}>
                  <strong>{dayLabel(d.day)}</strong>
                  <span>{d.count} envoi{d.count > 1 ? 's' : ''}</span>
                  <span class="muted">{size(d.size)}</span>
                </div>
              )}
            </div>
          ))}
        </div>
      </section>

      <section class="card">
        <h2>Par type</h2>
        <ul class="rows">
          {Object.entries(s.types).map(([t, [c, sz]]) => (
            <li><span>{typeLabels[t] ?? t}</span><span class="muted">{c} · {size(sz)}</span></li>
          ))}
          {Object.keys(s.types).length === 0 && <li class="muted">Rien pour l'instant</li>}
        </ul>
      </section>

      {scope === 'server' && s.server && (
        <section class="card">
          <h2>Serveur</h2>
          <ul class="rows">
            <li><span>Version</span><span class="muted">{s.server.version} ({s.server.go})</span></li>
            <li><span>Disponibilité</span><span class="muted">{duration(s.server.uptime)}</span></li>
            <li><span>Stockage réel (avec versions légères)</span><span class="muted">{size(s.server.stored)}</span></li>
            <li><span>Économisé par la déduplication</span><span class="muted">{size(s.server.dedup_saved)}</span></li>
            <li><span>Disque libre</span><span class="muted">{size(s.server.disk_free)} / {size(s.server.disk_total)}</span></li>
            <li><span>Mémoire</span><span class="muted">{size(s.server.memory)} · {s.server.goroutines} goroutines</span></li>
            <li><span>Cache</span><span class="muted">{s.server.cache}</span></li>
            <li><span>Compression (ffmpeg)</span><span class="muted">{s.server.ffmpeg ? 'disponible' : 'absent'}</span></li>
          </ul>
          <Meter used={s.server.disk_total - s.server.disk_free} total={s.server.disk_total} />
        </section>
      )}
    </div>
  )
}
