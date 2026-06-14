// Couche 3 — Adapter GraphQL : résolveurs relatifs aux workspaces et clés API.
// Délèguent aux use cases et clients injectés. Le contexte ApiContext est lu
// depuis l'argument contextValue (3e paramètre de tout résolveur GraphQL).

import type { ApiContext } from "@fleece/api-common";
import type { GetWorkspaceOverview } from "../../../application/use-cases/get-workspace-overview.js";
import type { ListApiKeys } from "../../../application/use-cases/list-api-keys.js";
import type { IdentityClient } from "../../../application/ports/output/clients.js";

// ---------------------------------------------------------------------------
// Types GraphQL locaux (argments extraits du schéma)
// ---------------------------------------------------------------------------

interface WorkspaceArgs {
  id: string;
}

interface ApiKeysArgs {
  workspaceId: string;
}

interface CreateApiKeyArgs {
  workspaceId: string;
  name: string;
}

interface RevokeApiKeyArgs {
  workspaceId: string;
  keyId: string;
}

// ---------------------------------------------------------------------------
// Dépendances injectées
// ---------------------------------------------------------------------------

export interface WorkspaceResolverDeps {
  getWorkspaceOverview: GetWorkspaceOverview;
  listApiKeys: ListApiKeys;
  identityClient: IdentityClient;
}

// ---------------------------------------------------------------------------
// Factory — retourne les résolveurs pour Query.workspace, apiKeys et Mutations
// ---------------------------------------------------------------------------

/**
 * Construit les résolveurs relatifs aux workspaces et clés API.
 * @param deps - Use cases et clients injectés depuis le composition root.
 */
export function buildWorkspaceResolvers(deps: WorkspaceResolverDeps) {
  const { getWorkspaceOverview, listApiKeys, identityClient } = deps;

  return {
    Query: {
      /**
       * Retourne l'aperçu du workspace (infos + balance).
       * Expose uniquement workspace (sans balance) pour rester conforme au schéma.
       * La balance est exposée via walletBalance (résolveur wallet).
       */
      workspace: async (
        _parent: unknown,
        args: WorkspaceArgs,
        ctx: ApiContext,
      ) => {
        const overview = await getWorkspaceOverview.execute(ctx, args.id);
        return overview?.workspace ?? null;
      },

      /** Liste les clés API d'un workspace. */
      apiKeys: async (
        _parent: unknown,
        args: ApiKeysArgs,
        ctx: ApiContext,
      ) => {
        return listApiKeys.execute(ctx, args.workspaceId);
      },
    },

    Mutation: {
      /**
       * Crée une nouvelle clé API.
       * Retourne rawKey (affiché une seule fois) + métadonnées.
       */
      createApiKey: async (
        _parent: unknown,
        args: CreateApiKeyArgs,
        ctx: ApiContext,
      ) => {
        return identityClient.createApiKey(ctx, args.workspaceId, args.name);
      },

      /**
       * Révoque une clé API (désactivation immédiate).
       * Retourne true si succès.
       */
      revokeApiKey: async (
        _parent: unknown,
        args: RevokeApiKeyArgs,
        ctx: ApiContext,
      ) => {
        await identityClient.revokeApiKey(ctx, args.workspaceId, args.keyId);
        return true;
      },
    },
  };
}
