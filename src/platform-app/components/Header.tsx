'use client';
/**
 * Dashboard Header — sticky, breadcrumb + optional action slot.
 * Design: .ia/design/DESIGN.md — Header spec.
 */
import React from 'react';

interface BreadcrumbItem {
  label: string;
  href?: string;
}

interface HeaderProps {
  breadcrumb: BreadcrumbItem[];
  actions?: React.ReactNode;
}

export function Header({ breadcrumb, actions }: HeaderProps) {
  return (
    <header
      style={{
        position: 'sticky',
        top: 0,
        height: '56px',
        zIndex: 50,
        backgroundColor: 'color-mix(in srgb, var(--bg) 85%, transparent)',
        backdropFilter: 'blur(10px)',
        WebkitBackdropFilter: 'blur(10px)',
        borderBottom: '1px solid var(--border-soft)',
        display: 'flex',
        alignItems: 'center',
        justifyContent: 'space-between',
        padding: '0 24px',
        gap: '16px',
      }}
    >
      {/* Breadcrumb */}
      <nav aria-label="Fil d'Ariane">
        <ol
          style={{
            listStyle: 'none',
            display: 'flex',
            alignItems: 'center',
            gap: '6px',
            fontFamily: 'var(--font-geist-mono, monospace)',
            fontSize: '13px',
          }}
        >
          {breadcrumb.map((item, index) => (
            <React.Fragment key={item.label}>
              {index > 0 && (
                <li aria-hidden="true" style={{ color: 'var(--text-3)', fontSize: '11px' }}>
                  /
                </li>
              )}
              <li>
                {item.href && index < breadcrumb.length - 1 ? (
                  <a
                    href={item.href}
                    style={{ color: 'var(--text-3)', textDecoration: 'none' }}
                  >
                    {item.label}
                  </a>
                ) : (
                  <span
                    aria-current={index === breadcrumb.length - 1 ? 'page' : undefined}
                    style={{
                      color: index === breadcrumb.length - 1 ? 'var(--text)' : 'var(--text-3)',
                    }}
                  >
                    {item.label}
                  </span>
                )}
              </li>
            </React.Fragment>
          ))}
        </ol>
      </nav>

      {/* Actions slot */}
      {actions && (
        <div style={{ display: 'flex', alignItems: 'center', gap: '8px' }}>
          {actions}
        </div>
      )}
    </header>
  );
}
