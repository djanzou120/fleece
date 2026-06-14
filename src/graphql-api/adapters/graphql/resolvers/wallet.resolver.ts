// Couche 3 — Adapter GraphQL : résolveurs relatifs au wallet et aux messages.
// Délèguent aux clients injectés (WalletClient, MessagingClient).
// Le contexte ApiContext est lu depuis l'argument contextValue (3e paramètre).

import type { ApiContext } from "@fleece/api-common";
import type {
  WalletClient,
  MessagingClient,
} from "../../../application/ports/output/clients.js";

// ---------------------------------------------------------------------------
// Types GraphQL locaux (arguments extraits du schéma)
// ---------------------------------------------------------------------------

interface WalletBalanceArgs {
  workspaceId: string;
}

interface MessagesArgs {
  workspaceId: string;
  cursor?: string;
  limit?: number;
}

// ---------------------------------------------------------------------------
// Dépendances injectées
// ---------------------------------------------------------------------------

export interface WalletResolverDeps {
  walletClient: WalletClient;
  messagingClient: MessagingClient;
}

// ---------------------------------------------------------------------------
// Factory
// ---------------------------------------------------------------------------

/**
 * Construit les résolveurs relatifs au wallet et aux messages.
 * @param deps - Clients injectés depuis le composition root.
 */
export function buildWalletResolvers(deps: WalletResolverDeps) {
  const { walletClient, messagingClient } = deps;

  return {
    Query: {
      /** Retourne le solde du wallet du workspace. */
      walletBalance: async (
        _parent: unknown,
        args: WalletBalanceArgs,
        ctx: ApiContext,
      ) => {
        return walletClient.getBalance(ctx, args.workspaceId);
      },

      /**
       * Liste les messages paginés d'un workspace.
       * Supporte la pagination cursor-based : cursor + limit.
       */
      messages: async (
        _parent: unknown,
        args: MessagesArgs,
        ctx: ApiContext,
      ) => {
        return messagingClient.listMessages(ctx, args.workspaceId, {
          cursor: args.cursor,
          limit: args.limit,
        });
      },
    },
  };
}
