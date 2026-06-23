/**
 * Root layout — applies dark theme, fonts, global CSS.
 *
 * FONT DEBT: Geist (by Vercel) is the design-specified font but requires either:
 *   a) `npm install geist` + import { GeistSans, GeistMono } from 'geist/font'
 *   b) or next/font/google (needs network at build time)
 * Both options are blocked offline. The body falls back to `system-ui` and
 * `ui-monospace` via CSS custom properties defined in globals.css.
 * Action: once `npm install` is available, install `geist` and switch to:
 *   import { GeistSans } from 'geist/font/sans';
 *   import { GeistMono } from 'geist/font/mono';
 *   <html className={`${GeistSans.variable} ${GeistMono.variable}`}>
 */
import type { Metadata } from 'next';
import './globals.css';
import { ToastProvider } from '@/components/ui/Toast';

export const metadata: Metadata = {
  title: 'Fleece — Dashboard',
  description: 'Plateforme de communication omnicanale Fleece',
  // themeColor moved to viewport export in Next 14+
};

export const viewport = {
  themeColor: '#0a0a0a',
};

export default function RootLayout({ children }: { children: React.ReactNode }) {
  return (
    <html lang="fr" data-theme="dark">
      <body>
        <ToastProvider>{children}</ToastProvider>
      </body>
    </html>
  );
}
