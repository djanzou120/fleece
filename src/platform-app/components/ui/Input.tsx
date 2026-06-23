'use client';
/**
 * Input — inline shadcn/ui-inspired component.
 * DEBT: Replace with shadcn/ui Input once online.
 */
import React from 'react';

interface InputProps extends React.InputHTMLAttributes<HTMLInputElement> {
  label?: string;
  error?: string;
  hint?: string;
}

export function Input({ label, error, hint, id, style, ...props }: InputProps) {
  const inputId = id ?? (label ? label.toLowerCase().replace(/\s+/g, '-') : undefined);

  return (
    <div style={{ display: 'flex', flexDirection: 'column', gap: '6px' }}>
      {label && (
        <label
          htmlFor={inputId}
          style={{ fontSize: '12px', fontWeight: 500, color: 'var(--text-2)' }}
        >
          {label}
        </label>
      )}
      <input
        id={inputId}
        style={{
          height: '40px',
          borderRadius: '11px',
          border: `1px solid ${error ? 'var(--danger)' : 'var(--border)'}`,
          backgroundColor: 'var(--bg-elev)',
          color: 'var(--text)',
          padding: '0 12px',
          fontSize: '13px',
          outline: 'none',
          transition: 'border-color 0.15s',
          width: '100%',
          ...style,
        }}
        aria-invalid={error ? 'true' : undefined}
        aria-describedby={error ? `${inputId}-error` : hint ? `${inputId}-hint` : undefined}
        {...props}
      />
      {error && (
        <span
          id={`${inputId}-error`}
          role="alert"
          style={{ fontSize: '11px', color: 'var(--danger)' }}
        >
          {error}
        </span>
      )}
      {hint && !error && (
        <span id={`${inputId}-hint`} style={{ fontSize: '11px', color: 'var(--text-3)' }}>
          {hint}
        </span>
      )}
    </div>
  );
}
