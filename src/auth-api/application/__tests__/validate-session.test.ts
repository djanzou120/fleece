// Tests unitaires — Couche 2 (Application). Use case ValidateSession.
// Utilise des mocks manuels des ports.

import { ValidateSession } from "../use-cases/validate-session.js";
import { AuthProvider, SessionData, UserRepository } from "../ports/output/repositories.js";
import { User } from "../../domain/user.js";
import { UnauthorizedError } from "../../domain/errors.js";

// ---------------------------------------------------------------------------
// Mocks manuels des ports
// ---------------------------------------------------------------------------

class InMemoryAuthProvider implements AuthProvider {
  private sessions = new Map<string, SessionData>();

  addSession(token: string, data: SessionData): void {
    this.sessions.set(token, data);
  }

  async createSession(_userId: string): Promise<SessionData> {
    throw new Error("non requis dans ce test");
  }

  async validateSession(token: string): Promise<SessionData | null> {
    return this.sessions.get(token) ?? null;
  }

  async deleteSession(_sessionId: string): Promise<void> {
    throw new Error("non requis dans ce test");
  }

  async verifySession(token: string): Promise<{ userId: string } | null> {
    const s = await this.validateSession(token);
    return s ? { userId: s.userId } : null;
  }
}

class InMemoryUserRepository implements UserRepository {
  private store = new Map<string, User>();

  addUser(user: User): void {
    this.store.set(user.id, user);
  }

  async save(user: User): Promise<void> {
    this.store.set(user.id, user);
  }

  async findById(id: string): Promise<User | null> {
    return this.store.get(id) ?? null;
  }

  async findByWorkspace(workspaceId: string): Promise<User[]> {
    return [...this.store.values()].filter((u) => u.workspaceId === workspaceId);
  }
}

// ---------------------------------------------------------------------------
// Fixtures
// ---------------------------------------------------------------------------

const WORKSPACE_ID = "ws-test-42";
const USER_ID = "user-test-1";
const SESSION_TOKEN = "valid-session-token";

const SESSION_DATA: SessionData = {
  sessionId: "sess-abc",
  userId: USER_ID,
  expiresAt: new Date(Date.now() + 3_600_000),
};

const USER: User = {
  id: USER_ID,
  email: "alice@example.com",
  name: "Alice",
  workspaceId: WORKSPACE_ID,
  role: "owner",
  createdAt: new Date("2026-01-01"),
};

// ---------------------------------------------------------------------------
// Tests
// ---------------------------------------------------------------------------

describe("ValidateSession", () => {
  let authProvider: InMemoryAuthProvider;
  let userRepo: InMemoryUserRepository;
  let useCase: ValidateSession;

  beforeEach(() => {
    authProvider = new InMemoryAuthProvider();
    userRepo = new InMemoryUserRepository();
    useCase = new ValidateSession(authProvider, userRepo);
  });

  it("retourne un ApiContext complet pour un token valide", async () => {
    authProvider.addSession(SESSION_TOKEN, SESSION_DATA);
    userRepo.addUser(USER);

    const context = await useCase.execute(SESSION_TOKEN);

    expect(context.workspaceId).toBe(WORKSPACE_ID);
    expect(context.userId).toBe(USER_ID);
    expect(context.authMethod).toBe("session");
  });

  it("lève UnauthorizedError si le token est invalide / session introuvable", async () => {
    // Pas de session enregistrée
    await expect(useCase.execute("invalid-token")).rejects.toThrow(UnauthorizedError);
  });

  it("lève UnauthorizedError si l'utilisateur est introuvable (session orpheline)", async () => {
    authProvider.addSession(SESSION_TOKEN, SESSION_DATA);
    // Pas d'utilisateur enregistré dans le repo

    await expect(useCase.execute(SESSION_TOKEN)).rejects.toThrow(UnauthorizedError);
  });

  it("le workspaceId retourné correspond bien à celui de l'utilisateur", async () => {
    const otherUser: User = { ...USER, id: "user-2", workspaceId: "ws-other" };
    const otherSession: SessionData = { ...SESSION_DATA, sessionId: "sess-2", userId: "user-2" };
    authProvider.addSession("token-2", otherSession);
    userRepo.addUser(otherUser);

    const context = await useCase.execute("token-2");

    expect(context.workspaceId).toBe("ws-other");
    expect(context.userId).toBe("user-2");
  });
});
