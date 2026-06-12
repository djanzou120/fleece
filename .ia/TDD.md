# Fleece — Technical Design Document (TDD)

**Version :** 1.0
**Statut :** Draft
**Document de référence :** [PRD.md](./PRD.md) v1.0
**Dernière mise à jour :** Juin 2026
**Public visé :** Équipe d'ingénierie, architectes, parties prenantes produit

---

## 1. Introduction

### 1.1 Objet du document

Ce document décrit la conception technique de **Fleece**, plateforme de communication omnicanale *API-first*. Il traduit les exigences fonctionnelles et non fonctionnelles du [PRD](./PRD.md) en une architecture concrète : modules, flux de données, parcours utilisateur, choix technologiques et contrats entre services.

Il sert de référence partagée entre les développeurs (qui y trouvent les responsabilités et interfaces de chaque service) et les parties prenantes (qui y trouvent la vision d'ensemble et la justification des décisions).

### 1.2 Portée

Le document couvre **l'architecture cible complète**. Les composants qui ne font pas partie du MVP sont explicitement étiquetés :

- 🟢 **MVP (P0)** — livré dans les 3 premiers mois
- 🟡 **V1 (P1)** — Contact Intelligence, Telegram, campagnes, analytics avancées
- 🔵 **V2 (P2)** — SSO, Messenger, RCS, optimisation IA du routage

### 1.3 Glossaire

| Terme | Définition |
|-------|-----------|
| **Workspace** | Espace isolé appartenant à une entreprise cliente (utilisateurs, clés API, wallet, campagnes). |
| **Channel** | Canal de messagerie : SMS, WhatsApp, Telegram, Messenger, Email, RCS. |
| **Provider** | Fournisseur tiers acheminant le message sur un canal donné (ex. opérateur SMS, Meta WhatsApp). |
| **Routing strategy** | Algorithme de sélection canal/fournisseur (lowest cost, highest delivery, fastest, custom). |
| **Fallback** | Repli automatique vers le canal/fournisseur suivant en cas d'échec. |
| **Wallet** | Portefeuille prépayé du workspace. |
| **Delivery score** | Indice de délivrabilité d'un contact, maintenu par Contact Intelligence. |

---

## 2. Objectifs du système

### 2.1 Objectifs (issus du PRD)

| Réf. | Objectif | Traduction technique |
|------|----------|----------------------|
| **G1 — Simplification** | Une API unique multi-canal | API REST publique unifiée + abstraction `Provider` |
| **G2 — Optimisation des coûts** | Sélection du fournisseur le plus avantageux par pays | Routing Service avec moteur de coût par pays/canal |
| **G3 — Délivrabilité** | Routage et fallback intelligents | Stratégies de routage + chaîne de fallback + scoring |
| **G4 — Scalabilité** | Millions de messages/jour | Architecture microservices event-driven, traitement asynchrone par queue |
| **G5 — DX** | Intégration simple, doc complète | API REST documentée (Redocly), webhooks signés, SDK |

### 2.2 Exigences non fonctionnelles directrices

| Domaine | Cible | Implication de conception |
|---------|-------|---------------------------|
| Performance | Réponse API **P95 < 200 ms** | L'envoi est **asynchrone** : l'API accuse réception et délègue à la queue. |
| Disponibilité | **99.9 %** | Services sans état, réplicas multiples, pas de point de défaillance unique. |
| Scalabilité | **10 M messages/jour** (~115/s soutenu, pics supérieurs) | Découplage par RabbitMQ, scaling horizontal des workers. |
| Sécurité | HTTPS, secrets chiffrés, audit logs, webhooks signés | TLS partout, chiffrement au repos des secrets, signature HMAC des webhooks. |

### 2.3 Critères de succès techniques

- Intégration client < 30 min (qualité de la DX).
- Taux de délivrabilité > 95 %.
- Marge brute positive sur tous les marchés (le pricing doit être calculé de façon fiable avant chaque envoi).

---

## 3. Vue d'ensemble de l'architecture

### 3.1 Style architectural

**Microservices event-driven.** Les services communiquent de deux manières :

- **Synchrone (REST interne)** pour les requêtes nécessitant une réponse immédiate (auth, vérification de solde, devis de coût). Décision : REST interne (cohérence avec l'API publique, simplicité d'outillage), pas de gRPC.
- **Asynchrone (événements via RabbitMQ)** pour tout le pipeline d'envoi et les effets de bord (débit, webhooks, scoring).

Ce découplage est ce qui permet de tenir simultanément la cible *P95 < 200 ms* (réponse rapide côté API) et *10 M messages/jour* (absorption des pics par la queue).

### 3.2 Diagramme de haut niveau

```text
                          ┌──────────────────────────┐
        Clients API ─────▶│      API Gateway          │
   (REST + API Key)       │  authn · rate limit · TLS │
                          └─────────────┬────────────┘
                                        │
   Dashboard          ┌─────────────────────────┐
   (Next.js) ─GraphQL─▶│  GraphQL Gateway (BFF)  │  ◀── TypeScript
                       │   agrège les services    │
                       └────────────┬─────────────┘
                                    │ (REST interne)
                                    ▼
                                    ┌─────────────────────────────────────────┐
                                    │            Couche Services               │
                                    │   (Go, sauf Identity = TypeScript)       │
   ┌──────────┐  ┌──────────────┐  ┌──────────────┐  ┌─────────────────────┐  │
   │ Identity │  │  Messaging   │  │   Routing    │  │   Provider Service  │  │
   │ (TS +    │  │  Service     │  │   Service    │  │ (adapters fournisseur)│ │
   │ Better   │  │              │  │              │  │                     │  │
   │  Auth)   │  │              │  │              │  │                     │  │
   └────┬─────┘  └──────┬───────┘  └──────┬───────┘  └──────────┬──────────┘  │
        │               │                 │                     │             │
   ┌────┴─────┐  ┌───────┴──────┐  ┌───────┴──────┐  ┌───────────┴─────────┐  │
   │  Wallet  │  │  Campaign    │  │   Webhook    │  │ Contact Intelligence│  │
   │ Service  │  │  Service 🟡  │  │   Service    │  │     Service 🟡      │  │
   └──────────┘  └──────────────┘  └──────────────┘  └─────────────────────┘  │
        │               │                                                     │
   ┌────┴───────────────┴──────┐                                             │
   │      Analytics Service 🟡  │                                             │
   └───────────────────────────┘                                             │
                                    └────────────────────────────────────────┘
                                        │            │            │
                                  ┌─────┴───┐  ┌─────┴────┐  ┌────┴─────┐
                                  │PostgreSQL│  │ RabbitMQ │  │  Redis   │
                                  └──────────┘  └──────────┘  └──────────┘
```

### 3.3 Stack technologique

| Couche | Technologie | Rôle |
|--------|-------------|------|
| Backend (cœur métier) | **Go** | Microservices du pipeline (Messaging, Routing, Provider, Wallet, Webhook, Campaign, Contact Intelligence, Analytics) |
| Service d'authentification | **TypeScript + Better Auth** | Identity Service : utilisateurs, workspaces, sessions, API Keys |
| Service GraphQL interne (BFF) | **TypeScript** | Couche GraphQL servant le dashboard, agrège les services métier |
| API publique | **REST** + doc **Redocly** | Surface d'intégration client |
| API Dashboard | **GraphQL** | Requêtes riches du frontend (servies par le service GraphQL TypeScript) |
| Frontend | **Next.js** + **shadcn/ui** | Dashboard workspace |
| Base de données | **PostgreSQL** | Base **unifiée** (une instance), **un schéma logique par service** — voir [ARCHITECTURE.md §2.1](./ARCHITECTURE.md) |
| Message broker | **RabbitMQ** | Bus d'événements, pipeline d'envoi |
| Cache | **Redis** | Cache, rate limiting, idempotence, verrous |
| Authentification | **Better Auth** (dashboard) + **API Keys** (API) | Sessions utilisateurs et accès machine |

> **Choix d'implémentation polyglotte.** Le cœur du pipeline d'envoi est en **Go** (performance, concurrence). Deux services sont en **TypeScript** : le **service d'authentification** (Identity Service), bâti sur **Better Auth**, et le **service GraphQL interne** (BFF) qui sert le dashboard. Ce choix tire parti de l'écosystème Better Auth/TypeScript pour l'auth et le GraphQL côté produit, tout en gardant Go pour la partie haute performance. La communication inter-services reste **REST interne**, indépendante du langage.

### 3.4 Principes transverses

- **Isolation des données par service** : base PostgreSQL **unifiée** mais **un schéma par service** ; chaque service n'accède qu'à son propre schéma, pas d'accès direct aux tables d'un autre service. Voir [ARCHITECTURE.md §2.1](./ARCHITECTURE.md).
- **Idempotence** : tout endpoint d'envoi accepte une clé d'idempotence (`Idempotency-Key`) stockée en Redis pour éviter les doublons en cas de retry réseau.
- **Cohérence éventuelle** : l'état d'un message converge via les événements ; les consommateurs sont idempotents.
- **Observabilité** : logs structurés, traçage distribué (trace-id propagé de l'API jusqu'au provider), métriques exposées pour les KPIs.

---

## 4. Modules principaux (services)

Chaque service est autonome, déployable indépendamment, propriétaire de ses données et de ses événements.

### 4.1 Identity Service 🟢

**Responsabilités :** utilisateurs, workspaces, authentification, clés API.
**Implémentation :** **TypeScript + Better Auth** (seul service d'auth de la plateforme).

- Création et gestion des workspaces (un workspace par entreprise).
- Gestion des utilisateurs et de leurs rôles au sein d'un workspace.
- Authentification dashboard via **Better Auth** (sessions) ; SSO en 🔵 V2.
- Cycle de vie des **API Keys** : création, révocation, rotation.
- Validation des clés API consommée par l'API Gateway (résultat mis en cache Redis).
- Expose une API **REST interne** consommée par les autres services (Go ou TypeScript), indépendante du langage.

**Données clés :** `workspaces`, `users`, `api_keys` (hash de la clé, jamais en clair), `audit_logs`.

### 4.2 Messaging Service 🟢

**Responsabilités :** point d'entrée logique de l'envoi, création et orchestration des messages.

- Reçoit la demande d'envoi validée par l'API Gateway.
- Crée l'entité `message` (statut initial `created`) et émet `message.created`.
- Orchestre le cycle de vie : interroge Routing pour la décision, demande le débit au Wallet, délègue l'envoi via le Provider Service, applique le **fallback** si échec.
- Tient à jour la machine à états du message (voir §6).

**Données clés :** `messages`, `message_attempts` (une ligne par tentative canal/fournisseur).

### 4.3 Routing Service 🟢

**Responsabilités :** décision de routage — meilleur canal, meilleur fournisseur, meilleure stratégie.

- Applique la stratégie demandée :
  - **Lowest Cost** — minimise le coût (via `EstimateCost` des providers).
  - **Highest Delivery** — maximise la délivrabilité (scores fournisseur + delivery score du contact 🟡).
  - **Fastest** — minimise le temps de livraison.
  - **Custom** — ordre de canaux imposé par le client.
- Produit une **liste ordonnée** `(canal, fournisseur)` qui définit la chaîne de fallback.
- Consomme les coûts par pays/canal, la disponibilité des fournisseurs et le scoring.

**Données clés :** tables de tarification fournisseur par pays/canal, scores fournisseur, règles de routage par workspace. Lectures fréquentes mises en cache Redis.

### 4.4 Provider Service 🟢

**Responsabilités :** intégration concrète des fournisseurs tiers.

- Implémente l'interface `Provider` unifiée (voir §7) : `Send`, `EstimateCost`, `GetStatus`.
- Un **adapter** par fournisseur encapsule les spécificités d'API, de format et d'erreurs.
- Reçoit les callbacks/DLR (delivery receipts) des fournisseurs et émet `message.delivered` / `message.failed`.
- Canaux : SMS + WhatsApp 🟢, Telegram 🟡, Messenger + RCS 🔵.

**Données clés :** `providers`, `provider_credentials` (chiffrés), `provider_messages` (mapping id interne ↔ id fournisseur).

### 4.5 Wallet Service 🟢

**Responsabilités :** portefeuille prépayé, facturation.

- Rechargement, débit automatique **avant envoi**, remboursement **automatique en cas d'échec**, historique.
- **Moyens de recharge par marché** : **Mobile Money** sur les marchés africains (Cameroun, Côte d'Ivoire, Sénégal) ; **Stripe** sur le marché européen (France). Un adapter de paiement par fournisseur, sélectionné selon le pays du workspace.
- Calcul du prix final : `prix fournisseur + marge fournisseur + frais plateforme`.
- Garantit l'intégrité comptable : opérations transactionnelles, solde jamais négatif, registre immuable des transactions.

**Données clés :** `wallets`, `wallet_transactions` (ledger append-only : `debit`, `credit`, `refund`).

### 4.6 Webhook Service 🟢

**Responsabilités :** notifications sortantes vers les clients.

- S'abonne aux événements internes et délivre les webhooks HTTP aux endpoints des workspaces.
- **Signature HMAC** de chaque payload (exigence sécurité du PRD).
- Retries avec backoff exponentiel, file de morts (dead-letter) pour les échecs persistants.
- Événements exposés : `message.created`, `message.queued`, `message.sent`, `message.delivered`, `message.failed`, `wallet.debited`, `wallet.refunded`.

**Données clés :** `webhook_endpoints`, `webhook_deliveries` (statut, tentatives).

### 4.7 Campaign Service 🟡

**Responsabilités :** campagnes marketing de masse.

- Création, planification, exécution, suivi.
- Décompose une campagne en messages individuels injectés dans le pipeline Messaging, en respectant le rate limiting du workspace.
- Planification (envoi différé) via scheduler interne.

**Données clés :** `campaigns`, `campaign_recipients`, `campaign_runs`.

### 4.8 Contact Intelligence Service 🟡

**Responsabilités :** base de connaissances des contacts (scoring, historique).

- Maintient pour chaque contact : canaux connus, historique d'envoi, succès/échecs, `delivery_score`.
- Alimente le Routing Service pour la stratégie *Highest Delivery*.
- Mis à jour de façon asynchrone à partir des événements de livraison.

**Exemple d'enregistrement :**

```json
{
  "phone": "+2376XXXXXXX",
  "known_channels": ["sms", "whatsapp"],
  "delivery_score": 98
}
```

### 4.9 Analytics Service 🟡

**Responsabilités :** métriques et rapports.

- Agrège les événements pour produire les KPIs produit et business (PRD §14).
- Sert le dashboard via GraphQL.

**Données clés :** tables agrégées / vues matérialisées par workspace, pays, canal.

### 4.10 GraphQL Gateway (BFF) 🟢

**Responsabilités :** servir le dashboard via une API GraphQL unique.
**Implémentation :** **TypeScript** (même écosystème que l'Identity Service).

- Expose le schéma GraphQL consommé par le frontend Next.js.
- Agrège et orchestre les appels **REST internes** vers les services métier (Identity, Wallet, Messaging, Analytics, etc.).
- Applique l'authentification de session (via Better Auth / Identity Service) pour chaque requête du dashboard.
- Sans état propre : ne possède pas de schéma de base de données dédié, il compose les données des services.

---

## 5. Parcours utilisateur

### 5.1 Parcours développeur — intégration et premier envoi

```text
1. Inscription ─▶ création du workspace (Identity Service)
2. Recharge du wallet (Wallet Service)
3. Génération d'une API Key depuis le dashboard (Identity Service)
4. (Optionnel) Configuration d'un endpoint webhook
5. Premier appel POST /v1/messages
6. Réception de la réponse 202 (message accepté, statut « queued »)
7. Suivi du statut via webhooks ou GET /v1/messages/{id}
```

Cible : **moins de 30 minutes** de l'inscription au premier message délivré.

### 5.2 Flux d'envoi d'un message (cœur du système) 🟢

Référence : section 5 du PRD. Le flux ci-dessous détaille la collaboration entre services.

```text
Client ──POST /v1/messages──▶ API Gateway
   │  (authn API Key, rate limit, validation, Idempotency-Key)
   ▼
API Gateway ──▶ Messaging Service
   1. Crée le message (statut: created) ──▶ émet « message.created »
   2. Vérifie le solde (Wallet Service)               ── si insuffisant ▶ 402
   3. Demande la décision de routage (Routing Service)
        ◀── liste ordonnée [(canal, fournisseur), …]  (chaîne de fallback)
   4. Débite le wallet (Wallet Service) ──▶ émet « wallet.debited »
   5. Répond 202 au client (statut: queued) ──▶ émet « message.queued »
   ─────────────────────────────────────────────────  (≤ 200 ms ici)
   ── à partir d'ici, traitement asynchrone via RabbitMQ ──
   6. Worker consomme l'envoi ──▶ Provider Service.Send()
        ├─ succès ▶ statut « sent » ──▶ émet « message.sent »
        └─ échec  ▶ FALLBACK : tentative suivante de la liste
                    si liste épuisée ▶ statut « failed »
                                       ──▶ émet « message.failed »
                                       ──▶ Wallet refund ▶ « wallet.refunded »
   7. DLR fournisseur ──▶ Provider Service ──▶ « message.delivered »
   8. Contact Intelligence 🟡 met à jour le score à partir des événements
   9. Webhook Service notifie le client à chaque transition
```

**Points de conception clés :**

- Le **débit a lieu avant l'envoi** (PRD §13), et un **remboursement automatique** est déclenché en cas d'échec final.
- La **réponse au client est immédiate** (202) ; la livraison réelle est asynchrone, ce qui protège la cible P95 < 200 ms.
- Le **fallback** consomme la liste ordonnée produite par le Routing Service jusqu'à succès ou épuisement.

### 5.3 Parcours marketeur — campagne 🟡

```text
1. Création de la campagne + import des destinataires (Campaign Service)
2. Choix de la stratégie de routage et planification éventuelle
3. À l'heure prévue : décomposition en messages individuels
4. Injection dans le pipeline Messaging (en respectant le rate limit)
5. Suivi en temps réel (Analytics Service / dashboard)
```

### 5.4 Parcours administrateur — facturation

```text
1. Consultation du solde et de l'historique du wallet (Wallet Service)
2. Recharge
3. Suivi des dépenses par pays/canal (Analytics Service 🟡)
```

---

## 6. Modèle de données et cycle de vie

### 6.1 Machine à états d'un message

```text
 created ──▶ queued ──▶ sent ──▶ delivered
                 │         │
                 │         └──(DLR négatif)──▶ failed
                 └──(toutes tentatives échouées)──▶ failed ──▶ (refund)
```

| Statut | Signification | Événement émis |
|--------|---------------|----------------|
| `created` | Message enregistré | `message.created` |
| `queued` | Accepté, débité, en file | `message.queued`, `wallet.debited` |
| `sent` | Remis au fournisseur | `message.sent` |
| `delivered` | Confirmé livré (DLR) | `message.delivered` |
| `failed` | Échec après fallback | `message.failed`, `wallet.refunded` |

### 6.2 Propriété des données par service

| Service | Entités principales |
|---------|--------------------|
| Identity | `workspaces`, `users`, `api_keys`, `audit_logs` |
| Messaging | `messages`, `message_attempts` |
| Routing | `provider_pricing`, `provider_scores`, `routing_rules` |
| Provider | `providers`, `provider_credentials`, `provider_messages` |
| Wallet | `wallets`, `wallet_transactions` |
| Webhook | `webhook_endpoints`, `webhook_deliveries` |
| Campaign 🟡 | `campaigns`, `campaign_recipients`, `campaign_runs` |
| Contact Intelligence 🟡 | `contacts`, `contact_channel_history` |
| Analytics 🟡 | vues / agrégats |

> Base unifiée, **un schéma par service** : chaque service détient exclusivement les tables de son schéma. Les besoins de données croisées passent par API interne ou événements, jamais par accès direct au schéma d'un autre service. Voir [ARCHITECTURE.md §2.1](./ARCHITECTURE.md).

---

## 7. Modèle Provider

L'extensibilité multi-fournisseur (mitigation du risque « dépendance fournisseurs » du PRD) repose sur une interface unique. Ajouter un fournisseur ou un canal = écrire un nouvel adapter, sans toucher au reste du système.

```go
type Provider interface {
    Send(ctx context.Context, message Message) error
    EstimateCost(ctx context.Context, message Message) Money
    GetStatus(ctx context.Context, id string) Status
}
```

- `Send` — achemine le message via le fournisseur.
- `EstimateCost` — fournit le coût utilisé par le Routing Service (stratégie *Lowest Cost*) et le Wallet (pricing).
- `GetStatus` — permet la réconciliation de statut (en complément des DLR push).

**Stratégie de tarification (PRD §13) :** `prix final = prix fournisseur + marge fournisseur + frais plateforme`, calculé avant le débit.

---

## 8. Préoccupations transverses

### 8.1 Sécurité

| Exigence (PRD) | Mise en œuvre |
|----------------|---------------|
| HTTPS obligatoire | TLS terminé à l'API Gateway, mTLS optionnel entre services internes. |
| Chiffrement des secrets | Credentials fournisseurs et API Keys chiffrés au repos ; API Keys stockées hachées. |
| Audit logs | Identity Service journalise les actions sensibles (clés, accès, wallet). |
| Signature des webhooks | HMAC par payload, secret par endpoint, horodatage anti-rejeu. |

### 8.2 Rate limiting & anti-fraude

- Limites configurables par workspace : **requêtes/minute** et **messages/minute** (PRD §8).
- Appliquées à l'API Gateway via compteurs Redis.
- Combinées à des quotas et au monitoring pour la mitigation de la fraude (PRD §16).

### 8.3 Scalabilité & résilience

- Services **sans état**, scalables horizontalement.
- **Déploiement sur Kubernetes** : chaque service est un déploiement indépendant, scaling horizontal automatique (HPA) piloté par la charge ; les workers d'envoi scalent sur la profondeur des files RabbitMQ.
- **RabbitMQ** absorbe les pics ; les workers d'envoi scalent indépendamment de l'API.
- **Redis** pour idempotence, verrous distribués et cache des décisions de routage.
- Cible : 10 M messages/jour, disponibilité 99.9 %.

### 8.4 Observabilité

- Logs structurés + **trace-id** propagé de l'API jusqu'au fournisseur.
- Métriques alimentant directement les **KPIs** : taux de délivrabilité, taux d'échec, temps moyen de livraison, coût moyen par message.

---

## 9. Découpage par phases (roadmap technique)

| Phase | Services / capacités | Canaux |
|-------|----------------------|--------|
| 🟢 **MVP (P0)** — 3 mois | Identity, Messaging, Routing, Provider, Wallet, Webhook, Dashboard, Fallback | WhatsApp, SMS |
| 🟡 **V1 (P1)** — 3 mois | Contact Intelligence, Campaign, Analytics avancées | + Telegram |
| 🔵 **V2 (P2)** — 6 mois | SSO, optimisation IA du routage | + Messenger, RCS |

---

## 10. Risques techniques et mitigations

| Risque (PRD §16) | Mitigation technique |
|------------------|----------------------|
| Dépendance fournisseurs | Interface `Provider` + multi-provider + fallback automatique. |
| Conformité réglementaire | Isolation par workspace, audit logs, chiffrement ; revues juridiques régulières. |
| Mauvaise délivrabilité | Scoring fournisseur + Contact Intelligence + stratégie *Highest Delivery*. |
| Explosion du volume | Pipeline asynchrone RabbitMQ, workers scalables, services sans état. |
| Fraude | Rate limiting Redis, quotas, monitoring, débit prépayé. |

---

## 11. Décisions d'architecture

### 11.1 Décisions tranchées

| # | Décision | Choix retenu |
|---|----------|--------------|
| 1 | Communication inter-services synchrone | **REST interne** (cohérence avec l'API publique, pas de gRPC). |
| 2 | Stratégie de déploiement | **Kubernetes** (un déploiement par service, scaling horizontal automatique). |
| 3 | Recharge du wallet | **Mobile Money** (marchés africains) et **Stripe** (Europe), via adapter de paiement par pays. |
| 4 | Souveraineté des données / RGPD | **Reportée** : à traiter lors du passage au marché européen. |

### 11.2 Questions encore ouvertes

1. **SDK clients** : langages prioritaires pour accélérer l'objectif « intégration < 30 min ».
2. **Réconciliation de statut** : fréquence du polling `GetStatus` en complément des DLR push.

---

*Document dérivé de [PRD.md](./PRD.md) v1.0. Toute évolution des exigences produit doit être répercutée ici.*
