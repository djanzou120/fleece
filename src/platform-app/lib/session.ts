/**
 * Server-side session resolver — Fleece dashboard.
 *
 * Reads the Better Auth session cookie from the incoming request headers and
 * calls the auth-api /validate-session endpoint to resolve the workspaceId,
 * userId and authMethod.
 *
 * USAGE (Next.js Server Components or Route Handlers ONLY — not in client code):
 *
 *   // Server Component
 *   import { cookies } from 'next/headers';
 *   import { resolveSession } from '@/lib/session';
 *
 *   const cookieJar = cookies();
 *   const cookieHeader = cookieJar.getAll().map((c) => `${c.name}=${c.value}`).join('; ');
 *   const session = await resolveSession(cookieHeader);
 *
 *   // Route Handler
 *   const cookieHeader = request.headers.get('cookie') ?? '';
 *   const session = await resolveSession(cookieHeader);
 *
 * DEV / OFFLINE FALLBACK:
 *   Set DEV_WORKSPACE_ID in .env.local (server-side env — NOT NEXT_PUBLIC_*).
 *   When the auth-api is unreachable or returns a non-OK response, the helper
 *   returns { workspaceId: DEV_WORKSPACE_ID, userId: 'dev', authMethod: 'dev' }.
 *   This enables full offline development without running the auth service.
 *
 * PRODUCTION:
 *   AUTH_API_URL must point to the internal URL of auth-api
 *   (e.g. http://auth-api:3001 in Kubernetes — see deploy/k8s/graphql-api.yaml).
 *   The /validate-session endpoint accepts the session cookie and returns
 *   { workspaceId: string, userId: string, authMethod: string }.
 *
 * SECURITY:
 *   The cookie header is forwarded server-to-server (not exposed to the client).
 *   The resolved workspaceId is passed to the WorkspaceContext as a plain string.
 *   It is NOT a secret — the BFF enforces authorization on each GraphQL call.
 */

const AUTH_API_URL = process.env.AUTH_API_URL ?? 'http://localhost:3001';

export interface SessionInfo {
  workspaceId: string;
  userId: string;
  authMethod: string;
}

/**
 * Resolve session info from a raw Cookie header string.
 *
 * @param cookieHeader - The raw Cookie header value forwarded to auth-api.
 * @returns A SessionInfo object if the session is valid, null otherwise.
 */
export async function resolveSession(cookieHeader: string): Promise<SessionInfo | null> {
  const devWorkspaceId = process.env.DEV_WORKSPACE_ID;

  try {
    const res = await fetch(`${AUTH_API_URL}/validate-session`, {
      method: 'POST',
      headers: {
        'Content-Type': 'application/json',
        ...(cookieHeader ? { Cookie: cookieHeader } : {}),
      },
      // Disable Next.js fetch cache — session must be validated fresh on each request.
      cache: 'no-store',
    });

    if (!res.ok) {
      // Auth failed — use dev workspace fallback if configured
      if (devWorkspaceId) {
        return { workspaceId: devWorkspaceId, userId: 'dev', authMethod: 'dev' };
      }
      return null;
    }

    const data = (await res.json()) as Partial<SessionInfo>;
    if (!data.workspaceId) {
      if (devWorkspaceId) {
        return { workspaceId: devWorkspaceId, userId: 'dev', authMethod: 'dev' };
      }
      return null;
    }
    return {
      workspaceId: data.workspaceId,
      userId: data.userId ?? 'unknown',
      authMethod: data.authMethod ?? 'session',
    };
  } catch {
    // Auth API unreachable — dev offline fallback
    if (devWorkspaceId) {
      return { workspaceId: devWorkspaceId, userId: 'dev', authMethod: 'dev' };
    }
    return null;
  }
}

/**
 * Convenience wrapper: resolve workspaceId only.
 *
 * @returns The workspaceId string, or null if session is invalid.
 */
export async function resolveWorkspaceId(cookieHeader: string): Promise<string | null> {
  const session = await resolveSession(cookieHeader);
  return session?.workspaceId ?? null;
}
