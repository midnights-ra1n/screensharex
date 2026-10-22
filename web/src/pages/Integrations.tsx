import { useState } from 'preact/hooks'
import { api, type Me } from '../api'
import { attempt, copy, Icon } from '../ui'

const dl = (name: string) => `/api/clients/${name}`

export function Integrations({ me, refresh }: { me: Me; refresh: () => void }) {
  const [show, setShow] = useState(false)
  const token = me.user.api_token ?? ''
  const base = me.base_url

  const regen = () => {
    if (!confirm('Régénérer le jeton ? Tous vos clients configurés devront être mis à jour.')) return
    attempt(async () => { await api('/me/token', { method: 'POST' }); refresh() }, 'Nouveau jeton généré')
  }

  const clients: { name: string; platform: string; files: [string, string][]; steps: any[] }[] = [
    {
      name: 'ShareX', platform: 'Windows',
      files: [['sharex.sxcu', 'Configuration .sxcu']],
      steps: ['Téléchargez le fichier et double-cliquez dessus : ShareX l\'importe automatiquement.',
        'Dans « Destinations », choisissez « Custom image/file uploader » → ScreenShareX.'],
    },
    {
      name: 'XerahS', platform: 'Windows · macOS · Linux',
      files: [['xerahs.sxcu', 'Configuration .sxcu']],
      steps: ['XerahS lit le format ShareX : ouvrez le fichier ou importez-le dans Destinations → Custom uploaders.',
        'Sélectionnez-le comme destination des images et fichiers.'],
    },
    {
      name: 'ishare', platform: 'macOS',
      files: [['ishare.iscu', 'Configuration .iscu']],
      steps: ['Ouvrez le fichier .iscu : ishare ajoute l\'uploader personnalisé.',
        'Dans les réglages d\'ishare, choisissez-le comme uploader actif.'],
    },
    {
      name: 'Spectacle', platform: 'KDE Plasma',
      files: [['screensharex-upload', 'Script d\'envoi'], ['screensharex.desktop', 'Entrée .desktop']],
      steps: [
        <>Installez le script et l'entrée : <code>install -Dm755 screensharex-upload ~/.local/bin/screensharex-upload && install -Dm644 screensharex.desktop ~/.local/share/applications/screensharex.desktop</code></>,
        'Dans Spectacle : Exporter → Ouvrir avec → ScreenShareX. Le lien est copié (wl-copy / xclip) et une notification s\'affiche.',
      ],
    },
    {
      name: 'ScreenCloud', platform: 'Windows · macOS · Linux',
      files: [['screensharex-upload', 'Script d\'envoi']],
      steps: ['Installez le script (voir Spectacle) puis, dans ScreenCloud, activez le service « Shell Script ».',
        <>Commande : <code>~/.local/bin/screensharex-upload {'{s}'}</code> — le lien est copié dans le presse-papiers.</>],
    },
    {
      name: 'macOS (feuille de partage)', platform: 'macOS · iOS',
      files: [],
      steps: [
        'Ouvrez Raccourcis et créez un raccourci, puis activez « Afficher dans la feuille de partage » (images, vidéos, fichiers).',
        <>Ajoutez « Obtenir le contenu de l'URL » : <code>{base}/api/upload</code>, méthode POST, en-tête <code>Authorization: Bearer &lt;jeton&gt;</code>, corps « Formulaire » avec un champ fichier <code>file</code> = Entrée du raccourci.</>,
        'Ajoutez « Obtenir la valeur du dictionnaire » (clé url) puis « Copier dans le presse-papiers ».',
      ],
    },
    {
      name: 'Ligne de commande', platform: 'Linux · macOS · WSL',
      files: [['screensharex-upload', 'Script d\'envoi']],
      steps: [<>Utilisation : <code>screensharex-upload capture.png video.mp4</code> — affiche et copie le lien de chaque fichier.</>,
        <>Les variables <code>SSX_URL</code> et <code>SSX_TOKEN</code> remplacent les valeurs intégrées.</>],
    },
  ]

  return (
    <div class="page">
      <header class="page-head"><h1>Clients de capture</h1></header>

      <section class="card">
        <h2>Jeton d'API</h2>
        <p class="muted">Les fichiers ci-dessous contiennent votre jeton : ne les partagez pas.</p>
        <div class="token">
          <code>{show ? token : '•'.repeat(26)}</code>
          <button onClick={() => setShow(!show)}>{show ? 'Masquer' : 'Afficher'}</button>
          <button onClick={() => copy(token)}><Icon name="copy" /> Copier</button>
          <button class="danger" onClick={regen}>Régénérer</button>
        </div>
      </section>

      <div class="clients">
        {clients.map((c) => (
          <section class="card client">
            <h2>{c.name}</h2>
            <span class="pill">{c.platform}</span>
            <ol>{c.steps.map((s) => <li>{s}</li>)}</ol>
            <div class="actions">
              {c.files.map(([f, label]) => <a class="button" href={dl(f)} download={f}><Icon name="download" /> {label}</a>)}
            </div>
          </section>
        ))}
      </div>

      <section class="card">
        <h2>API pour les autres outils</h2>
        <p class="muted">N'importe quel outil capable d'envoyer un formulaire multipart peut téléverser :</p>
        <pre>{`curl -H "Authorization: Bearer <jeton>" -F "file=@capture.png" ${base}/api/upload`}</pre>
        <ul class="rows api">
          <li><span>Authentification</span><span class="muted">En-tête <code>Authorization: Bearer</code>, <code>X-API-Key</code> ou champ de formulaire <code>token</code></span></li>
          <li><span>Champ fichier</span><span class="muted"><code>file</code> (sinon le premier fichier du formulaire)</span></li>
          <li><span>Réponse JSON</span><span class="muted"><code>url</code>, <code>raw</code>, <code>thumb</code>, <code>delete_url</code>, <code>media</code>, <code>duplicate</code></span></li>
          <li><span>Erreurs</span><span class="muted"><code>{'{"error": "…"}'}</code> — 401, 413 (taille/quota), 429 (limite d'envoi)</span></li>
          <li><span>Suppression</span><span class="muted"><code>DELETE delete_url</code> (ou ouvrir le lien dans un navigateur)</span></li>
        </ul>
      </section>
    </div>
  )
}
