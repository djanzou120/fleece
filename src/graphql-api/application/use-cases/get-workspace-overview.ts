// Couche 2 — Use case : agrège les informations de workspace et de solde wallet.
// Dépend uniquement des ports de sortie (interfaces) et de @fleece/api-common.
// Aucune dépendance vers adapters, infrastructure ou frameworks.

import type { ApiContext } from "@fleece/api-common";
import type {
  IdentityClient,
  WalletClient,
  WorkspaceDTO,
  WalletBalanceDTO,
} from "../ports/output/clients.js";

// ---------------------------------------------------------------------------
// Type de retour agrégé (type pur, local à l'application)
// ---------------------------------------------------------------------------

export interface WorkspaceOverview {
  workspace: WorkspaceDTO;
  balance: WalletBalanceDTO;
}

// ---------------------------------------------------------------------------
// Use case
// ---------------------------------------------------------------------------

/**
 * Agrège les informations de workspace (Identity) et de solde (Wallet).
 * Retourne null si le workspace n'existe pas.
 */
export class GetWorkspaceOverview {
  constructor(
    private readonly identityClient: IdentityClient,
    private readonly walletClient: WalletClient,
  ) {}

  async execute(
    ctx: ApiContext,
    workspaceId: string,
  ): Promise<WorkspaceOverview | null> {
    // Appels parallèles pour minimiser la latence
    const [workspace, balance] = await Promise.all([
      this.identityClient.getWorkspace(ctx, workspaceId),
      this.walletClient.getBalance(ctx, workspaceId),
    ]);

    // Si le workspace est absent, on ne renvoie rien
    if (workspace === null) {
      return null;
    }

    return { workspace, balance };
  }
}
