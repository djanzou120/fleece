// Couche 3 — Résolveurs GraphQL (driving). Délèguent aux use cases.

import { GetWorkspaceOverview } from "../../../application/use-cases/get-workspace-overview";

export function buildResolvers(overview: GetWorkspaceOverview) {
  return {
    Query: {
      workspaceOverview: (_parent: unknown, args: { id: string }) => overview.execute(args.id),
    },
  };
}
