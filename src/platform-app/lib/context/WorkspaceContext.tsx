'use client';
/**
 * WorkspaceContext — provides the resolved workspaceId to all dashboard client components.
 *
 * The workspaceId is resolved server-side in the dashboard layout
 * (app/(dashboard)/layout.tsx) via lib/session.ts, then injected here as a prop.
 * All interactive dashboard pages consume it via the useWorkspaceId() hook.
 *
 * USAGE (any client component inside the dashboard layout):
 *
 *   import { useWorkspaceId } from '@/lib/context/WorkspaceContext';
 *
 *   function MyPage() {
 *     const workspaceId = useWorkspaceId();
 *     // workspaceId is '' if session is not resolved (shows error in UI)
 *   }
 *
 * PROVIDER (wired in app/(dashboard)/layout.tsx):
 *
 *   <WorkspaceProvider workspaceId={resolvedId}>
 *     {children}
 *   </WorkspaceProvider>
 */
import React, { createContext, useContext } from 'react';

interface WorkspaceContextValue {
  /** The active workspace ID resolved from the server session. Empty string = unauthenticated. */
  workspaceId: string;
}

const WorkspaceContext = createContext<WorkspaceContextValue>({ workspaceId: '' });

/**
 * Provider component — wraps the dashboard layout.
 * Receives the server-resolved workspaceId as a prop.
 */
export function WorkspaceProvider({
  workspaceId,
  children,
}: {
  workspaceId: string;
  children: React.ReactNode;
}) {
  return (
    <WorkspaceContext.Provider value={{ workspaceId }}>
      {children}
    </WorkspaceContext.Provider>
  );
}

/**
 * Hook — retrieve the active workspace ID.
 * Must be called from a component rendered inside WorkspaceProvider
 * (i.e. any page within the (dashboard) route group).
 *
 * Returns '' if session could not be resolved — callers should handle this
 * with an appropriate UI error state.
 */
export function useWorkspaceId(): string {
  return useContext(WorkspaceContext).workspaceId;
}
