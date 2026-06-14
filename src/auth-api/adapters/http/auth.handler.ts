// Couche 3 — Adapter HTTP : handler REST exposant les endpoints de l'Identity Service.
// Utilise uniquement node:http (stdlib) — aucun framework Express/Hono non installé.
// Les use cases sont injectés via le constructeur (DI manuelle).

import { IncomingMessage, ServerResponse } from "node:http";
import { CreateWorkspace } from "../../application/use-cases/create-workspace.js";
import { CreateApiKey } from "../../application/use-cases/create-api-key.js";
import { RevokeApiKey } from "../../application/use-cases/revoke-api-key.js";
import { ValidateApiKey } from "../../application/use-cases/validate-api-key.js";
import { RotateApiKey } from "../../application/use-cases/rotate-api-key.js";
import {
  WorkspaceNotFoundError,
  ApiKeyNotFoundError,
  ApiKeyRevokedError,
  InvalidApiKeyError,
  UnauthorizedError,
} from "../../domain/errors.js";
import { ApiError } from "@fleece/api-common";
import { AuthProvider } from "../../application/ports/output/repositories.js";

// ---------------------------------------------------------------------------
// Utilitaires HTTP (stdlib uniquement)
// ---------------------------------------------------------------------------

/** Lit le corps d'une requête HTTP en entier (Buffer → JSON). */
async function readBody(req: IncomingMessage): Promise<unknown> {
  return new Promise((resolve, reject) => {
    const chunks: Buffer[] = [];
    req.on("data", (chunk: Buffer) => chunks.push(chunk));
    req.on("end", () => {
      try {
        const raw = Buffer.concat(chunks).toString("utf8");
        resolve(raw.length > 0 ? (JSON.parse(raw) as unknown) : {});
      } catch {
        reject(new ApiError("VALIDATION_ERROR", "Corps JSON invalide", 400));
      }
    });
    req.on("error", reject);
  });
}

/** Envoie une réponse JSON. */
function sendJson(res: ServerResponse, status: number, body: unknown): void {
  const json = JSON.stringify(body);
  res.writeHead(status, {
    "Content-Type": "application/json",
    "Content-Length": Buffer.byteLength(json),
  });
  res.end(json);
}

/** Mappe les erreurs domaine en codes HTTP cohérents. */
function errorToStatus(err: unknown): { status: number; code: string; message: string } {
  if (err instanceof WorkspaceNotFoundError) {
    return { status: 404, code: "NOT_FOUND", message: err.message };
  }
  if (err instanceof ApiKeyNotFoundError) {
    return { status: 404, code: "NOT_FOUND", message: err.message };
  }
  if (err instanceof ApiKeyRevokedError) {
    return { status: 400, code: "VALIDATION_ERROR", message: err.message };
  }
  if (err instanceof InvalidApiKeyError) {
    return { status: 401, code: "UNAUTHORIZED", message: err.message };
  }
  if (err instanceof UnauthorizedError) {
    return { status: 403, code: "FORBIDDEN", message: err.message };
  }
  if (err instanceof ApiError) {
    return { status: err.statusHint, code: err.code, message: err.message };
  }
  if (err instanceof Error) {
    return { status: 500, code: "INTERNAL_ERROR", message: err.message };
  }
  return { status: 500, code: "INTERNAL_ERROR", message: "Erreur interne" };
}

// ---------------------------------------------------------------------------
// Route matching simple (sans framework)
// ---------------------------------------------------------------------------

function matchPath(
  pathname: string,
  pattern: string,
): Record<string, string> | null {
  const patternParts = pattern.split("/");
  const pathParts = pathname.split("/");
  if (patternParts.length !== pathParts.length) return null;
  const params: Record<string, string> = {};
  for (let i = 0; i < patternParts.length; i++) {
    const pp = patternParts[i];
    const lp = pathParts[i];
    if (pp !== undefined && lp !== undefined) {
      if (pp.startsWith(":")) {
        params[pp.slice(1)] = lp;
      } else if (pp !== lp) {
        return null;
      }
    }
  }
  return params;
}

// ---------------------------------------------------------------------------
// Handler principal
// ---------------------------------------------------------------------------

export class AuthHandler {
  constructor(
    private readonly createWorkspace: CreateWorkspace,
    private readonly createApiKey: CreateApiKey,
    private readonly revokeApiKey: RevokeApiKey,
    private readonly validateApiKey: ValidateApiKey,
    private readonly rotateApiKey: RotateApiKey,
    private readonly authProvider: AuthProvider,
  ) {}

  /**
   * Point d'entrée unique : appelé pour chaque requête HTTP entrante.
   * Route vers le bon use case selon la méthode HTTP et le chemin.
   */
  async handle(req: IncomingMessage, res: ServerResponse): Promise<void> {
    const method = req.method ?? "GET";
    const rawUrl = req.url ?? "/";
    // Ignore les query strings pour le matching
    const pathname = rawUrl.split("?")[0] ?? "/";

    try {
      // POST /workspaces → CreateWorkspace
      if (method === "POST" && pathname === "/workspaces") {
        await this.handleCreateWorkspace(req, res);
        return;
      }

      // POST /workspaces/:id/api-keys → CreateApiKey
      const createKeyMatch = matchPath(pathname, "/workspaces/:id/api-keys");
      if (method === "POST" && createKeyMatch !== null) {
        await this.handleCreateApiKey(req, res, createKeyMatch.id ?? "");
        return;
      }

      // DELETE /workspaces/:id/api-keys/:keyId → RevokeApiKey
      const revokeKeyMatch = matchPath(pathname, "/workspaces/:id/api-keys/:keyId");
      if (method === "DELETE" && revokeKeyMatch !== null) {
        await this.handleRevokeApiKey(res, revokeKeyMatch.keyId ?? "");
        return;
      }

      // POST /workspaces/:id/api-keys/:keyId/rotate → RotateApiKey
      const rotateKeyMatch = matchPath(pathname, "/workspaces/:id/api-keys/:keyId/rotate");
      if (method === "POST" && rotateKeyMatch !== null) {
        await this.handleRotateApiKey(req, res, rotateKeyMatch.keyId ?? "");
        return;
      }

      // POST /validate-key → ValidateApiKey (endpoint interne pour les gateways)
      if (method === "POST" && pathname === "/validate-key") {
        await this.handleValidateKey(req, res);
        return;
      }

      // POST /validate-session → valide un token de session JWT via AuthProvider
      if (method === "POST" && pathname === "/validate-session") {
        await this.handleValidateSession(req, res);
        return;
      }

      // Route non trouvée
      sendJson(res, 404, { code: "NOT_FOUND", message: "Route non trouvée" });
    } catch (err) {
      const { status, code, message } = errorToStatus(err);
      sendJson(res, status, { code, message });
    }
  }

  // -------------------------------------------------------------------------
  // Handlers individuels
  // -------------------------------------------------------------------------

  private async handleCreateWorkspace(
    req: IncomingMessage,
    res: ServerResponse,
  ): Promise<void> {
    const body = await readBody(req) as {
      name?: string;
      country?: string;
      ownerEmail?: string;
      ownerName?: string;
      plan?: string;
    };

    if (!body.name || !body.country || !body.ownerEmail) {
      sendJson(res, 400, {
        code: "VALIDATION_ERROR",
        message: "Champs requis : name, country, ownerEmail",
      });
      return;
    }

    const result = await this.createWorkspace.execute({
      name: body.name,
      country: body.country,
      ownerEmail: body.ownerEmail,
      ownerName: body.ownerName ?? "",
      plan: body.plan,
    });

    sendJson(res, 201, {
      workspace: {
        id: result.workspace.id,
        name: result.workspace.name,
        country: result.workspace.country,
        slug: result.workspace.slug,
        plan: result.workspace.plan,
        createdAt: result.workspace.createdAt,
      },
      owner: {
        id: result.owner.id,
        email: result.owner.email,
        name: result.owner.name,
        role: result.owner.role,
      },
    });
  }

  private async handleCreateApiKey(
    req: IncomingMessage,
    res: ServerResponse,
    workspaceId: string,
  ): Promise<void> {
    const body = await readBody(req) as {
      name?: string;
      permissions?: string[];
      expiresAt?: string;
    };

    if (!body.name) {
      sendJson(res, 400, { code: "VALIDATION_ERROR", message: "Champ requis : name" });
      return;
    }

    const result = await this.createApiKey.execute({
      workspaceId,
      name: body.name,
      permissions: body.permissions,
      expiresAt: body.expiresAt ? new Date(body.expiresAt) : null,
    });

    sendJson(res, 201, {
      // La clé brute est renvoyée une seule fois. L'appelant doit la stocker.
      rawKey: result.rawKey,
      apiKey: {
        id: result.apiKey.id,
        workspaceId: result.apiKey.workspaceId,
        prefix: result.apiKey.prefix,
        name: result.apiKey.name,
        permissions: result.apiKey.permissions,
        active: result.apiKey.active,
        createdAt: result.apiKey.createdAt,
        expiresAt: result.apiKey.expiresAt,
      },
    });
  }

  private async handleRevokeApiKey(
    res: ServerResponse,
    keyId: string,
  ): Promise<void> {
    await this.revokeApiKey.execute({ apiKeyId: keyId });
    sendJson(res, 200, { revoked: true });
  }

  private async handleRotateApiKey(
    req: IncomingMessage,
    res: ServerResponse,
    keyId: string,
  ): Promise<void> {
    const body = await readBody(req) as { newName?: string };

    const result = await this.rotateApiKey.execute({
      oldApiKeyId: keyId,
      newName: body.newName,
    });

    sendJson(res, 201, {
      rawKey: result.rawKey,
      apiKey: {
        id: result.apiKey.id,
        workspaceId: result.apiKey.workspaceId,
        prefix: result.apiKey.prefix,
        name: result.apiKey.name,
        permissions: result.apiKey.permissions,
        active: result.apiKey.active,
        createdAt: result.apiKey.createdAt,
        expiresAt: result.apiKey.expiresAt,
      },
    });
  }

  private async handleValidateKey(
    req: IncomingMessage,
    res: ServerResponse,
  ): Promise<void> {
    const body = await readBody(req) as { key?: string };

    if (!body.key) {
      sendJson(res, 400, { code: "VALIDATION_ERROR", message: "Champ requis : key" });
      return;
    }

    const context = await this.validateApiKey.execute(body.key);
    sendJson(res, 200, context);
  }

  /**
   * POST /validate-session
   *
   * Extrait le Bearer token de l'en-tête Authorization (prioritaire) ou du corps JSON
   * `{ token }` (fallback — compatible avec graphql-api/infrastructure/session-validator.ts).
   * Appelle authProvider.validateSession(token).
   *
   * Réponse 200 : { workspaceId, userId, authMethod: "session" } (ApiContext)
   * Réponse 401 : token absent ou session invalide/expirée.
   */
  private async handleValidateSession(
    req: IncomingMessage,
    res: ServerResponse,
  ): Promise<void> {
    // 1. Tenter de lire le token depuis l'en-tête Authorization: Bearer <token>
    let token: string | undefined;
    const authHeader = req.headers["authorization"];
    if (typeof authHeader === "string" && authHeader.startsWith("Bearer ")) {
      token = authHeader.slice("Bearer ".length).trim();
    }

    // 2. Fallback : lire { token } depuis le corps JSON (appelé par session-validator.ts)
    if (!token) {
      const body = await readBody(req) as { token?: string };
      token = body.token;
    }

    if (!token) {
      sendJson(res, 401, { code: "UNAUTHORIZED", message: "Token de session manquant" });
      return;
    }

    const sessionData = await this.authProvider.validateSession(token);

    if (sessionData === null) {
      sendJson(res, 401, { code: "UNAUTHORIZED", message: "Session invalide ou expirée" });
      return;
    }

    // Retourne un ApiContext conforme à ValidateSessionResponse attendu par graphql-api
    sendJson(res, 200, {
      workspaceId: "", // TODO(session): récupérer le workspaceId depuis l'userId via UserRepository
      userId: sessionData.userId,
      authMethod: "session" as const,
    });
  }
}
