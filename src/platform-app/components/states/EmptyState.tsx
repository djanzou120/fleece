'use client';
/**
 * Empty state — shown when a list has no items.
 */
import React from 'react';
import { Button } from '@/components/ui/Button';

interface EmptyStateProps {
  title: string;
  description: string;
  action?: {
    label: string;
    onClick: () => void;
  };
  /** Optional icon/emoji rendered above the title */
  icon?: React.ReactNode;
  /** If true, renders a smaller inline variant */
  compact?: boolean;
}

export function EmptyState({ title, description, action, icon, compact }: EmptyStateProps) {
  return (
    <div
      role="status"
      aria-label={title}
      style={{
        display: 'flex',
        flexDirection: 'column',
        alignItems: 'center',
        justifyContent: 'center',
        textAlign: 'center',
        padding: compact ? '32px 24px' : '64px 24px',
        gap: '12px',
      }}
    >
      {icon && (
        <div
          aria-hidden="true"
          style={{
            fontSize: '32px',
            marginBottom: '4px',
            opacity: 0.5,
          }}
        >
          {icon}
        </div>
      )}
      <h3
        style={{
          fontSize: compact ? '13px' : '15px',
          fontWeight: 600,
          color: 'var(--text-2)',
        }}
      >
        {title}
      </h3>
      <p
        style={{
          fontSize: '13px',
          color: 'var(--text-3)',
          maxWidth: '360px',
          lineHeight: 1.6,
        }}
      >
        {description}
      </p>
      {action && (
        <Button variant="primary" onClick={action.onClick} style={{ marginTop: '8px' }}>
          {action.label}
        </Button>
      )}
    </div>
  );
}
