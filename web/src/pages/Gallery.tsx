import { useEffect, useRef, useState } from 'preact/hooks'
import { api, date, kind, size, uploadFile, type Item, type Me } from '../api'
import { attempt, copy, getSaver, Icon, Meter, setSaver, toast } from '../ui'

type Upload = { name: string; progress: number; error?: string }

export function Gallery({ me, refresh }: { me: Me; refresh: () => void }) {
  const [items, setItems] = useState<Item[]>([])
  const [more, setMore] = useState(false)
  const [page, setPage] = useState(0)
  const [q, setQ] = useState('')
  const [type, setType] = useState('')
  const [uploads, setUploads] = useState<Upload[]>([])
  const [drag, setDrag] = useState(false)
  const [open, setOpen] = useState<Item | null>(null)
  const [saver, setSaverState] = useState(getSaver())
  const input = useRef<HTMLInputElement>(null)

  const load = async (p = 0) => {
    const res = await api<{ items: Item[]; more: boolean }>(`/media?page=${p}&q=${encodeURIComponent(q)}&type=${type}`)
    setItems((old) => (p === 0 ? res.items : [...old, ...res.items]))
    setMore(res.more)
    setPage(p)
  }
  useEffect(() => {
    const t = setTimeout(() => load(0).catch((e) => toast(e.message, true)), 200)
    return () => clearTimeout(t)
  }, [q, type])

  const send = async (files: FileList | File[]) => {
    const list = [...files]
    for (const f of list) {
      if (f.size > me.max_upload) {
        toast(`${f.name} : trop volumineux (max ${size(me.max_upload)})`, true)
        continue
      }
      const entry: Upload = { name: f.name, progress: 0 }
      setUploads((u) => [...u, entry])
      try {
        const res = await uploadFile(f, (p) => { entry.progress = p; setUploads((u) => [...u]) })
        if (list.length === 1) copy(res.url)
      } catch (e) {
        toast(`${f.name} : ${(e as Error).message}`, true)
      }
      setUploads((u) => u.filter((x) => x !== entry))
    }
    load(0)
    refresh()
  }

  // Paste a screenshot straight from the clipboard (through a ref so the listener sees current state).
  const sendRef = useRef(send)
  sendRef.current = send
  useEffect(() => {
    const onPaste = (e: ClipboardEvent) => {
      const files = e.clipboardData?.files
      if (files?.length) sendRef.current(files)
    }
    addEventListener('paste', onPaste)
    return () => removeEventListener('paste', onPaste)
  }, [])

  const remove = (it: Item) => {
    if (!confirm(`Supprimer ${it.media.filename} ?`)) return
    attempt(async () => {
      await api(`/media/${it.media.id}`, { method: 'DELETE' })
      setItems((l) => l.filter((x) => x !== it))
      setOpen(null)
      refresh()
    }, 'Supprimé')
  }

  const toggleSaver = () => { setSaver(!saver); setSaverState(!saver) }

  return (
    <div
      class="page"
      onDragOver={(e) => { e.preventDefault(); setDrag(true) }}
      onDragLeave={(e) => e.currentTarget === e.target && setDrag(false)}
      onDrop={(e) => { e.preventDefault(); setDrag(false); e.dataTransfer?.files.length && send(e.dataTransfer.files) }}
    >
      <header class="page-head">
        <div>
          <h1>Médias</h1>
          <p class="muted">{me.count} fichiers · {size(me.used)}{me.quota > 0 && ` sur ${size(me.quota)}`}</p>
          {me.quota > 0 && <Meter used={me.used} total={me.quota} />}
        </div>
        <button class="primary" onClick={() => input.current?.click()}><Icon name="upload" /> Envoyer</button>
        <input ref={input} type="file" multiple accept="image/*,video/*,audio/*" hidden
          onChange={(e) => { const f = e.currentTarget.files; f && send(f); e.currentTarget.value = '' }} />
      </header>

      <div class={'dropzone' + (drag ? ' active' : '')} onClick={() => input.current?.click()}>
        <Icon name="upload" />
        <span>Glissez-déposez, collez (Ctrl+V) ou touchez pour envoyer · max {size(me.max_upload)}</span>
      </div>

      {uploads.map((u) => (
        <div class="upload-row">
          <span>{u.name}</span>
          <div class="meter"><div style={{ width: `${u.progress * 100}%` }} /></div>
        </div>
      ))}

      <div class="toolbar">
        <label class="search">
          <Icon name="search" />
          <input placeholder="Rechercher" value={q} onInput={(e) => setQ(e.currentTarget.value)} aria-label="Rechercher" />
        </label>
        <div class="chips">
          {[['', 'Tout'], ['image', 'Images'], ['video', 'Vidéos'], ['audio', 'Audio']].map(([v, l]) => (
            <button class={type === v ? 'chip active' : 'chip'} onClick={() => setType(v)}>{l}</button>
          ))}
          <button class={saver ? 'chip active' : 'chip'} onClick={toggleSaver} title="Charger les versions compressées">
            <Icon name="leaf" /> Éco data
          </button>
        </div>
      </div>

      {items.length === 0 ? (
        <div class="empty">
          <Icon name="grid" />
          <p>Aucun média pour l'instant.</p>
          <p class="muted">Envoyez un fichier ou configurez un client dans l'onglet <a href="#/clients">Clients</a>.</p>
        </div>
      ) : (
        <div class="grid">
          {items.map((it) => (
            <button class="tile" onClick={() => setOpen(it)} aria-label={it.media.filename}>
              <Thumb it={it} saver={saver} />
              <span class="tile-meta">
                <span class="name">{it.media.filename}</span>
                <span class="muted">{size(it.media.size)}</span>
              </span>
            </button>
          ))}
        </div>
      )}
      {more && <button class="ghost wide" onClick={() => load(page + 1)}>Charger plus</button>}

      {open && (
        <div class="modal" onClick={(e) => e.target === e.currentTarget && setOpen(null)} role="dialog" aria-modal="true" aria-label={open.media.filename}>
          <div class="modal-body">
            <button class="icon-btn close" onClick={() => setOpen(null)} aria-label="Fermer"><Icon name="close" /></button>
            <Preview it={open} saver={saver} />
            <div class="modal-info">
              <strong>{open.media.filename}</strong>
              <span class="muted">
                {size(open.media.size)}
                {open.media.lite_size > 0 && ` · version légère ${size(open.media.lite_size)}`} · {open.media.views} vues · {date(open.media.created_at)}
              </span>
              <div class="actions">
                <button class="primary" onClick={() => copy(open.links.url)}><Icon name="copy" /> Copier le lien</button>
                <button onClick={() => copy(open.links.raw)}><Icon name="copy" /> Lien direct</button>
                <a class="button" href={open.links.url} target="_blank" rel="noopener"><Icon name="external" /> Ouvrir</a>
                <a class="button" href={open.links.raw} download={open.media.filename}><Icon name="download" /> Télécharger</a>
                <button class="danger" onClick={() => remove(open)}><Icon name="trash" /> Supprimer</button>
              </div>
            </div>
          </div>
        </div>
      )}
    </div>
  )
}

function Thumb({ it, saver }: { it: Item; saver: boolean }) {
  const k = kind(it.media.mime)
  const liteIsVideo = it.media.lite_mime.startsWith('video/')
  if (k === 'image' || (k === 'gif' && !liteIsVideo)) return <img src={it.links.thumb} loading="lazy" alt="" />
  if (k === 'video' || k === 'gif')
    return (
      <span class="vid">
        <video src={it.links.thumb + '#t=0.1'} preload={saver ? 'none' : 'metadata'} muted playsInline />
        <span class="badge">{k === 'gif' ? 'GIF' : <Icon name="play" />}</span>
      </span>
    )
  return <span class="file-thumb"><Icon name="file" /><span>{it.media.mime}</span></span>
}

function Preview({ it, saver }: { it: Item; saver: boolean }) {
  const src = saver ? it.links.thumb : it.links.raw
  const mime = saver && it.media.lite_mime ? it.media.lite_mime : it.media.mime
  if (mime.startsWith('image/')) return <img src={src} alt={it.media.filename} />
  if (mime.startsWith('video/')) return <video src={src} controls autoPlay loop playsInline />
  if (mime.startsWith('audio/')) return <audio src={src} controls />
  return <span class="file-thumb big"><Icon name="file" /><span>{it.media.mime}</span></span>
}
