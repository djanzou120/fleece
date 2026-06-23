'use client';
/**
 * Error state — shown when a GraphQL query/mutation fails.
 */
import React from 'react';
import { Button } from '@/components/ui/Button';

interface ErrorStateProps {
  title?: string;
  message: string;
  onRetry?: () => void;
  compact?: boolean;
}

export function ErrorState({
  title = 'Une erreur est survenue',
  message,
  onRetry,
  compact,
}: ErrorStateProps) {
  return (
    <div
      role="alert"
      style={{
        display: 'flex',
        flexDirection: 'column',
        alignItems: 'center',
        justifyContent: 'center',
        textAlign: 'center',
        padding: compact ? '24px' : '60px 24px',
        gap: '10px',
      }}
    >
      <div aria-hidden="true" style={{ fontSize: '28px', opacity: 0.6 }}>
        ⚠
      </div>
      <h3
        style={{
          fontSize: '14px',
          fontWeight: 600,
          color: 'var(--danger)',
        }}
      >
        {title}
      </h3>
      <p
        style={{
          fontSize: '12px',
          color: 'var(--text-3)',
          maxWidth: '360px',
          lineHeight: 1.6,
          fontFamily: 'var(--font-geist-mono, monospace)',
        }}
      >
        {message}
      </p>
      {onRetry && (
        <Button variant="secondary" size="sm" onClick={onRetry} style={{ marginTop: '8px' }}>
          Réessayer
        </Button>
      )}
    </div>
  );
}
