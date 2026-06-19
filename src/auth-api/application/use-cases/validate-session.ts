// Couche 2 — Use case. Ne dépend que des ports (couche 2) et du domaine (couche 1).
// Importe ApiContext depuis @fleece/api-common (couche 0 transverse — autorisé).

import { ApiContext } from "@fleece/api-common";
import { AuthProvider, UserRepository } from "../ports/output/repositories.js";
import { UnauthorizedError } from "../../domain/errors.js";

/**
 * Cas d'usage : valider un token de session et résoudre le workspaceId.
 *
 * Flux :
 *  1. Appelle authProvider.validateSession(token) → SessionData | null
 *     - null → lève UnauthorizedError (session invalide ou expirée)
 *  2. Charge l'entité User via userRepository.findById(sessionData.userId)
 *     - null → lève UnauthorizedError (userId inconnu — session orpheline)
 *  3. Retourne ApiContext complet { workspaceId, userId, authMethod: "session" }
 *
 * Décision workspaceId :
 *   L'entité User (domain/user.ts) porte directement un champ `workspaceId`.
 *   UserRepository.findById(userId) suffit — pas besoin de WorkspaceRepository.findByOwnerId.
 *   Justification : un User appartient à exactement un Workspace (relation N:1 modélisée dans
 *   le domaine). La résolution se fait donc en O(1) sans traverser Workspace.
 */
export class ValidateSession {
  constructor(
    private readonly authProvider: AuthProvider,
    private readonly userRepository: UserRepository,
  ) {}

  async execute(token: string): Promise<ApiContext> {
    // 1. Valider le token via l'authProvider (Better Auth, confiné en couche 3/4)
    const sessionData = await this.authProvider.validateSession(token);

    if (sessionData === null) {
      throw new UnauthorizedError("Session invalide ou expirée");
    }

    // 2. Charger l'utilisateur pour résoudre le workspaceId
    const user = await this.userRepository.findById(sessionData.userId);

    if (user === null) {
      throw new UnauthorizedError(
        `Utilisateur introuvable pour la session : ${sessionData.userId}`,
      );
    }

    // 3. Construire le ApiContext complet
    const context: ApiContext = {
      workspaceId: user.workspaceId,
      userId: user.id,
      authMethod: "session",
    };

    return context;
  }
}
