/**
 * Dashboard overview page — /dashboard
 * Placeholder — full analytics implementation tracked as DASH-05 (V1).
 */
import React from 'react';
import { Header } from '@/components/Header';
import { Card, CardContent } from '@/components/ui/Card';

export default function OverviewPage() {
  return (
    <>
      <Header breadcrumb={[{ label: 'Fleece' }, { label: 'Overview' }]} />
      <div style={{ padding: '24px', display: 'flex', flexDirection: 'column', gap: '20px' }}>
        <div>
          <h1 style={{ fontSize: '22px', fontWeight: 600, color: 'var(--text)', letterSpacing: '-0.02em' }}>
            Overview
          </h1>
          <p style={{ fontSize: '13px', color: 'var(--text-3)', marginTop: '4px' }}>
            Bienvenue sur Fleece. Sélectionnez une section dans la barre de navigation.
          </p>
        </div>

        <div
          style={{
            display: 'grid',
            gridTemplateColumns: 'repeat(auto-fill, minmax(220px, 1fr))',
            gap: '16px',
          }}
        >
          {[
            { label: 'API Keys', href: '/dashboard/api-keys', desc: 'Gérer vos clés d\'accès' },
            { label: 'Wallet', href: '/dashboard/wallet', desc: 'Solde et recharges' },
            { label: 'Webhooks', href: '/dashboard/webhooks', desc: 'Endpoints de notification' },
          ].map((item) => (
            <a
              key={item.href}
              href={item.href}
              style={{ textDecoration: 'none' }}
            >
              <Card
                style={{ padding: '20px', cursor: 'pointer', transition: 'border-color 0.15s' }}
                aria-label={item.label}
              >
                <CardContent style={{ padding: 0 }}>
                  <div style={{ fontSize: '14px', fontWeight: 600, color: 'var(--text)', marginBottom: '4px' }}>
                    {item.label}
                  </div>
                  <div style={{ fontSize: '12px', color: 'var(--text-3)' }}>{item.desc}</div>
                </CardContent>
              </Card>
            </a>
          ))}
        </div>
      </div>
    </>
  );
}
