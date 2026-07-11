// Couche 2 — Ports pilotés : interfaces des clients de services (REST interne).
// Le BFF n'a pas de domaine propre ; il orchestre les services métier via ces ports.
// Les implémentations concrètes se trouvent dans adapters/clients/.
// Cette couche ne dépend QUE de @fleece/api-common (types purs).

import type { ApiContext, Page, PageArgs } from "@fleece/api-common";

// ---------------------------------------------------------------------------
// DTOs retournés par les clients (types de transfert — pas d'entités domaine)
// ---------------------------------------------------------------------------

export interface WorkspaceDTO {
  id: string;
  name: string;
  country: string;
  slug: string;
  plan: string;
  createdAt: string; // ISO 8601
}

export interface ApiKeyDTO {
  id: string;
  workspaceId: string;
  name: string;
  prefix: string;
  active: boolean;
  createdAt: string; // ISO 8601
  expiresAt: string | null;
}

export interface WalletBalanceDTO {
  workspaceId: string;
  balance: number;    // montant en centimes (ou unité monétaire la plus petite)
  currency: string;   // ex. "XOF", "EUR"
}

export interface TransactionDTO {
  id: string;
  workspaceId: string;
  /** Type de transaction : "credit" | "debit" | "refund" — mappé depuis `kind` du backend Go. */
  type: "credit" | "debit" | "refund";
  /** Montant en centimes. */
  amount: number;
  /** Devise ISO 4217 — récupérée via getBalance (le backend Go ne la retourne pas par transaction). */
  currency: string;
  /** Description lisible dérivée du kind + message_id. */
  description: string;
  createdAt: string; // ISO 8601
}

export interface MessageDTO {
  id: string;
  workspaceId: string;
  to: string;
  channel: string;  // "sms" | "whatsapp" | ...
  content: string;
  status: string;   // "pending" | "sent" | "delivered" | "failed"
  createdAt: string; // ISO 8601
  updatedAt: string; // ISO 8601
}

export interface WebhookEndpointDTO {
  id: string;
  workspaceId: string;
  url: string;
  events: string[];
  active: boolean;
  createdAt: string; // ISO 8601
  /**
   * Secret de signature brut (HMAC) retourné par le Webhook Service.
   * NE JAMAIS exposer en clair via GraphQL — le résolveur `secretPreview` masque cette valeur.
   */
  secret?: string;
}

// ---------------------------------------------------------------------------
// Ports de sortie (interfaces pilotées)
// ---------------------------------------------------------------------------

/**
 * Client vers le service Identity (auth-api).
 * Agrège les informations de workspace et de clés API.
 */
export interface IdentityClient {
  /** Récupère un workspace par son identifiant ; null si absent. */
  getWorkspace(ctx: ApiContext, workspaceId: string): Promise<WorkspaceDTO | null>;

  /**
   * Crée un nouveau workspace.
   * TODO(production) : auth-api doit dériver ownerEmail depuis le token de session
   * transmis dans l'en-tête Authorization (service-to-service).
   */
  createWorkspace(ctx: ApiContext, name: string, country: string): Promise<WorkspaceDTO>;

  /** Liste toutes les clés API d'un workspace. */
  listApiKeys(ctx: ApiContext, workspaceId: string): Promise<ApiKeyDTO[]>;

  /**
   * Crée une nouvelle clé API.
   * Retourne la clé brute (à afficher une seule fois) et les métadonnées.
   */
  createApiKey(
    ctx: ApiContext,
    workspaceId: string,
    name: string,
  ): Promise<{ rawKey: string; apiKey: ApiKeyDTO }>;

  /** Révoque une clé API existante. */
  revokeApiKey(ctx: ApiContext, workspaceId: string, keyId: string): Promise<void>;

  /**
   * Effectue la rotation d'une clé API : révoque l'ancienne et crée une nouvelle.
   * Retourne la nouvelle clé brute (à afficher une seule fois) et les métadonnées.
   * Route auth-api : POST /workspaces/{workspaceId}/api-keys/{keyId}/rotate
   */
  rotateApiKey(
    ctx: ApiContext,
    workspaceId: string,
    keyId: string,
  ): Promise<{ rawKey: string; apiKey: ApiKeyDTO }>;
}

/**
 * Client vers le Wallet Service (Go).
 * Consulte le solde et l'historique des transactions.
 */
export interface WalletClient {
  /** Récupère le solde du wallet d'un workspace. */
  getBalance(ctx: ApiContext, workspaceId: string): Promise<WalletBalanceDTO>;

  /**
   * Liste les transactions paginées (cursor-based).
   * La pagination est appliquée côté BFF sur le tableau complet retourné par le backend.
   * Le backend Go répond avec un tableau brut ; le BFF mappe et pagine.
   */
  listTransactions(
    ctx: ApiContext,
    workspaceId: string,
    page: PageArgs,
  ): Promise<Page<TransactionDTO>>;
}

/**
 * Client vers le Messaging Service (Go).
 * Consulte les messages envoyés.
 */
export interface MessagingClient {
  /** Liste les messages paginés d'un workspace. */
  listMessages(
    ctx: ApiContext,
    workspaceId: string,
    page: PageArgs,
  ): Promise<Page<MessageDTO>>;

  /** Récupère un message par son identifiant ; null si absent. */
  getMessage(
    ctx: ApiContext,
    workspaceId: string,
    id: string,
  ): Promise<MessageDTO | null>;
}

/**
 * Client vers le Webhook Service (Go).
 * Gère les endpoints de webhooks d'un workspace.
 */
export interface WebhookClient {
  /** Liste tous les endpoints webhook d'un workspace. */
  listEndpoints(ctx: ApiContext, workspaceId: string): Promise<WebhookEndpointDTO[]>;

  /**
   * Crée un nouvel endpoint webhook.
   * @param payload - URL de destination et liste des événements abonnés
   */
  createEndpoint(
    ctx: ApiContext,
    workspaceId: string,
    payload: { url: string; events: string[] },
  ): Promise<WebhookEndpointDTO>;

  /** Supprime un endpoint webhook. */
  deleteEndpoint(ctx: ApiContext, workspaceId: string, id: string): Promise<void>;
}
