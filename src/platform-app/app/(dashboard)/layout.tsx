/**
 * Dashboard route group layout — Server Component (async).
 *
 * Resolves the authenticated user's workspaceId server-side via the auth-api
 * /validate-session endpoint, forwarding the session cookie. Falls back to
 * DEV_WORKSPACE_ID if the auth service is unreachable (offline dev mode).
 *
 * The resolved workspaceId is injected into WorkspaceContext so that all
 * child client components can consume it via useWorkspaceId() without any
 * client-side fetch.
 *
 * See: lib/session.ts (resolver) · lib/context/WorkspaceContext.tsx (context)
 */
import React from 'react';
import { cookies } from 'next/headers';
import { Sidebar } from '@/components/Sidebar';
import { WorkspaceProvider } from '@/lib/context/WorkspaceContext';
import { resolveSession } from '@/lib/session';

export default async function DashboardLayout({ children }: { children: React.ReactNode }) {
  // Build the Cookie header string from the incoming request cookies.
  const cookieJar = cookies();
  const cookieHeader = cookieJar.getAll().map((c) => `${c.name}=${c.value}`).join('; ');

  // Resolve session server-side (falls back to DEV_WORKSPACE_ID when offline).
  const session = await resolveSession(cookieHeader);
  const workspaceId = session?.workspaceId ?? '';

  return (
    <WorkspaceProvider workspaceId={workspaceId}>
      <div
        style={{
          display: 'flex',
          minHeight: '100vh',
          backgroundColor: 'var(--bg)',
        }}
      >
        {/* Sidebar */}
        <Sidebar
          workspaceName="Acme Corp"
          userEmail="awa.diop@example.com"
        />

        {/* Main content */}
        <main
          id="main-content"
          style={{
            flex: 1,
            minWidth: 0,
            display: 'flex',
            flexDirection: 'column',
          }}
        >
          {children}
        </main>
      </div>
    </WorkspaceProvider>
  );
}
