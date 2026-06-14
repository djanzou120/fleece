// Composition root du service graphql-api (GraphQL Gateway / BFF Dashboard).
// Câble l'infrastructure, les adapters et les use cases (Clean Architecture).
// Point d'entrée compilé par esbuild (voir mk/node.mk).
// DI manuelle — pas de framework DI.

// -------------------------------------------------------------------------
// Couche 4 — Infrastructure : configuration
// -------------------------------------------------------------------------

import { loadConfig } from "./infrastructure/config.js";
import { HttpSessionValidator } from "./infrastructure/session-validator.js";
import { startServer } from "./infrastructure/server.js";

// -------------------------------------------------------------------------
// Couche 3 — Adapters : clients REST (driven)
// -------------------------------------------------------------------------

import { IdentityRestClient } from "./adapters/clients/identity.client.js";
import { WalletRestClient } from "./adapters/clients/wallet.client.js";
import { MessagingRestClient } from "./adapters/clients/messaging.client.js";
import { WebhookRestClient } from "./adapters/clients/webhook.client.js";

// -------------------------------------------------------------------------
// Couche 3 — Adapters : résolveurs GraphQL (driving)
// -------------------------------------------------------------------------

import { buildResolvers } from "./adapters/graphql/resolvers/index.js";

// -------------------------------------------------------------------------
// Couche 2 — Use cases
// -------------------------------------------------------------------------

import { GetWorkspaceOverview } from "./application/use-cases/get-workspace-overview.js";
import { ListApiKeys } from "./application/use-cases/list-api-keys.js";
import { ManageWebhook } from "./application/use-cases/manage-webhook.js";

// ---------------------------------------------------------------------------
// Amorçage
// ---------------------------------------------------------------------------

async function main(): Promise<void> {
  // -------------------------------------------------------------------------
  // Couche 4 — Chargement de la configuration
  // -------------------------------------------------------------------------

  const config = loadConfig();

  // -------------------------------------------------------------------------
  // Couche 4 — HttpSessionValidator (appelle auth-api /validate-session)
  // -------------------------------------------------------------------------

  const sessionValidator = new HttpSessionValidator(config.authApiUrl);

  // -------------------------------------------------------------------------
  // Couche 3 — Instanciation des clients REST (implémentent les ports de sortie)
  // -------------------------------------------------------------------------

  const identityClient = new IdentityRestClient(config.authApiUrl);
  const walletClient = new WalletRestClient(config.walletApiUrl);
  const messagingClient = new MessagingRestClient(config.messagingApiUrl);
  const webhookClient = new WebhookRestClient(config.webhookApiUrl);

  // -------------------------------------------------------------------------
  // Couche 2 — Instanciation des use cases (injectés avec les clients)
  // -------------------------------------------------------------------------

  const getWorkspaceOverview = new GetWorkspaceOverview(identityClient, walletClient);
  const listApiKeys = new ListApiKeys(identityClient);
  const manageWebhook = new ManageWebhook(webhookClient);

  // -------------------------------------------------------------------------
  // Couche 3 — Construction des résolveurs GraphQL (dépendances injectées)
  // -------------------------------------------------------------------------

  const resolvers = buildResolvers({
    getWorkspaceOverview,
    listApiKeys,
    manageWebhook,
    identityClient,
    walletClient,
    messagingClient,
  });

  // -------------------------------------------------------------------------
  // Couche 4 — Démarrage du serveur GraphQL
  // -------------------------------------------------------------------------

  startServer({
    resolvers,
    sessionValidator,
    config,
  });
}

void main();
