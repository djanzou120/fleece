// Couche 3 — Adapter GraphQL : résolveurs relatifs aux endpoints webhook.
// Délèguent au use case ManageWebhook injecté.
// Le contexte ApiContext est lu depuis l'argument contextValue (3e paramètre).
//
// Sécurité : WebhookEndpoint.secretPreview masque le secret de signature.
// Le secret brut (WebhookEndpointDTO.secret) n'est JAMAIS exposé en clair.

import type { ApiContext } from "@fleece/api-common";
import type { ManageWebhook } from "../../../application/use-cases/manage-webhook.js";
import type { WebhookEndpointDTO } from "../../../application/ports/output/clients.js";

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
// Helpers de masquage du secret
// ---------------------------------------------------------------------------

/**
 * Masque le secret de signature webhook.
 *
 * Règles :
 * - Secret absent ou vide → retourne "••••••••" (masque statique)
 * - Secret présent → affiche "••••" + les 4 derniers caractères (ex. "••••a1b2")
 *
 * Le résultat est destiné à permettre à l'utilisateur de vérifier qu'il a le bon
 * secret sans jamais exposer la valeur complète via l'API GraphQL.
 */
function maskSecret(secret: string | undefined): string {
  if (!secret || secret.length === 0) {
    return "••••••••";
  }
  const visible = secret.slice(-4);
  return `••••${visible}`;
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

    /**
     * Résolveurs de champ pour le type WebhookEndpoint.
     * Masque le secret — le champ `secret` du DTO ne doit JAMAIS traverser le BFF en clair.
     */
    WebhookEndpoint: {
      /**
       * Retourne une version masquée du secret de signature.
       * Format : "••••<4 derniers caractères>" ou "••••••••" si le secret est absent.
       * Le parent est le WebhookEndpointDTO retourné par ManageWebhook.list/create.
       */
      secretPreview: (parent: WebhookEndpointDTO): string => {
        return maskSecret(parent.secret);
      },
    },
  };
}
