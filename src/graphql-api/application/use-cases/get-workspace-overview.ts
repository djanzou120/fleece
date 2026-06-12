// Couche 2 — Use case d'agrégation (logique des résolveurs).

import { IdentityClient, WalletClient } from "../ports/output/clients";

export class GetWorkspaceOverview {
  constructor(
    private readonly identity: IdentityClient,
    private readonly wallet: WalletClient,
  ) {}

  async execute(workspaceId: string) {
    const [workspace, balance] = await Promise.all([
      this.identity.getWorkspace(workspaceId),
      this.wallet.getBalance(workspaceId),
    ]);
    return workspace ? { id: workspace.id, name: workspace.name, balance } : null;
  }
}
