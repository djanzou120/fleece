# Fleece - Product Requirements Document (PRD)

**Version:** 1.0
**Status:** Draft
**Author:** Product Team
**Last Updated:** June 2026

---

# 1. Executive Summary

## Vision

Fleece est une plateforme de communication omnicanale API-first permettant aux entreprises d'intégrer une seule API afin de communiquer avec leurs utilisateurs via plusieurs canaux de messagerie.

La plateforme sélectionne automatiquement le meilleur fournisseur, le meilleur canal et la meilleure stratégie de routage afin d'optimiser les coûts et maximiser la délivrabilité.

---

## Mission

Permettre aux développeurs d'envoyer un message à n'importe quel destinataire sans avoir à gérer :

* les fournisseurs SMS ;
* les APIs WhatsApp ;
* les différences géographiques ;
* les mécanismes de fallback ;
* les problématiques de délivrabilité.

---

## Proposition de valeur

> Une seule API pour envoyer chaque message sur le meilleur canal, au meilleur coût, avec le meilleur taux de délivrance.

---

# 2. Problem Statement

## Situation actuelle

Les entreprises doivent aujourd'hui intégrer plusieurs plateformes afin de communiquer avec leurs utilisateurs :

* SMS
* WhatsApp
* Telegram
* Facebook Messenger
* Email
* RCS

Chaque canal possède :

* sa propre API ;
* ses propres tarifs ;
* ses propres contraintes techniques ;
* ses propres mécanismes de délivrabilité.

Cette fragmentation augmente fortement la complexité technique et les coûts opérationnels.

---

## Points de friction

### Pour les entreprises

* Multiplication des intégrations.
* Maintenance de plusieurs fournisseurs.
* Gestion complexe des coûts.
* Difficulté à garantir la délivrabilité.
* Faible visibilité sur les performances.

### Pour les utilisateurs finaux

* Messages non reçus.
* Utilisation d'un canal inadapté.
* Expérience utilisateur dégradée.

---

## Impact métier

* Augmentation des coûts.
* Réduction du taux de conversion.
* Échec des notifications critiques.
* Temps de développement plus élevé.

---

# 3. Product Goals

## Objectifs principaux

### G1 - Simplification

Fournir une API unique permettant d'envoyer des messages via plusieurs canaux.

### G2 - Optimisation des coûts

Sélectionner automatiquement le fournisseur le plus avantageux selon le pays.

### G3 - Maximisation de la délivrabilité

Utiliser des mécanismes de routage et de fallback intelligents.

### G4 - Scalabilité

Permettre l'envoi de millions de messages par jour.

### G5 - Expérience développeur

Proposer une intégration simple avec une documentation complète.

---

# 4. Target Market

## Marchés prioritaires

### Phase 1

* Cameroun
* Côte d'Ivoire
* Sénégal
* France

### Phase 2

* Afrique francophone
* Europe

---

## Cibles

### Startups SaaS

* OTP
* Notifications
* Alertes

### Fintech

* Validation de transaction
* Sécurité
* Authentification

### E-commerce

* Confirmation de commande
* Livraison
* Marketing

### Agences Marketing

* Campagnes en masse
* Segmentation
* Reporting

---

# 5. Product Overview

## Fonctionnement général

Le client envoie un message via l'API Fleece.

Fleece :

1. Vérifie le solde du compte.
2. Évalue les canaux disponibles.
3. Sélectionne le fournisseur optimal.
4. Débite le wallet.
5. Envoie le message.
6. Suit sa délivrance.
7. Déclenche les webhooks.
8. Met à jour la base Contact Intelligence.

---

# 6. Core Concepts

## Workspace

Chaque entreprise possède un workspace unique.

Le workspace contient :

* utilisateurs ;
* API Keys ;
* wallet ;
* campagnes ;
* statistiques ;
* webhooks.

---

## Wallet

Chaque workspace dispose d'un portefeuille prépayé.

Le wallet permet :

* rechargement ;
* débit automatique ;
* remboursements.

---

## Contact Intelligence

Le système maintient une base de connaissances des contacts basée sur :

* les canaux connus ;
* les historiques d'envoi ;
* les succès ;
* les échecs ;
* les performances observées.

Exemple :

```json
{
  "phone": "+2376XXXXXXX",
  "known_channels": [
    "sms",
    "whatsapp"
  ],
  "delivery_score": 98
}
```

---

# 7. User Stories

## Messaging

### US-001

En tant que développeur,

Je souhaite envoyer un message via une API unique,

Afin de ne pas gérer plusieurs fournisseurs.

---

### US-002

En tant que développeur,

Je souhaite définir une priorité de canaux,

Afin de contrôler la stratégie d'envoi.

---

### US-003

En tant que développeur,

Je souhaite recevoir des webhooks,

Afin d'être informé du statut des messages.

---

## Campaigns

### US-004

En tant que marketeur,

Je souhaite envoyer une campagne en masse,

Afin de contacter mes clients.

---

### US-005

En tant que marketeur,

Je souhaite planifier une campagne,

Afin qu'elle soit exécutée ultérieurement.

---

## Billing

### US-006

En tant qu'administrateur,

Je souhaite consulter mes dépenses,

Afin de suivre ma consommation.

---

# 8. Functional Requirements

## Messaging API

### Envoi de message

Support :

* SMS
* WhatsApp
* Telegram
* Messenger

Modes :

* canal imposé
* sélection automatique

---

### Routage intelligent

Le moteur doit pouvoir choisir :

* le meilleur canal ;
* le meilleur fournisseur ;
* la meilleure stratégie.

---

### Stratégies supportées

#### Lowest Cost

Minimiser le coût.

#### Highest Delivery

Maximiser la délivrabilité.

#### Fastest

Minimiser le temps de livraison.

#### Custom

Ordre défini par le client.

Exemple :

```json
{
  "channels": [
    "whatsapp",
    "sms",
    "telegram"
  ]
}
```

---

### Fallback

Exemple :

```text
WhatsApp
↓
SMS
↓
Telegram
```

---

## Campaign Management

Fonctionnalités :

* création ;
* planification ;
* exécution ;
* suivi.

---

## Webhooks

Événements :

* message.created
* message.queued
* message.sent
* message.delivered
* message.failed
* wallet.debited
* wallet.refunded

---

## Wallet

Fonctionnalités :

* recharge ;
* débit ;
* remboursement ;
* historique.

---

## API Keys

Fonctionnalités :

* création ;
* révocation ;
* rotation.

---

## Rate Limiting

Limitation configurable :

* requêtes/minute ;
* messages/minute.

---

# 9. Non Functional Requirements

## Performance

Temps de réponse API :

* P95 < 200 ms

---

## Disponibilité

Objectif :

99.9 %

---

## Scalabilité

Capacité cible :

* 10 millions de messages/jour

---

## Sécurité

* HTTPS obligatoire
* Chiffrement des secrets
* Audit logs
* Signature des webhooks

---

# 10. Technical Architecture

## Style architectural

Microservices Event-Driven

---

## API publique

REST

Documentation :

Redocly

---

## Dashboard

GraphQL

> Le service GraphQL interne (BFF) servant le dashboard est développé en **TypeScript**.

---

## Backend

Go (cœur métier)

> Exceptions : le **service d'authentification** (Identity) et le **service GraphQL interne** sont développés en **TypeScript**.

---

## Frontend

Next.js

shadcn/ui

---

## Base de données

PostgreSQL

---

## Queue

RabbitMQ

---

## Cache

Redis

---

## Authentification

Better Auth (implémenté en **TypeScript** dans le service d'authentification)

API Keys

---

# 11. Services

## Identity Service

Implémentation : **TypeScript + Better Auth**.

Responsabilités :

* utilisateurs ;
* workspace ;
* authentification.

---

## Messaging Service

Responsabilités :

* création des messages ;
* orchestration.

---

## Routing Service

Responsabilités :

* sélection des canaux ;
* sélection des fournisseurs.

---

## Provider Service

Responsabilités :

* intégration des fournisseurs.

---

## Campaign Service

Responsabilités :

* campagnes marketing.

---

## Wallet Service

Responsabilités :

* facturation ;
* débit ;
* remboursement.

---

## Analytics Service

Responsabilités :

* métriques ;
* rapports.

---

## Contact Intelligence Service

Responsabilités :

* scoring ;
* historique.

---

## Webhook Service

Responsabilités :

* notifications externes.

---

# 12. Provider Model

Chaque fournisseur implémente :

```go
type Provider interface {
    Send(ctx context.Context, message Message) error
    EstimateCost(ctx context.Context, message Message) Money
    GetStatus(ctx context.Context, id string) Status
}
```

---

# 13. Pricing Model

## Principe

Prix final :

```text
Prix Fournisseur
+
Marge Fournisseur
+
Frais Plateforme
```

---

## Facturation

### Débit

Avant envoi.

### Remboursement

Automatique en cas d'échec.

---

# 14. KPIs

## Produit

* Taux de délivrabilité
* Taux d'échec
* Temps moyen de livraison
* Coût moyen par message

## Business

* MRR
* Nombre de workspaces actifs
* Messages envoyés
* Revenus par pays
* Revenus par canal

---

# 15. Roadmap

## MVP (P0)

### Canaux

* WhatsApp
* SMS

### Fonctionnalités

* API REST
* Wallet
* Dashboard
* Webhooks
* Routing
* Fallback

Durée estimée : 3 mois

---

## V1 (P1)

### Ajouts

* Contact Intelligence
* Telegram
* Campagnes marketing
* Analytics avancées

Durée estimée : 3 mois

---

## V2 (P2)

### Ajouts

* SSO
* Messenger
* RCS
* Optimisation IA du routage

Durée estimée : 6 mois

---

# 16. Risks

## Dépendance fournisseurs

Mitigation :

Multi-provider.

---

## Conformité réglementaire

Mitigation :

Audit juridique régulier.

---

## Mauvaise délivrabilité

Mitigation :

Scoring fournisseurs.

---

## Explosion du volume

Mitigation :

Architecture distribuée.

---

## Fraude

Mitigation :

Rate limiting, quotas, monitoring.

---

# 17. Success Criteria

Fleece sera considéré comme un succès lorsque :

* 100 entreprises utilisent la plateforme.
* 1 million de messages sont envoyés par mois.
* Le taux de délivrabilité dépasse 95 %.
* La marge brute reste positive sur tous les marchés.
* L'intégration client prend moins de 30 minutes.

```
```

