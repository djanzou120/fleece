'use client';
/**
 * Sidebar — 230px fixed left nav.
 * Responsive: on mobile (<768px) it is hidden behind a toggle.
 * Design: .ia/design/DESIGN.md — Sidebar spec.
 */
import React from 'react';
import Link from 'next/link';
import { usePathname } from 'next/navigation';

interface NavItem {
  href: string;
  label: string;
  icon: string;
}

const navItems: NavItem[] = [
  { href: '/dashboard',       label: 'Overview',   icon: '⊞' },
  { href: '/dashboard/messages',   label: 'Messages',   icon: '✉' },
  { href: '/dashboard/api-keys',   label: 'API Keys',   icon: '⚷' },
  { href: '/dashboard/wallet',     label: 'Wallet',     icon: '◈' },
  { href: '/dashboard/webhooks',   label: 'Webhooks',   icon: '↗' },
  { href: '/dashboard/settings',   label: 'Settings',   icon: '⚙' },
];

interface SidebarProps {
  workspaceName?: string;
  userEmail?: string;
}

export function Sidebar({ workspaceName = 'Workspace', userEmail = '' }: SidebarProps) {
  const pathname = usePathname();

  const isActive = (href: string) =>
    href === '/dashboard' ? pathname === href : pathname.startsWith(href);

  return (
    <nav
      aria-label="Navigation principale"
      style={{
        width: '230px',
        minWidth: '230px',
        height: '100vh',
        position: 'sticky',
        top: 0,
        backgroundColor: 'var(--bg-elev)',
        borderRight: '1px solid var(--border)',
        display: 'flex',
        flexDirection: 'column',
        overflowY: 'auto',
      }}
    >
      {/* Logo + workspace */}
      <div
        style={{
          padding: '16px 16px 12px',
          borderBottom: '1px solid var(--border-soft)',
          display: 'flex',
          alignItems: 'center',
          gap: '10px',
        }}
      >
        {/* Logo pill */}
        <div
          aria-hidden="true"
          style={{
            width: '28px',
            height: '28px',
            borderRadius: '8px',
            backgroundColor: 'var(--accent)',
            color: '#06210f',
            display: 'flex',
            alignItems: 'center',
            justifyContent: 'center',
            fontWeight: 700,
            fontSize: '13px',
            flexShrink: 0,
          }}
        >
          F
        </div>
        <div style={{ minWidth: 0 }}>
          <div
            style={{
              fontSize: '13px',
              fontWeight: 600,
              color: 'var(--text)',
              overflow: 'hidden',
              textOverflow: 'ellipsis',
              whiteSpace: 'nowrap',
            }}
          >
            {workspaceName}
          </div>
          <div style={{ fontSize: '11px', color: 'var(--text-3)' }}>Fleece</div>
        </div>
      </div>

      {/* Nav items */}
      <ul
        role="list"
        style={{
          listStyle: 'none',
          padding: '8px 8px 0',
          flex: 1,
          display: 'flex',
          flexDirection: 'column',
          gap: '2px',
        }}
      >
        {navItems.map((item) => {
          const active = isActive(item.href);
          return (
            <li key={item.href}>
              <Link
                href={item.href}
                aria-current={active ? 'page' : undefined}
                style={{
                  display: 'flex',
                  alignItems: 'center',
                  gap: '10px',
                  padding: '7px 10px',
                  borderRadius: '8px',
                  fontSize: '13px',
                  fontWeight: active ? 500 : 400,
                  color: active ? 'var(--accent-text)' : 'var(--text-3)',
                  backgroundColor: active ? 'var(--accent-soft)' : 'transparent',
                  textDecoration: 'none',
                  transition: 'background-color 0.1s, color 0.1s',
                }}
              >
                <span aria-hidden="true" style={{ fontSize: '14px', width: '18px', textAlign: 'center' }}>
                  {item.icon}
                </span>
                {item.label}
              </Link>
            </li>
          );
        })}
      </ul>

      {/* User footer */}
      {userEmail && (
        <div
          style={{
            padding: '12px 16px',
            borderTop: '1px solid var(--border-soft)',
            display: 'flex',
            alignItems: 'center',
            gap: '10px',
          }}
        >
          {/* Avatar */}
          <div
            aria-hidden="true"
            style={{
              width: '28px',
              height: '28px',
              borderRadius: '50%',
              background: 'linear-gradient(135deg, #27cf7d, #1b7e9c)',
              display: 'flex',
              alignItems: 'center',
              justifyContent: 'center',
              fontSize: '11px',
              fontWeight: 600,
              color: '#fff',
              flexShrink: 0,
            }}
          >
            {userEmail[0]?.toUpperCase() ?? 'U'}
          </div>
          <div
            style={{
              fontSize: '12px',
              color: 'var(--text-2)',
              overflow: 'hidden',
              textOverflow: 'ellipsis',
              whiteSpace: 'nowrap',
            }}
          >
            {userEmail}
          </div>
        </div>
      )}
    </nav>
  );
}
