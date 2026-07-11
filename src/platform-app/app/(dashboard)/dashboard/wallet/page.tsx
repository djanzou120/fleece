'use client';
/**
 * DASH-03 — Wallet page.
 *
 * Features:
 * - Current balance (walletBalance query), formatted with currency (centimes/100)
 * - "Recharger" button (modal placeholder — top-up mutation not yet in SDL)
 * - Transaction history (real Query.transactions — closes D-E01 / T-008.1b)
 *   Paginated list: type badge / amount (centimes÷100 via Intl) / description / date.
 *   Loading / empty-state / error-state / retry / load-more.
 *
 * Session: workspaceId resolved server-side in layout → injected via WorkspaceContext.
 * Replaces the WORKSPACE_ID_FROM_SESSION placeholder (D-E06 / T-008.4).
 *
 * Acceptance criteria (DASH-03):
 * - Balance visible, formatted with currency and cent conversion
 * - Transaction history: real data from Query.transactions
 * - Loading / error states for both balance and history
 * - Empty-state if no transactions (no static forced empty-state)
 * - WCAG 2.1 AA
 */
import React, { useState, useCallback, useEffect } from 'react';
import { Header } from '@/components/Header';
import { Card, CardHeader, CardTitle, CardContent } from '@/components/ui/Card';
import { Button } from '@/components/ui/Button';
import { Badge } from '@/components/ui/Badge';
import { Dialog } from '@/components/ui/Dialog';
import {
  Table, TableHeader, TableBody, TableRow, TableHead, TableCell,
} from '@/components/ui/Table';
import { LoadingCard, LoadingSpinner } from '@/components/states/LoadingState';
import { EmptyState } from '@/components/states/EmptyState';
import { ErrorState } from '@/components/states/ErrorState';
import { gqlClient } from '@/lib/graphql/client';
import { WALLET_BALANCE_QUERY, TRANSACTIONS_QUERY } from '@/lib/graphql/queries';
import type {
  WalletBalance,
  WalletBalanceQueryResponse,
  Transaction,
  TransactionsQueryResponse,
} from '@/lib/graphql/types';
import { useWorkspaceId } from '@/lib/context/WorkspaceContext';
import { formatBalance, formatDateTime } from '@/lib/utils';

const PAGE_SIZE = 20;

/* --------------------------------------------------------------------------- */
/* Transaction type badge                                                         */
/* --------------------------------------------------------------------------- */
function TransactionTypeBadge({ type }: { type: string }) {
  const map: Record<string, { label: string; variant: 'success' | 'warn' | 'info' | 'neutral' }> = {
    credit:  { label: 'Crédit',   variant: 'success' },
    debit:   { label: 'Débit',    variant: 'warn'    },
    refund:  { label: 'Remb.',    variant: 'info'    },
  };
  const config = map[type.toLowerCase()] ?? { label: type, variant: 'neutral' as const };
  return <Badge variant={config.variant}>{config.label}</Badge>;
}

/* --------------------------------------------------------------------------- */
/* Transaction amount — sign derived from type (credit/refund +, debit -)       */
/* --------------------------------------------------------------------------- */
function formatTxAmount(amount: number, currency: string, type: string): string {
  const isCredit = type === 'credit' || type === 'refund';
  const formatted = formatBalance(Math.abs(amount), currency);
  return isCredit ? `+${formatted}` : `-${formatted}`;
}

/* --------------------------------------------------------------------------- */
/* Top-up modal — placeholder (mutation not yet in SDL)                          */
/* --------------------------------------------------------------------------- */
function TopUpModal({ open, onClose, currency }: { open: boolean; onClose: () => void; currency: string }) {
  const isAfrica = ['XOF', 'XAF', 'GHS', 'NGN', 'KES', 'MAD'].includes(currency);

  return (
    <Dialog open={open} onClose={onClose} title="Recharger le wallet">
      <div style={{ display: 'flex', flexDirection: 'column', gap: '16px' }}>
        {/* Placeholder notice */}
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
          Cette interface est un aperçu UX. Connectez le fournisseur de paiement
          (Mobile Money / Stripe) côté backend pour activer cette fonctionnalité.
        </div>

        <p style={{ fontSize: '13px', color: 'var(--text-2)' }}>
          Choisissez votre méthode de paiement :
        </p>

        <div style={{ display: 'flex', flexDirection: 'column', gap: '8px' }}>
          {[
            {
              icon: '📱',
              label: 'Mobile Money',
              desc: 'MTN MoMo, Orange Money, Wave — Afrique',
              preferred: isAfrica,
            },
            {
              icon: '💳',
              label: 'Stripe',
              desc: 'Carte bancaire — Europe',
              preferred: !isAfrica,
            },
          ].map((opt) => (
            <button
              key={opt.label}
              disabled
              aria-disabled="true"
              style={{
                display: 'flex',
                alignItems: 'center',
                gap: '12px',
                padding: '14px',
                borderRadius: '9px',
                border: `1px solid ${opt.preferred ? 'var(--accent)' : 'var(--border)'}`,
                backgroundColor: opt.preferred ? 'var(--accent-soft)' : 'var(--surface)',
                cursor: 'not-allowed',
                opacity: 0.7,
                textAlign: 'left',
                width: '100%',
              }}
            >
              <span
                aria-hidden="true"
                style={{
                  width: '36px',
                  height: '36px',
                  borderRadius: '8px',
                  backgroundColor: opt.preferred ? 'var(--accent-soft)' : 'var(--surface)',
                  display: 'flex',
                  alignItems: 'center',
                  justifyContent: 'center',
                  fontSize: '18px',
                  border: '1px solid var(--border)',
                }}
              >
                {opt.icon}
              </span>
              <div>
                <div style={{ fontSize: '13px', fontWeight: 600, color: 'var(--text)' }}>
                  {opt.label}
                </div>
                <div style={{ fontSize: '11px', color: 'var(--text-3)' }}>{opt.desc}</div>
              </div>
              {opt.preferred && (
                <span
                  style={{
                    marginLeft: 'auto',
                    fontSize: '10px',
                    color: 'var(--accent-text)',
                    backgroundColor: 'var(--accent-soft)',
                    padding: '2px 8px',
                    borderRadius: '20px',
                    flexShrink: 0,
                  }}
                >
                  Recommandé
                </span>
              )}
            </button>
          ))}
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
                Devise : {balance.currency}
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
  // workspaceId resolved server-side by dashboard layout — no placeholder needed.
  const workspaceId = useWorkspaceId();

  // Balance state
  const [balance, setBalance] = useState<WalletBalance | null>(null);
  const [loadingBalance, setLoadingBalance] = useState(true);
  const [balanceError, setBalanceError] = useState<string | null>(null);

  // Transaction history state
  const [transactions, setTransactions] = useState<Transaction[]>([]);
  const [nextCursor, setNextCursor] = useState<string | null>(null);
  const [hasNextPage, setHasNextPage] = useState(false);
  const [loadingTx, setLoadingTx] = useState(true);
  const [loadingMoreTx, setLoadingMoreTx] = useState(false);
  const [txError, setTxError] = useState<string | null>(null);

  const [topUpOpen, setTopUpOpen] = useState(false);

  const fetchBalance = useCallback(async () => {
    if (!workspaceId) {
      setBalanceError('Session non résolue. Veuillez vous reconnecter.');
      setLoadingBalance(false);
      return;
    }
    setLoadingBalance(true);
    setBalanceError(null);
    try {
      const data = await gqlClient.query<WalletBalanceQueryResponse>(WALLET_BALANCE_QUERY, {
        workspaceId,
      });
      setBalance(data.walletBalance);
    } catch (err) {
      setBalanceError(err instanceof Error ? err.message : 'Impossible de charger le solde.');
    } finally {
      setLoadingBalance(false);
    }
  }, [workspaceId]);

  const fetchTransactions = useCallback(async (cursor?: string) => {
    if (!workspaceId) {
      setTxError('Session non résolue. Veuillez vous reconnecter.');
      setLoadingTx(false);
      return;
    }
    if (!cursor) setLoadingTx(true);
    else setLoadingMoreTx(true);
    setTxError(null);
    try {
      const data = await gqlClient.query<TransactionsQueryResponse>(TRANSACTIONS_QUERY, {
        workspaceId,
        cursor: cursor ?? null,
        limit: PAGE_SIZE,
      });
      if (cursor) {
        setTransactions((prev) => [...prev, ...data.transactions.items]);
      } else {
        setTransactions(data.transactions.items);
      }
      setNextCursor(data.transactions.pageInfo.nextCursor);
      setHasNextPage(data.transactions.pageInfo.hasNextPage);
    } catch (err) {
      setTxError(err instanceof Error ? err.message : 'Impossible de charger les transactions.');
    } finally {
      setLoadingTx(false);
      setLoadingMoreTx(false);
    }
  }, [workspaceId]);

  useEffect(() => { fetchBalance(); }, [fetchBalance]);
  useEffect(() => { fetchTransactions(); }, [fetchTransactions]);

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

        {/* Transaction history — real data from Query.transactions (closes D-E01) */}
        <Card>
          <CardHeader>
            <CardTitle>Historique des transactions</CardTitle>
          </CardHeader>
          <CardContent style={{ padding: 0 }}>
            {loadingTx && <LoadingCard />}
            {!loadingTx && txError && (
              <ErrorState message={txError} onRetry={() => fetchTransactions()} />
            )}
            {!loadingTx && !txError && transactions.length === 0 && (
              <EmptyState
                title="Aucune transaction"
                description="Les crédits, débits et remboursements de votre workspace apparaîtront ici."
                icon="receipt"
                compact
              />
            )}
            {!loadingTx && !txError && transactions.length > 0 && (
              <>
                <Table>
                  <TableHeader>
                    <TableRow>
                      <TableHead>Type</TableHead>
                      <TableHead>Montant</TableHead>
                      <TableHead>Description</TableHead>
                      <TableHead>Date</TableHead>
                    </TableRow>
                  </TableHeader>
                  <TableBody>
                    {transactions.map((tx) => (
                      <TableRow key={tx.id} hoverable>
                        <TableCell>
                          <TransactionTypeBadge type={tx.type} />
                        </TableCell>
                        <TableCell
                          style={{
                            fontVariantNumeric: 'tabular-nums',
                            fontWeight: 500,
                            color:
                              tx.type === 'debit'
                                ? 'var(--danger)'
                                : 'var(--accent-text)',
                            fontFamily: 'var(--font-geist-mono, monospace)',
                            fontSize: '13px',
                          }}
                        >
                          <span aria-label={`Montant : ${formatTxAmount(tx.amount, tx.currency, tx.type)}`}>
                            {formatTxAmount(tx.amount, tx.currency, tx.type)}
                          </span>
                        </TableCell>
                        <TableCell
                          style={{
                            color: 'var(--text-2)',
                            fontSize: '12px',
                            maxWidth: '280px',
                          }}
                        >
                          {tx.description}
                        </TableCell>
                        <TableCell style={{ color: 'var(--text-3)', fontSize: '12px' }}>
                          {formatDateTime(tx.createdAt)}
                        </TableCell>
                      </TableRow>
                    ))}
                  </TableBody>
                </Table>
                {hasNextPage && (
                  <div style={{ padding: '16px', textAlign: 'center' }}>
                    <Button
                      variant="secondary"
                      size="sm"
                      onClick={() => fetchTransactions(nextCursor ?? undefined)}
                      loading={loadingMoreTx}
                      disabled={loadingMoreTx}
                    >
                      Charger plus
                    </Button>
                  </div>
                )}
              </>
            )}
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
