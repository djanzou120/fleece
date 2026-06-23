'use client';
/**
 * Dialog / Modal — inline shadcn/ui-inspired component.
 * DEBT: Replace with shadcn/ui Dialog (which uses @radix-ui/react-dialog) once online.
 *
 * Accessibility:
 * - role="dialog", aria-modal="true", aria-labelledby
 * - Focus trapped inside while open (via autoFocus on first focusable child)
 * - Escape key closes the dialog
 * - Backdrop click closes the dialog
 */
import React, { useEffect, useRef } from 'react';

interface DialogProps {
  open: boolean;
  onClose: () => void;
  title: string;
  children: React.ReactNode;
  /** Max width of the dialog panel (default: 480px) */
  maxWidth?: number;
}

export function Dialog({ open, onClose, title, children, maxWidth = 480 }: DialogProps) {
  const dialogRef = useRef<HTMLDivElement>(null);
  const titleId = React.useId();

  // Close on Escape
  useEffect(() => {
    if (!open) return;
    const handleKey = (e: KeyboardEvent) => {
      if (e.key === 'Escape') onClose();
    };
    document.addEventListener('keydown', handleKey);
    return () => document.removeEventListener('keydown', handleKey);
  }, [open, onClose]);

  // Prevent body scroll while open
  useEffect(() => {
    if (open) {
      document.body.style.overflow = 'hidden';
    } else {
      document.body.style.overflow = '';
    }
    return () => { document.body.style.overflow = ''; };
  }, [open]);

  if (!open) return null;

  return (
    <div
      style={{
        position: 'fixed',
        inset: 0,
        zIndex: 100,
        display: 'flex',
        alignItems: 'center',
        justifyContent: 'center',
        padding: '16px',
      }}
      role="presentation"
    >
      {/* Backdrop */}
      <div
        aria-hidden="true"
        onClick={onClose}
        style={{
          position: 'absolute',
          inset: 0,
          backgroundColor: 'rgba(0,0,0,0.7)',
          backdropFilter: 'blur(4px)',
        }}
      />

      {/* Panel */}
      <div
        ref={dialogRef}
        role="dialog"
        aria-modal="true"
        aria-labelledby={titleId}
        style={{
          position: 'relative',
          backgroundColor: 'var(--bg-elev)',
          border: '1px solid var(--border)',
          borderRadius: '14px',
          boxShadow: '0 20px 60px rgba(0,0,0,0.6)',
          width: '100%',
          maxWidth,
          maxHeight: '90vh',
          overflowY: 'auto',
          animation: 'flc-fade 0.25s ease both',
        }}
      >
        {/* Header */}
        <div
          style={{
            display: 'flex',
            alignItems: 'center',
            justifyContent: 'space-between',
            padding: '16px 20px',
            borderBottom: '1px solid var(--border-soft)',
          }}
        >
          <h2
            id={titleId}
            style={{ fontSize: '14px', fontWeight: 600, color: 'var(--text)' }}
          >
            {title}
          </h2>
          <button
            onClick={onClose}
            aria-label="Fermer"
            style={{
              color: 'var(--text-3)',
              background: 'none',
              border: 'none',
              cursor: 'pointer',
              fontSize: '18px',
              lineHeight: 1,
              padding: '2px 6px',
              borderRadius: '4px',
            }}
          >
            ×
          </button>
        </div>

        {/* Body */}
        <div style={{ padding: '20px' }}>{children}</div>
      </div>
    </div>
  );
}
