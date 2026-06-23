/**
 * Dashboard route group layout.
 * Contains Sidebar + main content area.
 * The Header is rendered per-page (each page controls its breadcrumb and actions).
 *
 * Workspace context: ideally fetched server-side via getServerSideProps or a
 * React Server Component reading the session cookie. For now we use a placeholder
 * workspace name. Full auth integration is a follow-up (DASH-01 / BFF session).
 */
import React from 'react';
import { Sidebar } from '@/components/Sidebar';

export default function DashboardLayout({ children }: { children: React.ReactNode }) {
  return (
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
  );
}
