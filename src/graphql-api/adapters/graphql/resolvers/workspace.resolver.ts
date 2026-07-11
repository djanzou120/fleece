// Couche 3 — Adapter GraphQL : résolveurs relatifs aux workspaces et clés API.
// Délèguent aux use cases et clients injectés. Le contexte ApiContext est lu
// depuis l'argument contextValue (3e paramètre de tout résolveur GraphQL).

import type { ApiContext } from "@fleece/api-common";
import type { GetWorkspaceOverview } from "../../../application/use-cases/get-workspace-overview.js";
import type { ListApiKeys } from "../../../application/use-cases/list-api-keys.js";
import type { IdentityClient } from "../../../application/ports/output/clients.js";

// ---------------------------------------------------------------------------
// Types GraphQL locaux (arguments extraits du schéma)
// ---------------------------------------------------------------------------

interface WorkspaceArgs {
  id: string;
}

interface ApiKeysArgs {
  workspaceId: string;
}

interface CreateWorkspaceArgs {
  name: string;
  country: string;
}

interface CreateApiKeyArgs {
  workspaceId: string;
  name: string;
}

interface RevokeApiKeyArgs {
  workspaceId: string;
  keyId: string;
}

interface RotateApiKeyArgs {
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
       * Crée un nouveau workspace pour l'utilisateur authentifié.
       * Appelle auth-api POST /workspaces avec {name, country}.
       * TODO(production): auth-api doit résoudre l'owner depuis le token de session.
       */
      createWorkspace: async (
        _parent: unknown,
        args: CreateWorkspaceArgs,
        ctx: ApiContext,
      ) => {
        return identityClient.createWorkspace(ctx, args.name, args.country);
      },

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

      /**
       * Effectue la rotation d'une clé API : révoque l'ancienne, crée une nouvelle.
       * Retourne rawKey (affiché une seule fois) + métadonnées de la nouvelle clé.
       * Route auth-api : POST /workspaces/{workspaceId}/api-keys/{keyId}/rotate
       */
      rotateApiKey: async (
        _parent: unknown,
        args: RotateApiKeyArgs,
        ctx: ApiContext,
      ) => {
        return identityClient.rotateApiKey(ctx, args.workspaceId, args.keyId);
      },
    },
  };
}
