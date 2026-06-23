'use client';
/**
 * Card — inline shadcn/ui-inspired component.
 * DEBT: Replace with shadcn/ui Card once online.
 */
import React from 'react';

interface CardProps {
  children: React.ReactNode;
  className?: string;
  style?: React.CSSProperties;
  /** ARIA role override (default: 'region') */
  role?: string;
  'aria-label'?: string;
}

export function Card({ children, style, ...props }: CardProps) {
  return (
    <div
      style={{
        backgroundColor: 'var(--bg-elev)',
        border: '1px solid var(--border)',
        borderRadius: '14px',
        boxShadow: 'var(--shadow)',
        ...style,
      }}
      {...props}
    >
      {children}
    </div>
  );
}

export function CardHeader({
  children,
  style,
}: {
  children: React.ReactNode;
  style?: React.CSSProperties;
}) {
  return (
    <div
      style={{
        padding: '16px 20px',
        borderBottom: '1px solid var(--border-soft)',
        display: 'flex',
        alignItems: 'center',
        justifyContent: 'space-between',
        gap: '12px',
        ...style,
      }}
    >
      {children}
    </div>
  );
}

export function CardTitle({
  children,
  style,
}: {
  children: React.ReactNode;
  style?: React.CSSProperties;
}) {
  return (
    <h2
      style={{
        fontSize: '14px',
        fontWeight: 600,
        color: 'var(--text)',
        letterSpacing: '-0.02em',
        ...style,
      }}
    >
      {children}
    </h2>
  );
}

export function CardContent({
  children,
  style,
}: {
  children: React.ReactNode;
  style?: React.CSSProperties;
}) {
  return (
    <div style={{ padding: '20px', ...style }}>
      {children}
    </div>
  );
}
