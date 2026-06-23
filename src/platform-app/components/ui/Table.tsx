'use client';
/**
 * Table — inline shadcn/ui-inspired component.
 * DEBT: Replace with shadcn/ui Table once online.
 */
import React from 'react';

export function Table({ children }: { children: React.ReactNode }) {
  return (
    <div style={{ overflowX: 'auto', width: '100%' }}>
      <table
        style={{
          width: '100%',
          borderCollapse: 'collapse',
          fontSize: '13px',
        }}
      >
        {children}
      </table>
    </div>
  );
}

export function TableHeader({ children }: { children: React.ReactNode }) {
  return <thead>{children}</thead>;
}

export function TableBody({ children }: { children: React.ReactNode }) {
  return <tbody>{children}</tbody>;
}

export function TableRow({
  children,
  hoverable = false,
  style,
}: {
  children: React.ReactNode;
  hoverable?: boolean;
  style?: React.CSSProperties;
}) {
  const [hovered, setHovered] = React.useState(false);
  return (
    <tr
      style={{
        borderBottom: '1px solid var(--border-soft)',
        backgroundColor: hoverable && hovered ? 'var(--surface-2)' : 'transparent',
        transition: 'background-color 0.1s',
        ...style,
      }}
      onMouseEnter={() => hoverable && setHovered(true)}
      onMouseLeave={() => hoverable && setHovered(false)}
    >
      {children}
    </tr>
  );
}

export function TableHead({
  children,
  style,
}: {
  children: React.ReactNode;
  style?: React.CSSProperties;
}) {
  return (
    <th
      scope="col"
      style={{
        padding: '10px 16px',
        textAlign: 'left',
        fontSize: '11px',
        fontWeight: 500,
        color: 'var(--text-3)',
        textTransform: 'uppercase',
        letterSpacing: '0.06em',
        whiteSpace: 'nowrap',
        ...style,
      }}
    >
      {children}
    </th>
  );
}

export function TableCell({
  children,
  style,
}: {
  children: React.ReactNode;
  style?: React.CSSProperties;
}) {
  return (
    <td
      style={{
        padding: '12px 16px',
        color: 'var(--text)',
        verticalAlign: 'middle',
        ...style,
      }}
    >
      {children}
    </td>
  );
}
