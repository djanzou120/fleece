/**
 * DASH-01 — Signup page.
 * Auth is handled by the Identity Service (Better Auth in src/auth-api).
 * This form POSTs to the Better Auth API endpoint, then redirects to onboarding.
 *
 * Design: card centered, max-width 392px, radial-gradient bg.
 * Accessibility: WCAG 2.1 AA — labels, aria-describedby, contrast.
 */
'use client';
import React, { useState } from 'react';
import { Button } from '@/components/ui/Button';
import { Input } from '@/components/ui/Input';
import { useToast } from '@/components/ui/Toast';

export default function SignupPage() {
  const [name, setName] = useState('');
  const [email, setEmail] = useState('');
  const [password, setPassword] = useState('');
  const [errors, setErrors] = useState<{ name?: string; email?: string; password?: string }>({});
  const [loading, setLoading] = useState(false);
  const { toast } = useToast();

  const validate = (): boolean => {
    const next: typeof errors = {};
    if (!name.trim()) next.name = 'Le nom est requis.';
    if (!email.trim()) next.email = 'L\'email est requis.';
    else if (!/\S+@\S+\.\S+/.test(email)) next.email = 'Email invalide.';
    if (!password) next.password = 'Le mot de passe est requis.';
    else if (password.length < 8) next.password = 'Minimum 8 caractères.';
    setErrors(next);
    return Object.keys(next).length === 0;
  };

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    if (!validate()) return;
    setLoading(true);
    try {
      // In production: call Better Auth's signup endpoint
      // POST /api/auth/sign-up/email or the Better Auth route
      const res = await fetch('/api/auth/sign-up/email', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ name, email, password }),
      });
      if (!res.ok) {
        const body = await res.json().catch(() => ({}));
        throw new Error(body.message ?? `Erreur ${res.status}`);
      }
      // Redirect to onboarding
      window.location.href = '/onboarding';
    } catch (err) {
      toast(err instanceof Error ? err.message : 'Erreur lors de l\'inscription.', 'error');
    } finally {
      setLoading(false);
    }
  };

  return (
    <div
      style={{
        minHeight: '100vh',
        backgroundColor: 'var(--bg)',
        display: 'flex',
        alignItems: 'center',
        justifyContent: 'center',
        padding: '24px',
        /* Radial gradient bg per design spec */
        background: 'radial-gradient(ellipse 80% 60% at 50% 0%, rgba(39,207,125,0.07) 0%, transparent 70%), var(--bg)',
      }}
    >
      <div
        style={{
          width: '100%',
          maxWidth: '392px',
        }}
      >
        {/* Logo */}
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
          <span
            style={{
              fontSize: '18px',
              fontWeight: 600,
              color: 'var(--text)',
              letterSpacing: '-0.02em',
            }}
          >
            Fleece
          </span>
        </div>

        {/* Card */}
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
            Créer un compte
          </h1>
          <p style={{ fontSize: '13px', color: 'var(--text-3)', marginBottom: '24px' }}>
            Créez votre espace de travail Fleece.
          </p>

          <form
            onSubmit={handleSubmit}
            noValidate
            style={{ display: 'flex', flexDirection: 'column', gap: '14px' }}
          >
            <Input
              label="Nom complet"
              id="signup-name"
              type="text"
              placeholder="Awa Diop"
              autoComplete="name"
              value={name}
              onChange={(e) => setName(e.target.value)}
              error={errors.name}
              required
              aria-required="true"
            />
            <Input
              label="Email"
              id="signup-email"
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
              id="signup-password"
              type="password"
              placeholder="Minimum 8 caractères"
              autoComplete="new-password"
              value={password}
              onChange={(e) => setPassword(e.target.value)}
              error={errors.password}
              hint="8 caractères minimum"
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
              Créer mon compte
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
            Déjà un compte ?{' '}
            <a
              href="/auth/login"
              style={{ color: 'var(--accent-text)', textDecoration: 'none' }}
            >
              Se connecter
            </a>
          </div>
        </div>
      </div>
    </div>
  );
}
