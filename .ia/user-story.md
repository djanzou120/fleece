# Fleece — User Stories : Dashboard & Intégration API

**Version :** 1.0
**Documents de référence :** [PRD.md](./PRD.md) v1.0 · [TDD.md](./TDD.md) v1.0
**Périmètre :** Dashboard (Next.js / GraphQL) et Intégration API (REST publique)
**Dernière mise à jour :** Juin 2026

---

## Contexte global

### Personas (dérivés du PRD §4 et §7)

| Persona | Description | Objectifs principaux |
|---------|-------------|----------------------|
| **Dev (Développeur intégrateur)** | Développeur d'une startup SaaS, fintech ou e-commerce qui intègre Fleece dans son produit. Technique, pressé, veut une intégration < 30 min. | Envoyer des messages via une seule API, contrôler le routage, suivre les statuts. |
| **Admin (Administrateur de workspace)** | Responsable technique ou financier qui gère le compte, le wallet et la facturation. | Recharger le wallet, suivre la consommation, gérer les accès. |
| **Marketer (Marketeur)** 🟡 | Responsable marketing d'une agence ou d'un e-commerce. Peu technique, travaille depuis le dashboard. | Créer, planifier et suivre des campagnes de masse. |

### Objectifs métier (dérivés du PRD §3)

- **G1** Fournir une API unique multi-canal (simplification).
- **G2** Optimiser les coûts par sélection automatique du fournisseur.
- **G3** Maximiser la délivrabilité (routage + fallback).
- **G5** Offrir une expérience développeur de premier ordre (intégration < 30 min).

### Légende de priorité

🟢 MVP (P0) · 🟡 V1 (P1) · 🔵 V2 (P2)

---

# PARTIE A — INTÉGRATION API

---

## API-01 — Envoyer un message via l'API unifiée 🟢

### 1. User Story Format
- **As a** développeur intégrateur (Dev)
- **I want to** envoyer un message à un destinataire via un seul appel API
- **So that** je n'ai pas à intégrer et maintenir plusieurs fournisseurs SMS, WhatsApp, etc.

### 2. Story Details
- **Titre :** Envoi de message via API unifiée
- **Description :** Le Dev appelle `POST /v1/messages` avec un destinataire, un contenu et, optionnellement, une stratégie de canal. Fleece vérifie le solde, route le message vers le meilleur fournisseur, débite le wallet et renvoie immédiatement un accusé de réception.
- **Contexte persona :** Le Dev veut une API simple et prévisible, documentée, avec une réponse rapide.
- **Valeur métier :** Réalise G1 (simplification) et G2 (coûts) ; réduit la barrière à l'adoption.
- **Valeur utilisateur :** Une seule intégration pour tous les canaux ; moins de code à maintenir.

### 3. Acceptance Criteria
- **Happy path**
  - GIVEN un workspace avec un solde suffisant et une API Key valide
    WHEN le Dev envoie `POST /v1/messages` avec un destinataire et un contenu valides
    THEN l'API répond **202 Accepted** avec un `message_id` et un statut `queued` **en moins de 200 ms (P95)**.
  - Le message est ensuite acheminé de façon asynchrone vers le fournisseur sélectionné.
- **Edge cases**
  - Numéro/destinataire dans un pays non supporté → **422** avec un code d'erreur explicite.
  - Contenu dépassant la longueur max d'un canal → segmentation ou rejet documenté selon le canal.
  - Requête répétée avec le même `Idempotency-Key` → le même `message_id` est renvoyé, **aucun doublon** n'est créé ni débité.
- **Error states**
  - API Key absente/invalide/révoquée → **401 Unauthorized**.
  - Solde insuffisant → **402 Payment Required** avec `code: insufficient_funds`.
  - Dépassement du rate limit → **429 Too Many Requests** avec en-tête `Retry-After`.
  - Payload invalide → **400 Bad Request** avec détail du champ fautif.
- **Success conditions**
  - Le message progresse dans la machine à états (`queued → sent → delivered`) et chaque transition est observable via webhook ou `GET /v1/messages/{id}`.

### 4. User Flow
- **Entry point :** Le Dev a une API Key (voir DASH-02) et un wallet rechargé (voir DASH-03).
- **Étapes :**
  1. Construit la requête `POST /v1/messages` (destinataire, contenu, stratégie optionnelle, `Idempotency-Key`).
  2. Authentifie via l'en-tête `Authorization: Bearer <api_key>`.
  3. Reçoit `202` + `message_id`.
  4. Suit le statut via webhook ou polling.
- **Decision points :** mode d'envoi — **canal imposé** vs **sélection automatique** (voir API-02).
- **Alternative paths :** échec d'envoi → **fallback** automatique vers le canal suivant ; si tous échouent → `failed` + remboursement automatique.
- **Exit point :** message `delivered` (succès) ou `failed` (remboursé).

### 5. Technical Considerations
- **Exigences :** endpoint REST `POST /v1/messages` ; traitement **asynchrone** via RabbitMQ (TDD §5.2) ; idempotence via Redis.
- **Dépendances :** Identity (validation clé), Wallet (solde + débit), Routing (décision), Provider (envoi).
- **Contraintes :** P95 < 200 ms pour la réponse synchrone ; débit **avant** envoi ; remboursement automatique sur échec final.
- **Integration points :** Messaging Service ↔ Routing/Wallet/Provider ; Webhook Service pour notifications.

### 6. Design Requirements
- **API/DX :** documentation **Redocly** avec exemples copier-coller (curl + SDK) ; schémas de requête/réponse clairs ; codes d'erreur normalisés.
- **N/A UI** (intégration serveur), mais les exemples doivent permettre une intégration < 30 min.
- **Accessibilité :** documentation lisible, exemples multi-langages.

### 7. Success Metrics
- Temps médian d'intégration < 30 min (PRD §17).
- P95 de latence API < 200 ms.
- Taux de délivrabilité > 95 %.
- Taux d'erreur 4xx/5xx surveillé par workspace.

### 8. Related Stories
- **Dépend de :** DASH-02 (API Keys), DASH-03 (Wallet).
- **Relié à :** API-02 (stratégie de canal), API-03 (webhooks), API-04 (statut).
- **Follow-up :** support de canaux additionnels (Telegram 🟡, Messenger/RCS 🔵).

---

## API-02 — Définir la stratégie de canaux / routage 🟢

### 1. User Story Format
- **As a** développeur intégrateur (Dev)
- **I want to** définir une stratégie de routage ou un ordre de canaux pour mes messages
- **So that** je contrôle l'arbitrage entre coût, délivrabilité et vitesse.

### 2. Story Details
- **Titre :** Contrôle de la stratégie de routage
- **Description :** Le Dev passe une stratégie (`lowest_cost`, `highest_delivery`, `fastest`) ou un ordre de canaux personnalisé (`custom`) dans la requête d'envoi. Le Routing Service produit une liste ordonnée `(canal, fournisseur)` qui définit aussi la chaîne de fallback.
- **Contexte persona :** Le Dev a des cas d'usage différenciés (OTP critique vs marketing) et veut adapter la stratégie.
- **Valeur métier :** Réalise G2 (coûts) et G3 (délivrabilité).
- **Valeur utilisateur :** Flexibilité sans gérer la complexité fournisseur.

### 3. Acceptance Criteria
- **Happy path**
  - GIVEN une requête avec `strategy: "custom"` et `channels: ["whatsapp","sms","telegram"]`
    WHEN le message est envoyé
    THEN Fleece tente WhatsApp en premier, puis SMS, puis Telegram en cas d'échec.
  - GIVEN `strategy: "lowest_cost"` THEN le fournisseur le moins cher disponible pour le pays est choisi en premier.
- **Edge cases**
  - Canal demandé indisponible dans le pays du destinataire → ignoré, passage au suivant ; si aucun canal valide → **422**.
  - `strategy` absente → défaut documenté (ex. `highest_delivery`).
- **Error states**
  - Stratégie inconnue ou liste de canaux vide → **400 Bad Request**.
- **Success conditions**
  - L'ordre effectif des tentatives est traçable (visible dans `message_attempts` / réponse de statut).

### 4. User Flow
- **Entry point :** Dev construit la requête d'envoi.
- **Étapes :** choisit `strategy` ou `channels` → envoie → Routing produit la liste ordonnée → exécution avec fallback.
- **Decision point :** canal imposé (un seul canal) vs sélection automatique (stratégie).
- **Alternative paths :** fallback successif jusqu'à succès ou épuisement.
- **Exit point :** message acheminé selon la stratégie.

### 5. Technical Considerations
- **Dépendances :** Routing Service (TDD §4.3), Contact Intelligence 🟡 pour `highest_delivery`.
- **Contraintes :** la disponibilité canal/pays et les coûts doivent être à jour (cache Redis).
- **Integration points :** Routing ↔ Provider (`EstimateCost`).

### 6. Design Requirements
- **DX :** documentation claire des 4 stratégies et du comportement de fallback (exemple visuel WhatsApp → SMS → Telegram).

### 7. Success Metrics
- Coût moyen par message (KPI PRD §14).
- Taux d'échec après fallback.

### 8. Related Stories
- **Dépend de :** API-01.
- **Relié à :** Contact Intelligence 🟡, DASH-05 (analytics de routage).

---

## API-03 — Recevoir des webhooks d'événements 🟢

### 1. User Story Format
- **As a** développeur intégrateur (Dev)
- **I want to** recevoir des webhooks signés pour les événements de messages et de wallet
- **So that** mon système est informé en temps réel du statut sans polling.

### 2. Story Details
- **Titre :** Notifications webhook signées
- **Description :** Le Dev configure un endpoint (voir DASH-04). Fleece y envoie des événements signés (`message.*`, `wallet.*`) avec retries.
- **Contexte persona :** Le Dev veut une intégration événementielle fiable.
- **Valeur métier :** Renforce la DX (G5) et la confiance.
- **Valeur utilisateur :** Réactivité, pas de polling coûteux.

### 3. Acceptance Criteria
- **Happy path**
  - GIVEN un endpoint webhook configuré et actif
    WHEN un message passe à `delivered`
    THEN Fleece envoie `message.delivered` signé en **HMAC** avec un timestamp, et reçoit un `2xx`.
- **Edge cases**
  - Endpoint renvoie une erreur ou timeout → **retry avec backoff exponentiel** ; après N échecs → dead-letter et alerte dans le dashboard.
  - Ordre des événements non garanti → chaque payload est idempotent (le Dev gère via `message_id` + statut).
- **Error states**
  - Signature invalide côté client → le Dev doit rejeter (documenté).
- **Success conditions**
  - Tous les événements (`message.created/queued/sent/delivered/failed`, `wallet.debited/refunded`) sont délivrés au moins une fois.

### 4. User Flow
- **Entry point :** endpoint webhook configuré (DASH-04).
- **Étapes :** événement interne → Webhook Service signe → POST vers l'endpoint → vérification signature côté Dev → ack 2xx.
- **Alternative paths :** échec → retries → dead-letter.
- **Exit point :** événement acquitté ou abandonné après retries.

### 5. Technical Considerations
- **Dépendances :** Webhook Service (TDD §4.6), secret par endpoint.
- **Contraintes :** signature HMAC + anti-rejeu (timestamp) ; livraison « at least once ».
- **Integration points :** bus d'événements RabbitMQ → Webhook Service.

### 6. Design Requirements
- **DX :** documentation de vérification de signature ; exemples de payloads par événement.

### 7. Success Metrics
- Taux de livraison des webhooks (1er essai et après retries).
- Latence événement → livraison.

### 8. Related Stories
- **Dépend de :** DASH-04 (config endpoint), API-01.
- **Relié à :** API-04.

---

## API-04 — Consulter le statut d'un message 🟢

### 1. User Story Format
- **As a** développeur intégrateur (Dev)
- **I want to** interroger le statut d'un message par son identifiant
- **So that** je peux réconcilier l'état sans dépendre uniquement des webhooks.

### 2. Story Details
- **Titre :** Consultation de statut de message
- **Description :** `GET /v1/messages/{id}` renvoie le statut courant, l'historique des tentatives et le canal/fournisseur utilisé.
- **Valeur métier :** Fiabilité et transparence (G5).
- **Valeur utilisateur :** Filet de sécurité en cas de webhook manqué.

### 3. Acceptance Criteria
- **Happy path :** GIVEN un `message_id` valide du workspace WHEN `GET /v1/messages/{id}` THEN **200** avec statut, canal final, tentatives et horodatages.
- **Edge cases :** message d'un autre workspace → **404** (pas de fuite d'information).
- **Error states :** id inexistant → **404** ; clé invalide → **401**.
- **Success conditions :** le statut reflète la machine à états (TDD §6.1).

### 4. User Flow
- **Entry point :** Dev possède un `message_id`.
- **Étapes :** appel GET → réponse statut.
- **Exit point :** statut connu.

### 5. Technical Considerations
- **Dépendances :** Messaging Service (`messages`, `message_attempts`).
- **Integration points :** réconciliation possible via Provider `GetStatus`.

### 6. Design Requirements
- **DX :** schéma de réponse cohérent avec les événements webhook.

### 7. Success Metrics
- Usage du polling vs webhooks ; cohérence des statuts.

### 8. Related Stories
- **Dépend de :** API-01. **Relié à :** API-03.

---

# PARTIE B — DASHBOARD

---

## DASH-01 — Créer un workspace et s'onboarder 🟢

### 1. User Story Format
- **As a** administrateur (Admin)
- **I want to** créer un workspace et inviter mon équipe
- **So that** mon entreprise dispose d'un espace isolé pour gérer ses communications.

### 2. Story Details
- **Titre :** Création et onboarding du workspace
- **Description :** À l'inscription, un workspace unique est créé (utilisateurs, API Keys, wallet, webhooks). L'Admin complète l'onboarding (pays, premier rechargement, première clé).
- **Contexte persona :** Premier contact avec la plateforme ; doit être fluide.
- **Valeur métier :** Active le compte, point de départ de la monétisation (G5).
- **Valeur utilisateur :** Espace structuré et sécurisé.

### 3. Acceptance Criteria
- **Happy path :** GIVEN un nouvel utilisateur authentifié (Better Auth) WHEN il crée un workspace THEN le workspace est créé et il en est administrateur.
- **Edge cases :** email déjà associé à un workspace → invitation au lieu de création.
- **Error states :** échec d'authentification → message clair ; nom de workspace en doublon géré.
- **Success conditions :** un parcours d'onboarding guide vers DASH-02 (clé) et DASH-03 (wallet) ; objectif intégration < 30 min.

### 4. User Flow
- **Entry point :** page d'inscription.
- **Étapes :** inscription → création workspace → choix pays/marché → checklist d'onboarding.
- **Decision point :** créer un nouveau workspace vs rejoindre via invitation.
- **Exit point :** dashboard prêt à l'emploi.

### 5. Technical Considerations
- **Dépendances :** Identity Service (TDD §4.1), Better Auth.
- **Constraints :** isolation stricte des données par workspace.
- **Integration points :** GraphQL pour le dashboard.

### 6. Design Requirements
- **UI/UX :** checklist d'onboarding visible, état de progression.
- **Composants :** shadcn/ui (formulaires, cartes, stepper).
- **Responsive :** desktop d'abord, utilisable sur tablette.
- **Accessibilité :** WCAG 2.1 AA — navigation clavier, labels, contraste.

### 7. Success Metrics
- Taux d'achèvement de l'onboarding.
- Temps inscription → premier message envoyé.
- Nombre de workspaces actifs (KPI business).

### 8. Related Stories
- **Précède :** DASH-02, DASH-03.
- **Follow-up :** gestion des rôles/permissions, SSO 🔵.

---

## DASH-02 — Gérer les API Keys 🟢

### 1. User Story Format
- **As a** développeur intégrateur (Dev)
- **I want to** créer, faire tourner et révoquer des API Keys depuis le dashboard
- **So that** je sécurise l'accès programmatique à mon workspace.

### 2. Story Details
- **Titre :** Gestion du cycle de vie des API Keys
- **Description :** Le Dev génère une clé (affichée une seule fois), la révoque ou la fait tourner. Les clés sont stockées hachées.
- **Valeur métier :** Sécurité et confiance (PRD §9).
- **Valeur utilisateur :** Contrôle total des accès.

### 3. Acceptance Criteria
- **Happy path :** GIVEN un Dev dans un workspace WHEN il crée une clé THEN la clé en clair est affichée **une seule fois** et n'est plus jamais récupérable.
- **Edge cases :** rotation → ancienne clé valide pendant une fenêtre de grâce configurable puis révoquée.
- **Error states :** tentative d'usage d'une clé révoquée → **401** côté API.
- **Success conditions :** révocation prend effet immédiatement (cache invalidé) ; chaque action est tracée dans les audit logs.

### 4. User Flow
- **Entry point :** section « API Keys » du dashboard.
- **Étapes :** créer → copier la clé → utiliser ; révoquer/rotater au besoin.
- **Decision point :** révoquer immédiatement vs rotation avec grâce.
- **Exit point :** clé active et utilisable côté API.

### 5. Technical Considerations
- **Dépendances :** Identity Service ; cache Redis de validation des clés.
- **Constraints :** stockage haché, jamais en clair ; révocation propagée rapidement.
- **Integration points :** API Gateway consomme la validation.

### 6. Design Requirements
- **UI/UX :** affichage unique de la clé avec avertissement et bouton « copier » ; liste des clés avec date de dernière utilisation et statut.
- **Composants :** shadcn/ui (table, dialog, badge, toast).
- **Accessibilité :** WCAG 2.1 AA.

### 7. Success Metrics
- Délai création de clé → premier appel API réussi.
- Incidents de sécurité liés aux clés (cible : 0).

### 8. Related Stories
- **Dépend de :** DASH-01. **Relié à :** API-01.

---

## DASH-03 — Recharger et suivre le wallet 🟢

### 1. User Story Format
- **As a** administrateur (Admin)
- **I want to** recharger mon wallet et consulter mon historique de transactions
- **So that** je peux envoyer des messages et suivre ma consommation.

### 2. Story Details
- **Titre :** Rechargement et suivi du wallet
- **Description :** L'Admin recharge le wallet (**Mobile Money** sur les marchés africains, **Stripe** en Europe), consulte le solde et l'historique (débits, recharges, remboursements).
- **Valeur métier :** Modèle prépayé = revenu (KPI MRR, revenus par pays/canal).
- **Valeur utilisateur :** Transparence financière, pas de mauvaise surprise.

### 3. Acceptance Criteria
- **Happy path :** GIVEN un Admin dans un workspace au Cameroun WHEN il recharge via **Mobile Money** THEN le solde est crédité après confirmation du paiement et une transaction `credit` apparaît.
  - GIVEN un workspace en France WHEN il recharge via **Stripe** THEN même comportement via le moyen européen.
- **Edge cases :** paiement en attente/asynchrone → solde crédité seulement après confirmation ; paiement partiel/échoué → non crédité.
- **Error states :** échec du fournisseur de paiement → message clair, aucune écriture comptable erronée ; solde jamais négatif.
- **Success conditions :** historique cohérent et immuable (ledger append-only) ; remboursements automatiques visibles.

### 4. User Flow
- **Entry point :** section « Wallet ».
- **Étapes :** voir solde → recharger → choisir le moyen selon le pays → confirmer → solde mis à jour.
- **Decision point :** moyen de paiement déterminé par le pays du workspace.
- **Alternative paths :** paiement en attente → notification à la confirmation.
- **Exit point :** solde rechargé, prêt à envoyer.

### 5. Technical Considerations
- **Dépendances :** Wallet Service (TDD §4.5), adapter de paiement par pays (Mobile Money / Stripe).
- **Constraints :** opérations transactionnelles, intégrité comptable, idempotence des callbacks de paiement.
- **Integration points :** webhooks des fournisseurs de paiement ; émissions `wallet.debited`/`wallet.refunded`.

### 6. Design Requirements
- **UI/UX :** solde en évidence, graphique de consommation, table des transactions filtrable.
- **Composants :** shadcn/ui (card, table, chart, dialog de paiement).
- **Responsive :** consultation mobile du solde.
- **Accessibilité :** WCAG 2.1 AA ; montants et statuts lisibles aux lecteurs d'écran.

### 7. Success Metrics
- Taux de réussite des recharges par moyen de paiement.
- MRR, revenus par pays (KPI business).
- Temps solde épuisé → recharge.

### 8. Related Stories
- **Dépend de :** DASH-01. **Relié à :** API-01 (débit), DASH-05.

---

## DASH-04 — Configurer les endpoints webhook 🟢

### 1. User Story Format
- **As a** développeur intégrateur (Dev)
- **I want to** configurer et tester des endpoints webhook depuis le dashboard
- **So that** mon système reçoit les événements de façon fiable.

### 2. Story Details
- **Titre :** Configuration des webhooks
- **Description :** Le Dev ajoute une URL d'endpoint, sélectionne les événements souscrits, récupère le secret de signature et envoie un événement de test.
- **Valeur métier :** Fiabilité et DX (G5).
- **Valeur utilisateur :** Mise en place rapide et vérifiable.

### 3. Acceptance Criteria
- **Happy path :** GIVEN une URL HTTPS valide WHEN le Dev l'enregistre THEN un secret est généré et un événement de test peut être envoyé et tracé.
- **Edge cases :** URL non-HTTPS → refusée ; endpoint en échec persistant → marqué en erreur avec historique des tentatives.
- **Error states :** test échoué → diagnostic affiché (statut HTTP, latence).
- **Success conditions :** la liste des dernières livraisons (succès/échecs, retries) est consultable.

### 4. User Flow
- **Entry point :** section « Webhooks ».
- **Étapes :** ajouter URL → choisir événements → copier secret → tester → activer.
- **Decision point :** quels événements souscrire.
- **Exit point :** endpoint actif recevant des événements (API-03).

### 5. Technical Considerations
- **Dépendances :** Webhook Service ; `webhook_endpoints`, `webhook_deliveries`.
- **Constraints :** HTTPS obligatoire ; signature HMAC.
- **Integration points :** voir API-03.

### 6. Design Requirements
- **UI/UX :** journal de livraisons avec statut et possibilité de rejouer un événement ; affichage du secret avec masquage.
- **Composants :** shadcn/ui (form, table, badge, code block).
- **Accessibilité :** WCAG 2.1 AA.

### 7. Success Metrics
- Taux d'endpoints actifs sans échec persistant.
- Délai configuration → premier événement reçu.

### 8. Related Stories
- **Dépend de :** DASH-01. **Relié à :** API-03.

---

## DASH-05 — Visualiser les analytics de messagerie 🟡

### 1. User Story Format
- **As a** administrateur (Admin)
- **I want to** visualiser des tableaux de bord de délivrabilité, de coûts et de volumes
- **So that** je pilote la performance et la rentabilité de mes communications.

### 2. Story Details
- **Titre :** Tableaux de bord analytics
- **Description :** L'Admin consulte les KPIs : taux de délivrabilité, taux d'échec, temps moyen de livraison, coût moyen par message, répartition par pays/canal.
- **Valeur métier :** Visibilité (réduit le point de friction « faible visibilité » du PRD §2).
- **Valeur utilisateur :** Décisions éclairées sur le routage et le budget.

### 3. Acceptance Criteria
- **Happy path :** GIVEN un workspace avec de l'historique WHEN l'Admin ouvre Analytics THEN les KPIs s'affichent avec filtres période/pays/canal.
- **Edge cases :** workspace sans données → état vide explicite ; grandes plages → agrégats pré-calculés, pas de timeout.
- **Error states :** échec de chargement → message + retry, pas d'écran cassé.
- **Success conditions :** chiffres cohérents avec les statuts réels des messages.

### 4. User Flow
- **Entry point :** section « Analytics ».
- **Étapes :** choisir période/filtres → lire KPIs → exporter (option).
- **Decision point :** granularité (jour/canal/pays).
- **Exit point :** insight obtenu.

### 5. Technical Considerations
- **Dépendances :** Analytics Service (TDD §4.9), agrégats/vues matérialisées ; GraphQL.
- **Constraints :** requêtes performantes sur gros volumes.
- **Integration points :** consommation des événements de messages.

### 6. Design Requirements
- **UI/UX :** cartes KPI, graphiques temporels, tableaux de répartition.
- **Composants :** shadcn/ui + bibliothèque de charts.
- **Responsive :** lecture des KPIs clés sur mobile.
- **Accessibilité :** WCAG 2.1 AA ; graphiques avec alternatives textuelles/tabulaires.

### 7. Success Metrics
- Engagement avec le dashboard analytics.
- Corrélation usage analytics ↔ amélioration des stratégies de routage.

### 8. Related Stories
- **Relié à :** API-02, DASH-03. **Follow-up :** alertes/anomalies, optimisation IA 🔵.

---

## DASH-06 — Créer et planifier une campagne 🟡

### 1. User Story Format
- **As a** marketeur (Marketer)
- **I want to** créer, planifier et suivre une campagne de masse
- **So that** je touche mes clients sur le bon canal sans intervention technique.

### 2. Story Details
- **Titre :** Gestion de campagnes marketing
- **Description :** Le Marketer importe une liste de destinataires, rédige un message, choisit une stratégie de canal, planifie l'envoi et suit l'exécution.
- **Contexte persona :** Peu technique, travaille entièrement depuis le dashboard.
- **Valeur métier :** Cible « Agences Marketing » et « E-commerce » (PRD §4).
- **Valeur utilisateur :** Campagnes simples, mesurables, planifiables.

### 3. Acceptance Criteria
- **Happy path :** GIVEN une liste valide et un solde suffisant WHEN le Marketer planifie la campagne THEN elle s'exécute à l'heure prévue, décomposée en messages individuels respectant le rate limit.
- **Edge cases :** solde insuffisant pour l'ensemble → avertissement avant lancement et estimation du coût ; doublons dans la liste → dédupliqués.
- **Error states :** import de fichier invalide → erreurs ligne par ligne ; échec partiel → suivi par destinataire.
- **Success conditions :** statut de campagne (planifiée/en cours/terminée) et métriques de livraison consultables.

### 4. User Flow
- **Entry point :** section « Campaigns ».
- **Étapes :** importer destinataires → rédiger → choisir stratégie → estimer coût → planifier/lancer → suivre.
- **Decision point :** envoi immédiat vs planifié.
- **Alternative paths :** mise en pause/annulation avant exécution.
- **Exit point :** campagne terminée + rapport.

### 5. Technical Considerations
- **Dépendances :** Campaign Service (TDD §4.7), Messaging pipeline, Wallet (estimation/débit), rate limiting.
- **Constraints :** respect du rate limit du workspace ; décomposition idempotente.
- **Integration points :** réutilise API-01/API-02 en interne.

### 6. Design Requirements
- **UI/UX :** assistant multi-étapes (stepper), prévisualisation du message, estimation de coût en temps réel, suivi en direct.
- **Composants :** shadcn/ui (stepper, table, upload, progress, chart).
- **Responsive :** création desktop, suivi consultable sur mobile.
- **Accessibilité :** WCAG 2.1 AA.

### 7. Success Metrics
- Nombre de campagnes créées, volume de messages.
- Taux de délivrabilité par campagne.
- Rétention des marketeurs.

### 8. Related Stories
- **Dépend de :** DASH-03, API-02. **Relié à :** DASH-05.
- **Follow-up :** segmentation, A/B testing, personnalisation.

---

## Récapitulatif des dépendances

```text
DASH-01 (workspace)
   ├─▶ DASH-02 (API Keys) ─▶ API-01 (envoi)
   │                            ├─▶ API-02 (routage)
   │                            ├─▶ API-03 (webhooks) ◀─ DASH-04 (config webhooks)
   │                            └─▶ API-04 (statut)
   ├─▶ DASH-03 (wallet) ───────▶ API-01 (débit)
   ├─▶ DASH-05 (analytics) 🟡
   └─▶ DASH-06 (campagnes) 🟡 ─▶ réutilise API-01/API-02
```

---

*Document dérivé de [PRD.md](./PRD.md) et [TDD.md](./TDD.md). Personas, objectifs et fonctionnalités extraits de ces sources ; les placeholders de contexte de la demande ont été renseignés à partir des documents existants.*
