'use client';
/**
 * DASH-03 — Wallet page.
 *
 * Features:
 * - Current balance (walletBalance query), formatted with currency
 * - "Recharger" button (modal placeholder — top-up mutation does not exist in SDL)
 * - Transaction history (empty-state + debt note)
 *
 * DEBT — Query.transactions: The SDL defines TransactionPage and Transaction types
 * but does NOT expose a `transactions(workspaceId: ID!, ...)` Query field.
 * The history section shows an empty state with a clear debt notice.
 * BFF TODO: expose `transactions(workspaceId: ID!, cursor: String, limit: Int): TransactionPage!`
 *
 * DEBT — Top-up mutation: No `topUp` or `createPayment` mutation exists in the SDL.
 * The "Recharger" modal is a placeholder with Mobile Money / Stripe options.
 * BFF TODO: expose a top-up flow (Mobile Money + Stripe adapters).
 *
 * Acceptance criteria (DASH-03):
 * - Balance visible, formatted with currency and cent conversion (÷100)
 * - Loading / error states for balance
 * - History empty-state with debt annotation
 * - WCAG 2.1 AA
 */
import React, { useState, useCallback, useEffect } from 'react';
import { Header } from '@/components/Header';
import { Card, CardHeader, CardTitle, CardContent } from '@/components/ui/Card';
import { Button } from '@/components/ui/Button';
import { Dialog } from '@/components/ui/Dialog';
import { LoadingCard, LoadingSpinner } from '@/components/states/LoadingState';
import { EmptyState } from '@/components/states/EmptyState';
import { ErrorState } from '@/components/states/ErrorState';
import { gqlClient } from '@/lib/graphql/client';
import { WALLET_BALANCE_QUERY } from '@/lib/graphql/queries';
import type { WalletBalance, WalletBalanceQueryResponse } from '@/lib/graphql/types';
import { formatBalance } from '@/lib/utils';

const WORKSPACE_ID = 'WORKSPACE_ID_FROM_SESSION';

/* --------------------------------------------------------------------------- */
/* Top-up modal — placeholder (mutation not yet in SDL)                          */
/* --------------------------------------------------------------------------- */
function TopUpModal({ open, onClose, currency }: { open: boolean; onClose: () => void; currency: string }) {
  const isAfrica = ['XOF', 'XAF', 'GHS', 'NGN', 'KES', 'MAD'].includes(currency);

  return (
    <Dialog open={open} onClose={onClose} title="Recharger le wallet">
      <div style={{ display: 'flex', flexDirection: 'column', gap: '16px' }}>
        {/* Debt notice */}
        <div
          role="status"
          style={{
            backgroundColor: 'var(--warn-soft)',
            border: '1px solid var(--warn)',
            borderRadius: '9px',
            padding: '12px 14px',
            fontSize: '12px',
            color: 'var(--warn)',
            lineHeight: 1.6,
          }}
        >
          <strong>Note :</strong> La mutation de rechargement n'est pas encore exposée par le BFF.
          Cette interface est un aperçu UX. Connectez le fournisseur de paiement (Mobile Money / Stripe)
          côté backend pour activer cette fonctionnalité.
        </div>

        <p style={{ fontSize: '13px', color: 'var(--text-2)' }}>
          Choisissez votre méthode de paiement :
        </p>

        <div style={{ display: 'flex', flexDirection: 'column', gap: '8px' }}>
          {/* Mobile Money option */}
          <button
            disabled
            aria-disabled="true"
            style={{
              display: 'flex',
              alignItems: 'center',
              gap: '12px',
              padding: '14px',
              borderRadius: '9px',
              border: `1px solid ${isAfrica ? 'var(--accent)' : 'var(--border)'}`,
              backgroundColor: isAfrica ? 'var(--accent-soft)' : 'var(--surface)',
              cursor: 'not-allowed',
              opacity: 0.7,
              textAlign: 'left',
            }}
          >
            <span
              aria-hidden="true"
              style={{
                width: '36px',
                height: '36px',
                borderRadius: '8px',
                backgroundColor: 'var(--warn-soft)',
                display: 'flex',
                alignItems: 'center',
                justifyContent: 'center',
                fontSize: '18px',
              }}
            >
              📱
            </span>
            <div>
              <div style={{ fontSize: '13px', fontWeight: 600, color: 'var(--text)' }}>
                Mobile Money
              </div>
              <div style={{ fontSize: '11px', color: 'var(--text-3)' }}>
                MTN MoMo, Orange Money, Wave — Afrique
              </div>
            </div>
            {isAfrica && (
              <span
                style={{
                  marginLeft: 'auto',
                  fontSize: '10px',
                  color: 'var(--accent-text)',
                  backgroundColor: 'var(--accent-soft)',
                  padding: '2px 8px',
                  borderRadius: '20px',
                }}
              >
                Recommandé
              </span>
            )}
          </button>

          {/* Stripe option */}
          <button
            disabled
            aria-disabled="true"
            style={{
              display: 'flex',
              alignItems: 'center',
              gap: '12px',
              padding: '14px',
              borderRadius: '9px',
              border: `1px solid ${!isAfrica ? 'var(--accent)' : 'var(--border)'}`,
              backgroundColor: !isAfrica ? 'var(--accent-soft)' : 'var(--surface)',
              cursor: 'not-allowed',
              opacity: 0.7,
              textAlign: 'left',
            }}
          >
            <span
              aria-hidden="true"
              style={{
                width: '36px',
                height: '36px',
                borderRadius: '8px',
                backgroundColor: 'var(--info-soft)',
                display: 'flex',
                alignItems: 'center',
                justifyContent: 'center',
                fontSize: '18px',
              }}
            >
              💳
            </span>
            <div>
              <div style={{ fontSize: '13px', fontWeight: 600, color: 'var(--text)' }}>
                Stripe
              </div>
              <div style={{ fontSize: '11px', color: 'var(--text-3)' }}>
                Carte bancaire — Europe
              </div>
            </div>
            {!isAfrica && (
              <span
                style={{
                  marginLeft: 'auto',
                  fontSize: '10px',
                  color: 'var(--accent-text)',
                  backgroundColor: 'var(--accent-soft)',
                  padding: '2px 8px',
                  borderRadius: '20px',
                }}
              >
                Recommandé
              </span>
            )}
          </button>
        </div>

        <div style={{ display: 'flex', justifyContent: 'flex-end' }}>
          <Button variant="ghost" onClick={onClose}>
            Fermer
          </Button>
        </div>
      </div>
    </Dialog>
  );
}

/* --------------------------------------------------------------------------- */
/* Balance card                                                                   */
/* --------------------------------------------------------------------------- */
function BalanceCard({
  balance,
  loading,
  error,
  onRetry,
  onTopUp,
}: {
  balance: WalletBalance | null;
  loading: boolean;
  error: string | null;
  onRetry: () => void;
  onTopUp: () => void;
}) {
  return (
    <Card aria-label="Solde du wallet">
      <CardContent style={{ padding: '24px' }}>
        {loading && (
          <div style={{ display: 'flex', alignItems: 'center', gap: '10px', minHeight: '80px' }}>
            <LoadingSpinner />
            <span style={{ fontSize: '13px', color: 'var(--text-3)' }}>Chargement du solde…</span>
          </div>
        )}
        {!loading && error && <ErrorState message={error} onRetry={onRetry} compact />}
        {!loading && !error && balance && (
          <div
            style={{
              display: 'flex',
              alignItems: 'center',
              justifyContent: 'space-between',
              flexWrap: 'wrap',
              gap: '16px',
            }}
          >
            <div>
              <div style={{ fontSize: '12px', color: 'var(--text-3)', marginBottom: '6px' }}>
                Solde disponible
              </div>
              <div
                style={{
                  fontSize: '36px',
                  fontWeight: 600,
                  color: balance.balance <= 0 ? 'var(--danger)' : 'var(--text)',
                  fontVariantNumeric: 'tabular-nums',
                  letterSpacing: '-0.02em',
                }}
                aria-label={`Solde : ${formatBalance(balance.balance, balance.currency)}`}
              >
                {formatBalance(balance.balance, balance.currency)}
              </div>
              <div style={{ fontSize: '11px', color: 'var(--text-3)', marginTop: '4px' }}>
                Devise : {balance.currency} · Montant en centimes stocké : {balance.balance.toLocaleString()}
              </div>
            </div>
            <Button variant="primary" onClick={onTopUp} aria-label="Recharger le wallet">
              + Recharger
            </Button>
          </div>
        )}
      </CardContent>
    </Card>
  );
}

/* --------------------------------------------------------------------------- */
/* Main page                                                                     */
/* --------------------------------------------------------------------------- */
export default function WalletPage() {
  const [balance, setBalance] = useState<WalletBalance | null>(null);
  const [loadingBalance, setLoadingBalance] = useState(true);
  const [balanceError, setBalanceError] = useState<string | null>(null);
  const [topUpOpen, setTopUpOpen] = useState(false);

  const fetchBalance = useCallback(async () => {
    setLoadingBalance(true);
    setBalanceError(null);
    try {
      const data = await gqlClient.query<WalletBalanceQueryResponse>(WALLET_BALANCE_QUERY, {
        workspaceId: WORKSPACE_ID,
      });
      setBalance(data.walletBalance);
    } catch (err) {
      setBalanceError(err instanceof Error ? err.message : 'Impossible de charger le solde.');
    } finally {
      setLoadingBalance(false);
    }
  }, []);

  useEffect(() => { fetchBalance(); }, [fetchBalance]);

  return (
    <>
      <Header
        breadcrumb={[{ label: 'Fleece', href: '/dashboard' }, { label: 'Wallet' }]}
        actions={
          <Button
            variant="primary"
            size="sm"
            onClick={() => setTopUpOpen(true)}
            aria-label="Recharger le wallet"
          >
            + Recharger
          </Button>
        }
      />

      <div
        className="animate-fade-in"
        style={{ padding: '24px', display: 'flex', flexDirection: 'column', gap: '24px' }}
      >
        <div>
          <h1
            style={{
              fontSize: '22px',
              fontWeight: 600,
              color: 'var(--text)',
              letterSpacing: '-0.02em',
            }}
          >
            Wallet
          </h1>
          <p style={{ fontSize: '13px', color: 'var(--text-3)', marginTop: '4px' }}>
            Solde prépayé et historique des transactions de votre workspace.
          </p>
        </div>

        {/* Balance */}
        <BalanceCard
          balance={balance}
          loading={loadingBalance}
          error={balanceError}
          onRetry={fetchBalance}
          onTopUp={() => setTopUpOpen(true)}
        />

        {/* Transaction history */}
        <Card>
          <CardHeader>
            <CardTitle>Historique des transactions</CardTitle>
          </CardHeader>
          <CardContent style={{ padding: 0 }}>
            <EmptyState
              title="Historique non disponible"
              description="L'historique des transactions sera disponible prochainement. La query Query.transactions doit être exposée par le BFF GraphQL."
              icon="📋"
              compact
            />
            {/* DEBT annotation for developers */}
            <div
              style={{
                margin: '0 20px 20px',
                padding: '10px 12px',
                backgroundColor: 'var(--surface)',
                border: '1px solid var(--border)',
                borderRadius: '6px',
                fontSize: '11px',
                color: 'var(--text-3)',
                fontFamily: 'var(--font-geist-mono, monospace)',
              }}
              aria-hidden="true"
            >
              DEBT BFF: expose{' '}
              <code style={{ color: 'var(--warn)' }}>
                Query.transactions(workspaceId: ID!, cursor: String, limit: Int): TransactionPage!
              </code>
            </div>
          </CardContent>
        </Card>
      </div>

      <TopUpModal
        open={topUpOpen}
        onClose={() => setTopUpOpen(false)}
        currency={balance?.currency ?? 'XOF'}
      />
    </>
  );
}
