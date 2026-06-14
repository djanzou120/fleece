// Couche 3 — Adapter GraphQL : résolveurs relatifs aux endpoints webhook.
// Délèguent au use case ManageWebhook injecté.
// Le contexte ApiContext est lu depuis l'argument contextValue (3e paramètre).

import type { ApiContext } from "@fleece/api-common";
import type { ManageWebhook } from "../../../application/use-cases/manage-webhook.js";

// ---------------------------------------------------------------------------
// Types GraphQL locaux (arguments extraits du schéma)
// ---------------------------------------------------------------------------

interface WebhookEndpointsArgs {
  workspaceId: string;
}

interface CreateWebhookEndpointArgs {
  workspaceId: string;
  url: string;
  events: string[];
}

interface DeleteWebhookEndpointArgs {
  workspaceId: string;
  id: string;
}

// ---------------------------------------------------------------------------
// Dépendances injectées
// ---------------------------------------------------------------------------

export interface WebhookResolverDeps {
  manageWebhook: ManageWebhook;
}

// ---------------------------------------------------------------------------
// Factory
// ---------------------------------------------------------------------------

/**
 * Construit les résolveurs relatifs aux webhooks.
 * @param deps - Use case injecté depuis le composition root.
 */
export function buildWebhookResolvers(deps: WebhookResolverDeps) {
  const { manageWebhook } = deps;

  return {
    Query: {
      /** Liste les endpoints webhook d'un workspace. */
      webhookEndpoints: async (
        _parent: unknown,
        args: WebhookEndpointsArgs,
        ctx: ApiContext,
      ) => {
        return manageWebhook.list(ctx, args.workspaceId);
      },
    },

    Mutation: {
      /** Enregistre un nouvel endpoint webhook. */
      createWebhookEndpoint: async (
        _parent: unknown,
        args: CreateWebhookEndpointArgs,
        ctx: ApiContext,
      ) => {
        return manageWebhook.create(ctx, args.workspaceId, {
          url: args.url,
          events: args.events,
        });
      },

      /**
       * Supprime un endpoint webhook.
       * Retourne true si la suppression s'est déroulée sans erreur.
       */
      deleteWebhookEndpoint: async (
        _parent: unknown,
        args: DeleteWebhookEndpointArgs,
        ctx: ApiContext,
      ) => {
        await manageWebhook.delete(ctx, args.workspaceId, args.id);
        return true;
      },
    },
  };
}
