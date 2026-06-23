'use client';
/**
 * Badge / Status pill — inline shadcn/ui-inspired component.
 * DEBT: Replace with shadcn/ui Badge once online.
 */
import React from 'react';

type BadgeVariant = 'success' | 'info' | 'warn' | 'danger' | 'neutral';

interface BadgeProps {
  variant?: BadgeVariant;
  children: React.ReactNode;
  /** Show a dot indicator before the label */
  dot?: boolean;
  style?: React.CSSProperties;
}

const variantStyles: Record<BadgeVariant, React.CSSProperties> = {
  success: { color: 'var(--accent-text)',  backgroundColor: 'var(--accent-soft)' },
  info:    { color: 'var(--info)',          backgroundColor: 'var(--info-soft)' },
  warn:    { color: 'var(--warn)',          backgroundColor: 'var(--warn-soft)' },
  danger:  { color: 'var(--danger)',        backgroundColor: 'var(--danger-soft)' },
  neutral: { color: 'var(--text-3)',        backgroundColor: 'var(--surface)' },
};

export function Badge({ variant = 'neutral', dot = false, children, style }: BadgeProps) {
  return (
    <span
      style={{
        display: 'inline-flex',
        alignItems: 'center',
        gap: '5px',
        borderRadius: '20px',
        padding: '3px 10px',
        fontSize: '11px',
        fontWeight: 500,
        lineHeight: 1.4,
        ...variantStyles[variant],
        ...style,
      }}
    >
      {dot && (
        <span
          aria-hidden="true"
          style={{
            width: '6px',
            height: '6px',
            borderRadius: '50%',
            backgroundColor: 'currentColor',
            flexShrink: 0,
          }}
        />
      )}
      {children}
    </span>
  );
}

/** Convenience: map a boolean `active` flag to a badge */
export function ActiveBadge({ active }: { active: boolean }) {
  return (
    <Badge variant={active ? 'success' : 'neutral'} dot>
      {active ? 'Actif' : 'Inactif'}
    </Badge>
  );
}

/** Convenience: map a message status string to a badge */
export function StatusBadge({ status }: { status: string }) {
  const map: Record<string, BadgeVariant> = {
    delivered: 'success',
    sent: 'info',
    queued: 'warn',
    failed: 'danger',
    pending: 'warn',
  };
  const variant = map[status.toLowerCase()] ?? 'neutral';
  return (
    <Badge variant={variant} dot>
      {status}
    </Badge>
  );
}
