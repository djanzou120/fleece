'use client';
/**
 * Button — inline shadcn/ui-inspired component.
 * DEBT: Replace with the real shadcn/ui Button once `npm install` is available
 * and @radix-ui/react-slot is installed.
 */
import React from 'react';

type ButtonVariant = 'primary' | 'secondary' | 'ghost' | 'danger';
type ButtonSize = 'sm' | 'md' | 'lg';

interface ButtonProps extends React.ButtonHTMLAttributes<HTMLButtonElement> {
  variant?: ButtonVariant;
  size?: ButtonSize;
  loading?: boolean;
  children: React.ReactNode;
}

const variantStyles: Record<ButtonVariant, React.CSSProperties> = {
  primary: {
    backgroundColor: 'var(--accent)',
    color: '#06210f',
    border: '1px solid transparent',
    fontWeight: 600,
  },
  secondary: {
    backgroundColor: 'var(--surface)',
    color: 'var(--text)',
    border: '1px solid var(--border)',
    fontWeight: 500,
  },
  ghost: {
    backgroundColor: 'transparent',
    color: 'var(--text-2)',
    border: '1px solid transparent',
    fontWeight: 500,
  },
  danger: {
    backgroundColor: 'var(--danger-soft)',
    color: 'var(--danger)',
    border: '1px solid var(--danger)',
    fontWeight: 500,
  },
};

const sizeStyles: Record<ButtonSize, React.CSSProperties> = {
  sm: { height: '30px', padding: '0 10px', fontSize: '12px', borderRadius: '6px' },
  md: { height: '36px', padding: '0 14px', fontSize: '13px', borderRadius: '8px' },
  lg: { height: '44px', padding: '0 20px', fontSize: '14px', borderRadius: '11px' },
};

export function Button({
  variant = 'secondary',
  size = 'md',
  loading = false,
  disabled,
  children,
  style,
  ...props
}: ButtonProps) {
  return (
    <button
      disabled={disabled || loading}
      style={{
        display: 'inline-flex',
        alignItems: 'center',
        justifyContent: 'center',
        gap: '6px',
        cursor: disabled || loading ? 'not-allowed' : 'pointer',
        opacity: disabled || loading ? 0.5 : 1,
        transition: 'opacity 0.15s, background-color 0.15s',
        whiteSpace: 'nowrap',
        userSelect: 'none',
        ...variantStyles[variant],
        ...sizeStyles[size],
        ...style,
      }}
      {...props}
    >
      {loading && (
        <span
          className="animate-spin"
          style={{
            width: '12px',
            height: '12px',
            borderRadius: '50%',
            border: '2px solid currentColor',
            borderTopColor: 'transparent',
            display: 'inline-block',
          }}
          aria-hidden="true"
        />
      )}
      {children}
    </button>
  );
}
