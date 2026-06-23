/**
 * Settings page — stub.
 * Full implementation is a follow-up story.
 */
import React from 'react';
import { Header } from '@/components/Header';
import { Card, CardContent } from '@/components/ui/Card';
import { EmptyState } from '@/components/states/EmptyState';

export default function SettingsPage() {
  return (
    <>
      <Header
        breadcrumb={[{ label: 'Fleece', href: '/dashboard' }, { label: 'Settings' }]}
      />
      <div style={{ padding: '24px' }}>
        <h1
          style={{
            fontSize: '22px',
            fontWeight: 600,
            color: 'var(--text)',
            letterSpacing: '-0.02em',
            marginBottom: '24px',
          }}
        >
          Settings
        </h1>
        <Card>
          <CardContent>
            <EmptyState
              title="Paramètres à venir"
              description="La configuration du workspace, des membres d'équipe et des notifications sera disponible dans une prochaine version."
              icon="⚙"
            />
          </CardContent>
        </Card>
      </div>
    </>
  );
}
