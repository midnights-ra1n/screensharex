export type User = {
  id: number
  username: string
  is_admin: boolean
  quota: number
  api_token?: string
  embed_title: string
  embed_description: string
  embed_color: string
  embed_site: string
  embed_raw: boolean
  created_at: number
}

export type Me = { user: User; used: number; count: number; quota: number; max_upload: number; base_url: string }

export type Media = {
  id: number
  slug: string
  filename: string
  mime: string
  size: number
  lite_mime: string
  lite_size: number
  views: number
  created_at: number
}

export type Links = { url: string; raw: string; thumb: string; delete_url: string }
export type Item = { media: Media; links: Links }

export async function api<T = any>(path: string, opts: { method?: string; body?: unknown } = {}): Promise<T> {
  const res = await fetch('/api' + path, {
    method: opts.method ?? (opts.body === undefined ? 'GET' : 'POST'),
    headers: opts.body === undefined ? {} : { 'Content-Type': 'application/json' },
    body: opts.body === undefined ? undefined : JSON.stringify(opts.body),
  })
  const data = await res.json().catch(() => ({}))
  if (!res.ok) throw new Error(data.error ?? `Erreur ${res.status}`)
  return data
}

// XHR rather than fetch: fetch has no upload progress events.
export function uploadFile(file: File, onProgress: (p: number) => void): Promise<any> {
  return new Promise((resolve, reject) => {
    const xhr = new XMLHttpRequest()
    const form = new FormData()
    form.append('file', file)
    xhr.open('POST', '/api/upload')
    xhr.upload.onprogress = (e) => e.lengthComputable && onProgress(e.loaded / e.total)
    xhr.onload = () => {
      let data: any = {}
      try { data = JSON.parse(xhr.responseText) } catch {}
      xhr.status === 200 ? resolve(data) : reject(new Error(data.error ?? `Erreur ${xhr.status}`))
    }
    xhr.onerror = () => reject(new Error('Connexion perdue'))
    xhr.send(form)
  })
}

export function size(n: number): string {
  if (n < 1024) return `${n} o`
  const units = ['Ko', 'Mo', 'Go', 'To']
  let i = -1
  do { n /= 1024; i++ } while (n >= 1024 && i < units.length - 1)
  return `${n.toFixed(n < 10 ? 1 : 0)} ${units[i]}`
}

export const date = (ts: number) =>
  new Date(ts * 1000).toLocaleDateString('fr-FR', { day: 'numeric', month: 'short', year: 'numeric' })

export const kind = (mime: string) =>
  mime === 'image/gif' ? 'gif' : mime.startsWith('image/') ? 'image' : mime.startsWith('video/') ? 'video' : mime.startsWith('audio/') ? 'audio' : 'file'
