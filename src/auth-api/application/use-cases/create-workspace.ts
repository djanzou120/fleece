// Couche 2 — Use case. Ne dépend que des ports (couche 2) et du domaine (couche 1).
// Aucun import framework.

import { randomUUID } from "node:crypto";
import { Workspace } from "../../domain/workspace.js";
import { User } from "../../domain/user.js";
import { WorkspaceRepository, UserRepository } from "../ports/output/repositories.js";

export interface CreateWorkspaceInput {
  name: string;
  country: string;
  ownerEmail: string;
  ownerName: string;
  plan?: string;
}

export interface CreateWorkspaceOutput {
  workspace: Workspace;
  owner: User;
}

/**
 * Cas d'usage : créer un nouveau workspace et son utilisateur propriétaire.
 * Génère les UUIDs, persiste via les repositories (ports).
 */
export class CreateWorkspace {
  constructor(
    private readonly workspaceRepo: WorkspaceRepository,
    private readonly userRepo: UserRepository,
  ) {}

  async execute(input: CreateWorkspaceInput): Promise<CreateWorkspaceOutput> {
    const workspaceId = randomUUID();
    const userId = randomUUID();
    const now = new Date();

    // Slug dérivé du nom : lower-case, espaces → tirets, caractères non-alphanum supprimés.
    const slug = input.name
      .toLowerCase()
      .replace(/\s+/g, "-")
      .replace(/[^a-z0-9-]/g, "");

    const workspace = new Workspace(
      workspaceId,
      input.name,
      input.country,
      userId, // ownerId
      slug,
      input.plan ?? "free",
      now,
    );

    const owner: User = {
      id: userId,
      email: input.ownerEmail,
      name: input.ownerName,
      workspaceId,
      role: "owner",
      createdAt: now,
    };

    await this.workspaceRepo.save(workspace);
    await this.userRepo.save(owner);

    return { workspace, owner };
  }
}
