'use client';
/**
 * Loading state — skeleton / spinner.
 * Used on every data-fetching screen while the GraphQL query is in flight.
 */
import React from 'react';

interface LoadingStateProps {
  /** Descriptive label for screen readers */
  label?: string;
  rows?: number;
}

export function LoadingSpinner({ size = 20 }: { size?: number }) {
  return (
    <span
      className="animate-spin"
      role="status"
      aria-label="Chargement en cours"
      style={{
        display: 'inline-block',
        width: size,
        height: size,
        borderRadius: '50%',
        border: '2px solid var(--border)',
        borderTopColor: 'var(--accent)',
      }}
    />
  );
}

export function LoadingState({ label = 'Chargement en cours…', rows = 5 }: LoadingStateProps) {
  return (
    <div
      role="status"
      aria-label={label}
      style={{ padding: '24px', display: 'flex', flexDirection: 'column', gap: '12px' }}
    >
      <span className="sr-only">{label}</span>
      {Array.from({ length: rows }).map((_, i) => (
        <div
          key={i}
          style={{
            height: '16px',
            borderRadius: '6px',
            backgroundColor: 'var(--surface-2)',
            width: `${[75, 60, 85, 65, 70][i % 5]}%`,
            opacity: 1 - i * 0.1,
          }}
          aria-hidden="true"
        />
      ))}
    </div>
  );
}

export function LoadingCard() {
  return (
    <div
      role="status"
      aria-label="Chargement en cours"
      style={{
        display: 'flex',
        alignItems: 'center',
        justifyContent: 'center',
        padding: '60px 24px',
        gap: '12px',
        color: 'var(--text-3)',
        fontSize: '13px',
      }}
    >
      <LoadingSpinner />
      <span>Chargement…</span>
    </div>
  );
}
