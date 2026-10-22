J'aimerais créer un clone de XBackBone, une sorte de Imgur mais auto hébergé qui apporte le support de ShareX pour les captures d'écran, enregistrements d'écran, GIF etc...

# Stack

Je voudrais une stack moderne comme d'habitude et sans PHP. J'aimerais avoir un frontend écrit en TypeScript et un backend en Golang, ainsi qu'une base SQLite.

J'aimerais aussi une interface web moderne et claire, optimisée pour mobile.

# Fonctionnalités

Les fonctionnalités à mettre seront :
- Belle interface moderne avec ce qui a été uploadé, les statistiques serveur etc...
- Pouvoir uploader depuis l'interface web une image, GIF ou vidéo
- Ajouter une limite d'upload
- Dédier un quota d'espace pour les utilisateurs
- Ajouter une taille limite d'upload
- Compresser les images/vidéos/GIF pour les utilisateurs avec mauvaise connexion/sur données mobile
- Empreinte unique sur les fichiers pour éviter les doublons
- Support d'une version lite de Redis (si ça existe) pour une mise en cache rapide
- Gestion multi users. Ajoute aussi la possibilité si on peut se créer ou non un compte sur le serveur
- Embed personnalisé. Pour Discord et autres qui permet d'envoyer un lien de la photo et la charger directement dans l'application de messagerie plutot que de forcer son ouverture dans un navigateur web. Possibilité de créer un embed (pour Discord) personnalisé qui va contenir le contenu.
- Support de différentes apps de capture d'écran :
    - ShareX
    - Xerahs
    - ScreenCloud
    - ishare
    - Spectacle (KDE)
    - macOS (Share sheet)
    - CLI script
    - et d'autres outils. Pour les autres outils on peut fournir une API
- Export et import complet de l'ensemble des médias présents sur le serveur et également pour la base de données
- Système de mises à jour. Pour le paquet .deb sur Debian/Ubuntu, dans l'interface web mettre un bouton qui va rechercher les mises à jour, proposer de les installer et redémarrer le serveur sous sa nouvelle version.

# Plateformes

Je voudrais un serveur qui s'exécute sur Debian/Ubuntu sous la forme d'un paquet binaire et aussi une image Docker. Le tout pour x86_64 et image Docker. Le runner GitHub Action devra être sous Ubuntu 24.04.
