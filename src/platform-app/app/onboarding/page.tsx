'use client';
/**
 * DASH-01 — Onboarding page — 4-step stepper.
 *
 * Steps:
 * 1. Workspace — name + country
 * 2. API Key — createApiKey → displays rawKey ONCE (DASH-02 AC)
 * 3. Wallet top-up — placeholder (mutation not in SDL)
 * 4. Send first message — placeholder / CTA to dashboard
 *
 * Design: card centered max-width 392px, radial-gradient bg.
 * Stepper state: client component (useState).
 * Accessibility: WCAG 2.1 AA — step indicators use aria-current, form labels.
 */
import React, { useState } from 'react';
import { Button } from '@/components/ui/Button';
import { Input } from '@/components/ui/Input';
import { useToast } from '@/components/ui/Toast';
import { gqlClient } from '@/lib/graphql/client';
import { CREATE_API_KEY_MUTATION } from '@/lib/graphql/queries';
import type { CreateApiKeyMutationResponse, CreateApiKeyResult } from '@/lib/graphql/types';

/* --------------------------------------------------------------------------- */
/* Stepper indicator                                                              */
/* --------------------------------------------------------------------------- */
const STEPS = [
  { number: 1, label: 'Workspace' },
  { number: 2, label: 'API Key' },
  { number: 3, label: 'Wallet' },
  { number: 4, label: 'Premier message' },
];

function StepperIndicator({ current }: { current: number }) {
  return (
    <nav aria-label="Étapes de l'onboarding" style={{ marginBottom: '28px' }}>
      <ol
        style={{
          display: 'flex',
          alignItems: 'center',
          gap: '0',
          listStyle: 'none',
        }}
      >
        {STEPS.map((step, index) => {
          const done = step.number < current;
          const active = step.number === current;
          return (
            <React.Fragment key={step.number}>
              <li
                aria-current={active ? 'step' : undefined}
                style={{
                  display: 'flex',
                  flexDirection: 'column',
                  alignItems: 'center',
                  gap: '4px',
                  flex: 1,
                }}
              >
                <div
                  aria-hidden="true"
                  style={{
                    width: '28px',
                    height: '28px',
                    borderRadius: '50%',
                    border: `2px solid ${done || active ? 'var(--accent)' : 'var(--border)'}`,
                    backgroundColor: done ? 'var(--accent)' : active ? 'var(--accent-soft)' : 'transparent',
                    display: 'flex',
                    alignItems: 'center',
                    justifyContent: 'center',
                    fontSize: '11px',
                    fontWeight: 600,
                    color: done ? '#06210f' : active ? 'var(--accent-text)' : 'var(--text-3)',
                    transition: 'all 0.2s',
                  }}
                >
                  {done ? '✓' : step.number}
                </div>
                <span
                  style={{
                    fontSize: '10px',
                    color: active ? 'var(--text)' : done ? 'var(--accent-text)' : 'var(--text-3)',
                    fontWeight: active ? 600 : 400,
                    whiteSpace: 'nowrap',
                  }}
                >
                  {step.label}
                </span>
              </li>
              {index < STEPS.length - 1 && (
                <li
                  aria-hidden="true"
                  style={{
                    height: '2px',
                    flex: 1,
                    backgroundColor: step.number < current ? 'var(--accent)' : 'var(--border)',
                    marginBottom: '16px',
                    transition: 'background-color 0.3s',
                  }}
                />
              )}
            </React.Fragment>
          );
        })}
      </ol>
    </nav>
  );
}

/* --------------------------------------------------------------------------- */
/* Step 1 — Workspace                                                            */
/* --------------------------------------------------------------------------- */
interface WorkspaceStepData {
  name: string;
  country: string;
}

function Step1Workspace({
  onNext,
}: {
  onNext: (data: WorkspaceStepData) => void;
}) {
  const [name, setName] = useState('');
  const [country, setCountry] = useState('');
  const [errors, setErrors] = useState<Partial<WorkspaceStepData>>({});
  const { toast } = useToast();

  const handleNext = (e: React.FormEvent) => {
    e.preventDefault();
    const next: typeof errors = {};
    if (!name.trim()) next.name = 'Le nom du workspace est requis.';
    if (!country.trim()) next.country = 'Le pays est requis.';
    setErrors(next);
    if (Object.keys(next).length > 0) return;

    // In a real flow: call a createWorkspace mutation or Better Auth workspace creation.
    // Not in the current SDL — BFF TODO.
    toast('Workspace configuré !', 'success');
    onNext({ name: name.trim(), country: country.trim() });
  };

  return (
    <form onSubmit={handleNext} noValidate style={{ display: 'flex', flexDirection: 'column', gap: '14px' }}>
      <div>
        <h2 style={{ fontSize: '16px', fontWeight: 600, color: 'var(--text)', marginBottom: '4px' }}>
          Créez votre workspace
        </h2>
        <p style={{ fontSize: '12px', color: 'var(--text-3)', lineHeight: 1.6 }}>
          Un workspace est votre espace isolé pour gérer messages, API Keys et facturation.
        </p>
      </div>
      <Input
        label="Nom du workspace"
        id="ws-name"
        placeholder="Acme Corp"
        value={name}
        onChange={(e) => setName(e.target.value)}
        error={errors.name}
        autoFocus
        required
        aria-required="true"
      />
      <div style={{ display: 'flex', flexDirection: 'column', gap: '6px' }}>
        <label
          htmlFor="ws-country"
          style={{ fontSize: '12px', fontWeight: 500, color: 'var(--text-2)' }}
        >
          Pays / Région
        </label>
        <select
          id="ws-country"
          value={country}
          onChange={(e) => setCountry(e.target.value)}
          required
          aria-required="true"
          aria-invalid={errors.country ? 'true' : undefined}
          style={{
            height: '40px',
            borderRadius: '11px',
            border: `1px solid ${errors.country ? 'var(--danger)' : 'var(--border)'}`,
            backgroundColor: 'var(--bg-elev)',
            color: country ? 'var(--text)' : 'var(--text-3)',
            padding: '0 12px',
            fontSize: '13px',
            outline: 'none',
            width: '100%',
            cursor: 'pointer',
          }}
        >
          <option value="">Sélectionnez un pays</option>
          <optgroup label="Afrique francophone">
            <option value="CM">Cameroun</option>
            <option value="SN">Sénégal</option>
            <option value="CI">Côte d'Ivoire</option>
            <option value="ML">Mali</option>
            <option value="BF">Burkina Faso</option>
            <option value="TG">Togo</option>
            <option value="BJ">Bénin</option>
            <option value="GN">Guinée</option>
            <option value="MG">Madagascar</option>
          </optgroup>
          <optgroup label="Europe">
            <option value="FR">France</option>
            <option value="BE">Belgique</option>
            <option value="CH">Suisse</option>
            <option value="LU">Luxembourg</option>
          </optgroup>
          <optgroup label="Autre">
            <option value="OTHER">Autre</option>
          </optgroup>
        </select>
        {errors.country && (
          <span role="alert" style={{ fontSize: '11px', color: 'var(--danger)' }}>
            {errors.country}
          </span>
        )}
      </div>
      <Button type="submit" variant="primary" size="lg" style={{ width: '100%', marginTop: '8px' }}>
        Continuer
      </Button>
    </form>
  );
}

/* --------------------------------------------------------------------------- */
/* Step 2 — API Key                                                               */
/* --------------------------------------------------------------------------- */
function Step2ApiKey({
  workspaceId,
  onNext,
}: {
  workspaceId: string;
  onNext: (result: CreateApiKeyResult) => void;
}) {
  const [keyName, setKeyName] = useState('Production');
  const [loading, setLoading] = useState(false);
  const [created, setCreated] = useState<CreateApiKeyResult | null>(null);
  const [copied, setCopied] = useState(false);
  const [nameError, setNameError] = useState('');
  const { toast } = useToast();

  const handleCreate = async (e: React.FormEvent) => {
    e.preventDefault();
    if (!keyName.trim()) {
      setNameError('Le nom est requis.');
      return;
    }
    setNameError('');
    setLoading(true);
    try {
      const data = await gqlClient.mutate<CreateApiKeyMutationResponse>(
        CREATE_API_KEY_MUTATION,
        { workspaceId, name: keyName.trim() },
      );
      setCreated(data.createApiKey);
    } catch (err) {
      toast(err instanceof Error ? err.message : 'Erreur lors de la création.', 'error');
    } finally {
      setLoading(false);
    }
  };

  const handleCopy = async () => {
    if (!created) return;
    await navigator.clipboard.writeText(created.rawKey);
    setCopied(true);
    setTimeout(() => setCopied(false), 2000);
  };

  if (created) {
    return (
      <div style={{ display: 'flex', flexDirection: 'column', gap: '16px' }}>
        <div>
          <h2 style={{ fontSize: '16px', fontWeight: 600, color: 'var(--text)', marginBottom: '4px' }}>
            Votre clé API est prête !
          </h2>
          <p style={{ fontSize: '12px', color: 'var(--text-3)', lineHeight: 1.6 }}>
            Copiez cette clé maintenant — elle ne sera plus jamais affichée.
          </p>
        </div>
        <div
          role="alert"
          style={{
            backgroundColor: 'var(--warn-soft)',
            border: '1px solid var(--warn)',
            borderRadius: '9px',
            padding: '14px',
          }}
        >
          <div style={{ fontSize: '11px', color: 'var(--warn)', fontWeight: 600, marginBottom: '8px' }}>
            Affichez une seule fois — à stocker en lieu sûr.
          </div>
          <div style={{ display: 'flex', gap: '8px', alignItems: 'center' }}>
            <code
              style={{
                fontFamily: 'var(--font-geist-mono, monospace)',
                fontSize: '11px',
                color: 'var(--text)',
                backgroundColor: 'var(--surface)',
                padding: '6px 10px',
                borderRadius: '6px',
                border: '1px solid var(--border)',
                flex: 1,
                wordBreak: 'break-all',
                userSelect: 'all',
              }}
              aria-label="Clé API"
            >
              {created.rawKey}
            </code>
            <Button
              variant="secondary"
              size="sm"
              onClick={handleCopy}
              aria-label="Copier la clé API"
            >
              {copied ? '✓ Copié' : 'Copier'}
            </Button>
          </div>
        </div>
        <Button
          variant="primary"
          size="lg"
          style={{ width: '100%' }}
          onClick={() => onNext(created)}
        >
          J'ai copié ma clé →
        </Button>
      </div>
    );
  }

  return (
    <form onSubmit={handleCreate} noValidate style={{ display: 'flex', flexDirection: 'column', gap: '14px' }}>
      <div>
        <h2 style={{ fontSize: '16px', fontWeight: 600, color: 'var(--text)', marginBottom: '4px' }}>
          Créez votre première API Key
        </h2>
        <p style={{ fontSize: '12px', color: 'var(--text-3)', lineHeight: 1.6 }}>
          Cette clé vous permettra d'envoyer des messages via l'API Fleece.
          Elle sera affichée une seule fois.
        </p>
      </div>
      <Input
        label="Nom de la clé"
        id="onboard-key-name"
        placeholder="Production"
        value={keyName}
        onChange={(e) => setKeyName(e.target.value)}
        error={nameError}
        required
        aria-required="true"
      />
      <Button type="submit" variant="primary" size="lg" loading={loading} disabled={loading} style={{ width: '100%' }}>
        Créer la clé API
      </Button>
    </form>
  );
}

/* --------------------------------------------------------------------------- */
/* Step 3 — Wallet                                                               */
/* --------------------------------------------------------------------------- */
function Step3Wallet({ onNext }: { onNext: () => void }) {
  return (
    <div style={{ display: 'flex', flexDirection: 'column', gap: '16px' }}>
      <div>
        <h2 style={{ fontSize: '16px', fontWeight: 600, color: 'var(--text)', marginBottom: '4px' }}>
          Rechargez votre wallet
        </h2>
        <p style={{ fontSize: '12px', color: 'var(--text-3)', lineHeight: 1.6 }}>
          Fleece fonctionne en prépayé. Rechargez votre wallet pour envoyer des messages.
        </p>
      </div>

      <div
        style={{
          backgroundColor: 'var(--warn-soft)',
          border: '1px solid var(--warn)',
          borderRadius: '9px',
          padding: '12px 14px',
          fontSize: '12px',
          color: 'var(--warn)',
          lineHeight: 1.6,
        }}
        role="note"
      >
        <strong>Note :</strong> L'intégration du paiement (Mobile Money / Stripe) sera disponible
        prochainement. Vous pourrez passer cette étape pour l'instant.
      </div>

      <div style={{ display: 'flex', flexDirection: 'column', gap: '8px' }}>
        {[
          { method: '📱 Mobile Money', desc: 'MTN MoMo, Orange Money, Wave — Afrique', disabled: true },
          { method: '💳 Stripe', desc: 'Carte bancaire — Europe', disabled: true },
        ].map((opt) => (
          <button
            key={opt.method}
            disabled={opt.disabled}
            aria-disabled="true"
            style={{
              display: 'flex',
              alignItems: 'center',
              gap: '12px',
              padding: '12px 14px',
              borderRadius: '9px',
              border: '1px solid var(--border)',
              backgroundColor: 'var(--surface)',
              cursor: 'not-allowed',
              opacity: 0.6,
              textAlign: 'left',
              width: '100%',
            }}
          >
            <span style={{ fontSize: '20px' }}>{opt.method.split(' ')[0]}</span>
            <div>
              <div style={{ fontSize: '13px', fontWeight: 500, color: 'var(--text)' }}>
                {opt.method.slice(opt.method.indexOf(' ') + 1)}
              </div>
              <div style={{ fontSize: '11px', color: 'var(--text-3)' }}>{opt.desc}</div>
            </div>
          </button>
        ))}
      </div>

      <Button variant="ghost" size="lg" style={{ width: '100%' }} onClick={onNext}>
        Passer cette étape →
      </Button>
    </div>
  );
}

/* --------------------------------------------------------------------------- */
/* Step 4 — Send first message                                                   */
/* --------------------------------------------------------------------------- */
function Step4SendMessage({ apiKeyPrefix }: { apiKeyPrefix?: string }) {
  const exampleCurl = apiKeyPrefix
    ? `curl -X POST https://api.fleece.io/v1/messages \\
  -H "Authorization: Bearer ${apiKeyPrefix}..." \\
  -H "Content-Type: application/json" \\
  -d '{"to":"+221XXXXXXXXX","content":"Bonjour !"}'`
    : `curl -X POST https://api.fleece.io/v1/messages \\
  -H "Authorization: Bearer flc_..." \\
  -H "Content-Type: application/json" \\
  -d '{"to":"+221XXXXXXXXX","content":"Bonjour !"}'`;

  return (
    <div style={{ display: 'flex', flexDirection: 'column', gap: '16px' }}>
      <div>
        <h2 style={{ fontSize: '16px', fontWeight: 600, color: 'var(--text)', marginBottom: '4px' }}>
          Envoyez votre premier message !
        </h2>
        <p style={{ fontSize: '12px', color: 'var(--text-3)', lineHeight: 1.6 }}>
          Utilisez votre clé API pour envoyer un message dès maintenant.
        </p>
      </div>

      <div
        style={{
          backgroundColor: 'var(--bg-elev)',
          border: '1px solid var(--border)',
          borderRadius: '11px',
          padding: '14px',
        }}
      >
        <div style={{ fontSize: '11px', color: 'var(--text-3)', marginBottom: '8px', fontFamily: 'monospace' }}>
          cURL
        </div>
        <pre
          aria-label="Exemple d'appel API cURL"
          style={{
            fontFamily: 'var(--font-geist-mono, monospace)',
            fontSize: '11px',
            lineHeight: 1.7,
            color: 'var(--text-2)',
            overflow: 'auto',
            whiteSpace: 'pre-wrap',
            wordBreak: 'break-all',
          }}
        >
          {exampleCurl}
        </pre>
      </div>

      <a href="/dashboard" style={{ display: 'block', textDecoration: 'none' }}>
        <Button variant="primary" size="lg" style={{ width: '100%' }}>
          Aller au dashboard →
        </Button>
      </a>
    </div>
  );
}

/* --------------------------------------------------------------------------- */
/* Main onboarding page                                                          */
/* --------------------------------------------------------------------------- */
export default function OnboardingPage() {
  const [step, setStep] = useState(1);
  // Placeholder workspaceId — in production, returned by createWorkspace mutation
  const [workspaceId] = useState('WORKSPACE_ID_FROM_SESSION');
  const [apiKeyResult, setApiKeyResult] = useState<CreateApiKeyResult | null>(null);

  return (
    <div
      style={{
        minHeight: '100vh',
        backgroundColor: 'var(--bg)',
        display: 'flex',
        alignItems: 'center',
        justifyContent: 'center',
        padding: '24px',
        background:
          'radial-gradient(ellipse 80% 60% at 50% 0%, rgba(39,207,125,0.07) 0%, transparent 70%), var(--bg)',
      }}
    >
      <div style={{ width: '100%', maxWidth: '392px' }}>
        {/* Logo */}
        <div
          style={{
            display: 'flex',
            alignItems: 'center',
            gap: '10px',
            marginBottom: '28px',
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

        {/* Stepper */}
        <StepperIndicator current={step} />

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
          {step === 1 && (
            <Step1Workspace
              onNext={() => setStep(2)}
            />
          )}
          {step === 2 && (
            <Step2ApiKey
              workspaceId={workspaceId}
              onNext={(result) => {
                setApiKeyResult(result);
                setStep(3);
              }}
            />
          )}
          {step === 3 && (
            <Step3Wallet onNext={() => setStep(4)} />
          )}
          {step === 4 && (
            <Step4SendMessage apiKeyPrefix={apiKeyResult?.apiKey.prefix} />
          )}
        </div>

        {/* Step counter */}
        <div
          style={{
            textAlign: 'center',
            marginTop: '16px',
            fontSize: '12px',
            color: 'var(--text-3)',
          }}
          aria-live="polite"
          aria-label={`Étape ${step} sur ${STEPS.length}`}
        >
          Étape {step} / {STEPS.length}
        </div>
      </div>
    </div>
  );
}
