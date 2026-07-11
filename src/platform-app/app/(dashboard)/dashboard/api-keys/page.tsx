'use client';
/**
 * DASH-02 — API Keys management page.
 *
 * Features:
 * - List all API keys (table: name, prefix, status, createdAt, expiresAt)
 * - Create key (modal → name input → displays rawKey ONCE via CreateApiKeyResult)
 * - Revoke key (confirm dialog → revokeApiKey mutation)
 * - Rotate key (ATOMIC — single rotateApiKey mutation, closes D-E03 / T-008.1a)
 *
 * Session: workspaceId resolved server-side in layout → injected via WorkspaceContext.
 * Replaces the WORKSPACE_ID_FROM_SESSION placeholder (D-E06 / T-008.4).
 *
 * Acceptance criteria (DASH-02):
 * - rawKey displayed only once, with copy button
 * - Rotate is atomic (single BFF mutation, no revoke+create race condition)
 * - Revoke takes effect immediately
 * - Loading / empty / error states present
 * - WCAG 2.1 AA: keyboard nav, labels, contrast
 */
import React, { useState, useCallback, useEffect } from 'react';
import { Header } from '@/components/Header';
import { Card, CardHeader, CardTitle, CardContent } from '@/components/ui/Card';
import { Button } from '@/components/ui/Button';
import { Badge, ActiveBadge } from '@/components/ui/Badge';
import { Input } from '@/components/ui/Input';
import { Dialog } from '@/components/ui/Dialog';
import {
  Table, TableHeader, TableBody, TableRow, TableHead, TableCell,
} from '@/components/ui/Table';
import { LoadingCard } from '@/components/states/LoadingState';
import { EmptyState } from '@/components/states/EmptyState';
import { ErrorState } from '@/components/states/ErrorState';
import { useToast } from '@/components/ui/Toast';
import { gqlClient } from '@/lib/graphql/client';
import {
  API_KEYS_QUERY,
  CREATE_API_KEY_MUTATION,
  REVOKE_API_KEY_MUTATION,
  ROTATE_API_KEY_MUTATION,
} from '@/lib/graphql/queries';
import type {
  ApiKey,
  ApiKeysQueryResponse,
  CreateApiKeyMutationResponse,
  RevokeApiKeyMutationResponse,
  RotateApiKeyMutationResponse,
  CreateApiKeyResult,
} from '@/lib/graphql/types';
import { useWorkspaceId } from '@/lib/context/WorkspaceContext';
import { formatDate } from '@/lib/utils';

/* --------------------------------------------------------------------------- */
/* Raw key display — shown ONCE after creation or rotation                       */
/* --------------------------------------------------------------------------- */
function RawKeyDisplay({ rawKey }: { rawKey: string }) {
  const [copied, setCopied] = useState(false);

  const handleCopy = async () => {
    await navigator.clipboard.writeText(rawKey);
    setCopied(true);
    setTimeout(() => setCopied(false), 2000);
  };

  return (
    <div
      role="alert"
      style={{
        backgroundColor: 'var(--warn-soft)',
        border: '1px solid var(--warn)',
        borderRadius: '9px',
        padding: '14px 16px',
        marginBottom: '16px',
      }}
    >
      <p style={{ fontSize: '12px', color: 'var(--warn)', fontWeight: 600, marginBottom: '8px' }}>
        Copiez cette clé maintenant — elle ne sera plus affichée.
      </p>
      <div
        style={{
          display: 'flex',
          gap: '8px',
          alignItems: 'center',
          flexWrap: 'wrap',
        }}
      >
        <code
          style={{
            fontFamily: 'var(--font-geist-mono, monospace)',
            fontSize: '12px',
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
          {rawKey}
        </code>
        <Button
          variant="secondary"
          size="sm"
          onClick={handleCopy}
          aria-label="Copier la clé API"
        >
          {copied ? 'Copie !' : 'Copier'}
        </Button>
      </div>
    </div>
  );
}

/* --------------------------------------------------------------------------- */
/* Create key modal                                                               */
/* --------------------------------------------------------------------------- */
interface CreateKeyModalProps {
  open: boolean;
  onClose: () => void;
  onCreate: (result: CreateApiKeyResult) => void;
  workspaceId: string;
}

function CreateKeyModal({ open, onClose, onCreate, workspaceId }: CreateKeyModalProps) {
  const [name, setName] = useState('');
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState('');
  const { toast } = useToast();

  const handleCreate = async (e: React.FormEvent) => {
    e.preventDefault();
    if (!name.trim()) {
      setError('Le nom est requis.');
      return;
    }
    setError('');
    setLoading(true);
    try {
      const data = await gqlClient.mutate<CreateApiKeyMutationResponse>(
        CREATE_API_KEY_MUTATION,
        { workspaceId, name: name.trim() },
      );
      onCreate(data.createApiKey);
      setName('');
    } catch (err) {
      const msg = err instanceof Error ? err.message : 'Erreur inconnue';
      toast(msg, 'error');
    } finally {
      setLoading(false);
    }
  };

  const handleClose = () => {
    setName('');
    setError('');
    onClose();
  };

  return (
    <Dialog open={open} onClose={handleClose} title="Créer une clé API">
      <form onSubmit={handleCreate} noValidate style={{ display: 'flex', flexDirection: 'column', gap: '16px' }}>
        <Input
          label="Nom de la clé"
          id="key-name"
          placeholder="ex: Production, CI/CD"
          value={name}
          onChange={(e) => setName(e.target.value)}
          error={error}
          autoFocus
          required
          aria-required="true"
        />
        <p style={{ fontSize: '12px', color: 'var(--text-3)', lineHeight: 1.6 }}>
          La clé brute sera affichée une seule fois immédiatement après la création.
          Copiez-la et stockez-la en lieu sûr.
        </p>
        <div style={{ display: 'flex', gap: '8px', justifyContent: 'flex-end' }}>
          <Button type="button" variant="ghost" onClick={handleClose}>
            Annuler
          </Button>
          <Button type="submit" variant="primary" loading={loading} disabled={loading}>
            Créer la clé
          </Button>
        </div>
      </form>
    </Dialog>
  );
}

/* --------------------------------------------------------------------------- */
/* New key result modal — shows rawKey ONCE (create OR rotate)                   */
/* --------------------------------------------------------------------------- */
interface NewKeyResultModalProps {
  result: CreateApiKeyResult | null;
  title?: string;
  onClose: () => void;
}

function NewKeyResultModal({ result, title = 'Clé API créée', onClose }: NewKeyResultModalProps) {
  return (
    <Dialog open={result !== null} onClose={onClose} title={title}>
      {result && (
        <div style={{ display: 'flex', flexDirection: 'column', gap: '12px' }}>
          <RawKeyDisplay rawKey={result.rawKey} />
          <div style={{ fontSize: '13px', color: 'var(--text-2)' }}>
            <strong>Nom :</strong> {result.apiKey.name}
          </div>
          <div style={{ fontSize: '13px', color: 'var(--text-2)' }}>
            <strong>Préfixe :</strong>{' '}
            <code style={{ fontFamily: 'monospace' }}>{result.apiKey.prefix}...</code>
          </div>
          <div style={{ display: 'flex', justifyContent: 'flex-end', marginTop: '8px' }}>
            <Button variant="primary" onClick={onClose}>
              J'ai copié ma clé
            </Button>
          </div>
        </div>
      )}
    </Dialog>
  );
}

/* --------------------------------------------------------------------------- */
/* Revoke confirm dialog                                                          */
/* --------------------------------------------------------------------------- */
interface RevokeDialogProps {
  keyToRevoke: ApiKey | null;
  onClose: () => void;
  onConfirm: () => Promise<void>;
}

function RevokeDialog({ keyToRevoke, onClose, onConfirm }: RevokeDialogProps) {
  const [loading, setLoading] = useState(false);

  const handleConfirm = async () => {
    setLoading(true);
    await onConfirm();
    setLoading(false);
  };

  return (
    <Dialog open={keyToRevoke !== null} onClose={onClose} title="Révoquer la clé API">
      <div style={{ display: 'flex', flexDirection: 'column', gap: '16px' }}>
        <p style={{ fontSize: '13px', color: 'var(--text-2)', lineHeight: 1.6 }}>
          Êtes-vous sûr de vouloir révoquer la clé{' '}
          <strong style={{ color: 'var(--text)' }}>{keyToRevoke?.name}</strong> ?
          Cette action est irréversible — toutes les intégrations utilisant cette clé cesseront
          de fonctionner immédiatement.
        </p>
        <div style={{ display: 'flex', gap: '8px', justifyContent: 'flex-end' }}>
          <Button type="button" variant="ghost" onClick={onClose}>
            Annuler
          </Button>
          <Button
            type="button"
            variant="danger"
            loading={loading}
            disabled={loading}
            onClick={handleConfirm}
          >
            Révoquer
          </Button>
        </div>
      </div>
    </Dialog>
  );
}

/* --------------------------------------------------------------------------- */
/* Main page                                                                     */
/* --------------------------------------------------------------------------- */
export default function ApiKeysPage() {
  // workspaceId resolved server-side by dashboard layout — no placeholder needed.
  const workspaceId = useWorkspaceId();

  const [keys, setKeys] = useState<ApiKey[]>([]);
  const [loadingKeys, setLoadingKeys] = useState(true);
  const [fetchError, setFetchError] = useState<string | null>(null);

  const [createModalOpen, setCreateModalOpen] = useState(false);
  const [newKeyResult, setNewKeyResult] = useState<CreateApiKeyResult | null>(null);
  const [newKeyModalTitle, setNewKeyModalTitle] = useState('Clé API créée');
  const [keyToRevoke, setKeyToRevoke] = useState<ApiKey | null>(null);
  const [rotatingKeyId, setRotatingKeyId] = useState<string | null>(null);

  const { toast } = useToast();

  const fetchKeys = useCallback(async () => {
    if (!workspaceId) {
      setFetchError('Session non résolue. Veuillez vous reconnecter.');
      setLoadingKeys(false);
      return;
    }
    setLoadingKeys(true);
    setFetchError(null);
    try {
      const data = await gqlClient.query<ApiKeysQueryResponse>(API_KEYS_QUERY, {
        workspaceId,
      });
      setKeys(data.apiKeys);
    } catch (err) {
      setFetchError(err instanceof Error ? err.message : 'Impossible de charger les clés.');
    } finally {
      setLoadingKeys(false);
    }
  }, [workspaceId]);

  useEffect(() => { fetchKeys(); }, [fetchKeys]);

  const handleCreate = (result: CreateApiKeyResult) => {
    setCreateModalOpen(false);
    setNewKeyModalTitle('Clé API créée');
    setNewKeyResult(result);
    setKeys((prev) => [result.apiKey, ...prev]);
    toast('Clé API créée.', 'success');
  };

  const handleRevoke = async () => {
    if (!keyToRevoke) return;
    try {
      await gqlClient.mutate<RevokeApiKeyMutationResponse>(REVOKE_API_KEY_MUTATION, {
        workspaceId,
        keyId: keyToRevoke.id,
      });
      setKeys((prev) =>
        prev.map((k) => k.id === keyToRevoke.id ? { ...k, active: false } : k),
      );
      toast('Clé révoquée.', 'success');
    } catch (err) {
      toast(err instanceof Error ? err.message : 'Erreur lors de la révocation.', 'error');
    } finally {
      setKeyToRevoke(null);
    }
  };

  /**
   * Rotate — single atomic BFF call (Mutation.rotateApiKey).
   * Closes D-E03: replaces the previous revoke+create two-step approach.
   * The rawKey from the result is displayed once in the NewKeyResultModal.
   */
  const handleRotate = async (key: ApiKey) => {
    setRotatingKeyId(key.id);
    try {
      const data = await gqlClient.mutate<RotateApiKeyMutationResponse>(
        ROTATE_API_KEY_MUTATION,
        { workspaceId, keyId: key.id },
      );
      // Replace the old key with the newly created one
      setKeys((prev) => [
        data.rotateApiKey.apiKey,
        ...prev.filter((k) => k.id !== key.id),
      ]);
      setNewKeyModalTitle('Clé tournée — nouvelle clé');
      setNewKeyResult(data.rotateApiKey);
      toast('Rotation effectuée. Copiez votre nouvelle clé.', 'success');
    } catch (err) {
      toast(err instanceof Error ? err.message : 'Erreur lors de la rotation.', 'error');
    } finally {
      setRotatingKeyId(null);
    }
  };

  const activeKeys = keys.filter((k) => k.active);
  const revokedKeys = keys.filter((k) => !k.active);

  return (
    <>
      <Header
        breadcrumb={[{ label: 'Fleece', href: '/dashboard' }, { label: 'API Keys' }]}
        actions={
          <Button
            variant="primary"
            size="sm"
            onClick={() => setCreateModalOpen(true)}
            aria-label="Créer une nouvelle clé API"
          >
            + Créer une clé
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
            API Keys
          </h1>
          <p style={{ fontSize: '13px', color: 'var(--text-3)', marginTop: '4px' }}>
            Gérez les clés d'accès programmatique à votre workspace.
          </p>
        </div>

        {/* Stats */}
        {!loadingKeys && !fetchError && (
          <div style={{ display: 'flex', gap: '12px', flexWrap: 'wrap' }}>
            <div
              style={{
                backgroundColor: 'var(--surface)',
                border: '1px solid var(--border)',
                borderRadius: '9px',
                padding: '12px 16px',
                minWidth: '120px',
              }}
            >
              <div
                style={{
                  fontSize: '22px',
                  fontWeight: 600,
                  color: 'var(--accent-text)',
                  fontVariantNumeric: 'tabular-nums',
                }}
              >
                {activeKeys.length}
              </div>
              <div style={{ fontSize: '11px', color: 'var(--text-3)', marginTop: '2px' }}>
                Clés actives
              </div>
            </div>
            <div
              style={{
                backgroundColor: 'var(--surface)',
                border: '1px solid var(--border)',
                borderRadius: '9px',
                padding: '12px 16px',
                minWidth: '120px',
              }}
            >
              <div
                style={{
                  fontSize: '22px',
                  fontWeight: 600,
                  color: 'var(--text-2)',
                  fontVariantNumeric: 'tabular-nums',
                }}
              >
                {revokedKeys.length}
              </div>
              <div style={{ fontSize: '11px', color: 'var(--text-3)', marginTop: '2px' }}>
                Clés révoquées
              </div>
            </div>
          </div>
        )}

        {/* Table card */}
        <Card>
          <CardHeader>
            <CardTitle>Toutes les clés</CardTitle>
            <Button
              variant="primary"
              size="sm"
              onClick={() => setCreateModalOpen(true)}
              aria-label="Créer une nouvelle clé API"
            >
              + Créer
            </Button>
          </CardHeader>
          <CardContent style={{ padding: 0 }}>
            {loadingKeys && <LoadingCard />}
            {!loadingKeys && fetchError && (
              <ErrorState message={fetchError} onRetry={fetchKeys} />
            )}
            {!loadingKeys && !fetchError && keys.length === 0 && (
              <EmptyState
                title="Aucune clé API"
                description="Créez votre première clé API pour commencer à envoyer des messages via l'API Fleece."
                action={{ label: '+ Créer une clé', onClick: () => setCreateModalOpen(true) }}
                icon="key"
              />
            )}
            {!loadingKeys && !fetchError && keys.length > 0 && (
              <Table>
                <TableHeader>
                  <TableRow>
                    <TableHead>Nom</TableHead>
                    <TableHead>Préfixe</TableHead>
                    <TableHead>Statut</TableHead>
                    <TableHead>Créée le</TableHead>
                    <TableHead>Expiration</TableHead>
                    <TableHead style={{ textAlign: 'right' }}>Actions</TableHead>
                  </TableRow>
                </TableHeader>
                <TableBody>
                  {keys.map((key) => (
                    <TableRow key={key.id} hoverable>
                      <TableCell>
                        <span style={{ fontWeight: 500 }}>{key.name}</span>
                      </TableCell>
                      <TableCell>
                        <code
                          style={{
                            fontFamily: 'var(--font-geist-mono, monospace)',
                            fontSize: '12px',
                            color: 'var(--text-2)',
                            backgroundColor: 'var(--surface)',
                            padding: '2px 6px',
                            borderRadius: '4px',
                          }}
                        >
                          {key.prefix}…
                        </code>
                      </TableCell>
                      <TableCell>
                        <ActiveBadge active={key.active} />
                      </TableCell>
                      <TableCell style={{ color: 'var(--text-2)', fontSize: '12px' }}>
                        {formatDate(key.createdAt)}
                      </TableCell>
                      <TableCell style={{ color: 'var(--text-3)', fontSize: '12px' }}>
                        {key.expiresAt ? formatDate(key.expiresAt) : (
                          <Badge variant="neutral">Jamais</Badge>
                        )}
                      </TableCell>
                      <TableCell style={{ textAlign: 'right' }}>
                        <div
                          style={{ display: 'flex', gap: '6px', justifyContent: 'flex-end' }}
                          role="group"
                          aria-label={`Actions pour la clé ${key.name}`}
                        >
                          {key.active && (
                            <>
                              <Button
                                variant="ghost"
                                size="sm"
                                onClick={() => handleRotate(key)}
                                loading={rotatingKeyId === key.id}
                                disabled={rotatingKeyId !== null}
                                aria-label={`Tourner la clé ${key.name}`}
                              >
                                Rotation
                              </Button>
                              <Button
                                variant="danger"
                                size="sm"
                                onClick={() => setKeyToRevoke(key)}
                                disabled={rotatingKeyId !== null}
                                aria-label={`Révoquer la clé ${key.name}`}
                              >
                                Révoquer
                              </Button>
                            </>
                          )}
                          {!key.active && (
                            <span style={{ fontSize: '12px', color: 'var(--text-3)' }}>—</span>
                          )}
                        </div>
                      </TableCell>
                    </TableRow>
                  ))}
                </TableBody>
              </Table>
            )}
          </CardContent>
        </Card>
      </div>

      {/* Modals */}
      <CreateKeyModal
        open={createModalOpen}
        onClose={() => setCreateModalOpen(false)}
        onCreate={handleCreate}
        workspaceId={workspaceId}
      />
      <NewKeyResultModal
        result={newKeyResult}
        title={newKeyModalTitle}
        onClose={() => setNewKeyResult(null)}
      />
      <RevokeDialog
        keyToRevoke={keyToRevoke}
        onClose={() => setKeyToRevoke(null)}
        onConfirm={handleRevoke}
      />
    </>
  );
}
