'use client';
/**
 * Login page — DASH-01.
 * Submits to Better Auth's sign-in endpoint, then redirects to dashboard.
 */
import React, { useState } from 'react';
import { Button } from '@/components/ui/Button';
import { Input } from '@/components/ui/Input';
import { useToast } from '@/components/ui/Toast';

export default function LoginPage() {
  const [email, setEmail] = useState('');
  const [password, setPassword] = useState('');
  const [errors, setErrors] = useState<{ email?: string; password?: string }>({});
  const [loading, setLoading] = useState(false);
  const { toast } = useToast();

  const validate = (): boolean => {
    const next: typeof errors = {};
    if (!email.trim()) next.email = "L'email est requis.";
    if (!password) next.password = 'Le mot de passe est requis.';
    setErrors(next);
    return Object.keys(next).length === 0;
  };

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    if (!validate()) return;
    setLoading(true);
    try {
      const res = await fetch('/api/auth/sign-in/email', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ email, password }),
      });
      if (!res.ok) {
        const body = await res.json().catch(() => ({}));
        throw new Error(body.message ?? `Erreur ${res.status}`);
      }
      window.location.href = '/dashboard';
    } catch (err) {
      toast(err instanceof Error ? err.message : 'Identifiants incorrects.', 'error');
    } finally {
      setLoading(false);
    }
  };

  return (
    <div
      style={{
        minHeight: '100vh',
        display: 'flex',
        alignItems: 'center',
        justifyContent: 'center',
        padding: '24px',
        background:
          'radial-gradient(ellipse 80% 60% at 50% 0%, rgba(39,207,125,0.07) 0%, transparent 70%), var(--bg)',
      }}
    >
      <div style={{ width: '100%', maxWidth: '392px' }}>
        <div
          style={{
            display: 'flex',
            alignItems: 'center',
            gap: '10px',
            marginBottom: '32px',
            justifyContent: 'center',
          }}
        >
          <div
            aria-hidden="true"
            style={{
              width: '32px',
              height: '32px',
              borderRadius: '8px',
              backgroundColor: 'var(--accent)',
              color: '#06210f',
              display: 'flex',
              alignItems: 'center',
              justifyContent: 'center',
              fontWeight: 700,
              fontSize: '16px',
            }}
          >
            F
          </div>
          <span style={{ fontSize: '18px', fontWeight: 600, color: 'var(--text)', letterSpacing: '-0.02em' }}>
            Fleece
          </span>
        </div>

        <div
          style={{
            backgroundColor: 'var(--bg-elev)',
            border: '1px solid var(--border)',
            borderRadius: '14px',
            boxShadow: '0 4px 24px rgba(0,0,0,0.4)',
            padding: '28px',
          }}
        >
          <h1
            style={{
              fontSize: '20px',
              fontWeight: 600,
              color: 'var(--text)',
              letterSpacing: '-0.02em',
              marginBottom: '6px',
            }}
          >
            Se connecter
          </h1>
          <p style={{ fontSize: '13px', color: 'var(--text-3)', marginBottom: '24px' }}>
            Accédez à votre dashboard Fleece.
          </p>

          <form
            onSubmit={handleSubmit}
            noValidate
            style={{ display: 'flex', flexDirection: 'column', gap: '14px' }}
          >
            <Input
              label="Email"
              id="login-email"
              type="email"
              placeholder="awa@example.com"
              autoComplete="email"
              value={email}
              onChange={(e) => setEmail(e.target.value)}
              error={errors.email}
              required
              aria-required="true"
            />
            <Input
              label="Mot de passe"
              id="login-password"
              type="password"
              placeholder="••••••••"
              autoComplete="current-password"
              value={password}
              onChange={(e) => setPassword(e.target.value)}
              error={errors.password}
              required
              aria-required="true"
            />
            <Button
              type="submit"
              variant="primary"
              size="lg"
              loading={loading}
              disabled={loading}
              style={{ width: '100%', marginTop: '8px' }}
            >
              Se connecter
            </Button>
          </form>

          <div
            style={{
              marginTop: '20px',
              textAlign: 'center',
              fontSize: '13px',
              color: 'var(--text-3)',
            }}
          >
            Pas encore de compte ?{' '}
            <a href="/auth/signup" style={{ color: 'var(--accent-text)', textDecoration: 'none' }}>
              S'inscrire
            </a>
          </div>
        </div>
      </div>
    </div>
  );
}
