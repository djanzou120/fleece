// Couche 3 — Adapter GraphQL : point d'entrée des résolveurs.
// Fusionne les résolveurs workspace, wallet et webhook en un map { Query, Mutation, ... }
// injecté dans le serveur GraphQL (Apollo / Yoga).

import type { GetWorkspaceOverview } from "../../../application/use-cases/get-workspace-overview.js";
import type { ListApiKeys } from "../../../application/use-cases/list-api-keys.js";
import type { ManageWebhook } from "../../../application/use-cases/manage-webhook.js";
import type { IdentityClient, WalletClient, MessagingClient } from "../../../application/ports/output/clients.js";

import { buildWorkspaceResolvers } from "./workspace.resolver.js";
import { buildWalletResolvers } from "./wallet.resolver.js";
import { buildWebhookResolvers } from "./webhook.resolver.js";

// ---------------------------------------------------------------------------
// Types des dépendances globales
// ---------------------------------------------------------------------------

export interface ResolverDeps {
  getWorkspaceOverview: GetWorkspaceOverview;
  listApiKeys: ListApiKeys;
  manageWebhook: ManageWebhook;
  identityClient: IdentityClient;
  walletClient: WalletClient;
  messagingClient: MessagingClient;
}

// ---------------------------------------------------------------------------
// buildResolvers — fusionne les 3 groupes de résolveurs
// ---------------------------------------------------------------------------

/**
 * Construit la map complète des résolveurs GraphQL à partir des dépendances injectées.
 * La map est directement passée au serveur GraphQL (Apollo Server, GraphQL Yoga, etc.).
 *
 * Structure retournée :
 *   { Query, Mutation, WebhookEndpoint }
 *
 * WebhookEndpoint contient les résolveurs de champ (secretPreview) qui masquent
 * les données sensibles avant qu'elles ne quittent le BFF.
 */
export function buildResolvers(deps: ResolverDeps) {
  const workspace = buildWorkspaceResolvers({
    getWorkspaceOverview: deps.getWorkspaceOverview,
    listApiKeys: deps.listApiKeys,
    identityClient: deps.identityClient,
  });

  const wallet = buildWalletResolvers({
    walletClient: deps.walletClient,
    messagingClient: deps.messagingClient,
  });

  const webhook = buildWebhookResolvers({
    manageWebhook: deps.manageWebhook,
  });

  // Fusion des groupes Query, Mutation et résolveurs de type de chaque sous-module
  return {
    Query: {
      ...workspace.Query,
      ...wallet.Query,
      ...webhook.Query,
    },
    Mutation: {
      ...workspace.Mutation,
      ...webhook.Mutation,
    },
    // Résolveurs de champ pour WebhookEndpoint (masquage du secret)
    WebhookEndpoint: webhook.WebhookEndpoint,
  };
}
