'use client';
/**
 * DASH-04 — Webhooks configuration page.
 *
 * Features:
 * - List webhook endpoints (url, events, active badge, createdAt)
 * - Secret field: shown as "••••••••" placeholder (DEBT — not in SDL)
 * - Add endpoint modal (url + multi-select events → createWebhookEndpoint)
 * - Delete endpoint (confirm dialog → deleteWebhookEndpoint)
 * - Delivery history: empty-state (DEBT — no deliveries Query/type in SDL)
 *
 * DEBT — webhook secret: The SDL WebhookEndpoint type has no `secret` field.
 * BFF TODO: add `secret: String` (or masked `secretPreview: String`) to
 * WebhookEndpoint so the dashboard can display the HMAC signing secret.
 *
 * DEBT — webhook deliveries: No `deliveries` field on WebhookEndpoint and no
 * standalone deliveries query in the SDL. BFF TODO: expose
 * `WebhookEndpoint.deliveries: [WebhookDelivery!]!` or a separate query.
 *
 * Acceptance criteria (DASH-04):
 * - List endpoints with url, events, active, createdAt
 * - Secret masked "••••" with debt note
 * - Add endpoint via createWebhookEndpoint mutation
 * - Delete with confirmation via deleteWebhookEndpoint mutation
 * - Deliveries empty-state with debt annotation
 * - Loading / empty / error states
 * - WCAG 2.1 AA
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
  WEBHOOK_ENDPOINTS_QUERY,
  CREATE_WEBHOOK_ENDPOINT_MUTATION,
  DELETE_WEBHOOK_ENDPOINT_MUTATION,
} from '@/lib/graphql/queries';
import type {
  WebhookEndpoint,
  WebhookEndpointsQueryResponse,
  CreateWebhookEndpointMutationResponse,
  DeleteWebhookEndpointMutationResponse,
} from '@/lib/graphql/types';
import { formatDate } from '@/lib/utils';

const WORKSPACE_ID = 'WORKSPACE_ID_FROM_SESSION';

/* Available webhook event types (from API-03 / TDD §4.6) */
const AVAILABLE_EVENTS = [
  { value: 'message.created',   label: 'message.created' },
  { value: 'message.queued',    label: 'message.queued' },
  { value: 'message.sent',      label: 'message.sent' },
  { value: 'message.delivered', label: 'message.delivered' },
  { value: 'message.failed',    label: 'message.failed' },
  { value: 'wallet.debited',    label: 'wallet.debited' },
  { value: 'wallet.refunded',   label: 'wallet.refunded' },
];

/* --------------------------------------------------------------------------- */
/* Add endpoint modal                                                             */
/* --------------------------------------------------------------------------- */
interface AddEndpointModalProps {
  open: boolean;
  onClose: () => void;
  onCreated: (endpoint: WebhookEndpoint) => void;
}

function AddEndpointModal({ open, onClose, onCreated }: AddEndpointModalProps) {
  const [url, setUrl] = useState('');
  const [selectedEvents, setSelectedEvents] = useState<string[]>([]);
  const [urlError, setUrlError] = useState('');
  const [eventsError, setEventsError] = useState('');
  const [loading, setLoading] = useState(false);
  const { toast } = useToast();

  const toggleEvent = (value: string) => {
    setSelectedEvents((prev) =>
      prev.includes(value) ? prev.filter((e) => e !== value) : [...prev, value],
    );
  };

  const validate = (): boolean => {
    let valid = true;
    if (!url.trim()) {
      setUrlError('L\'URL est requise.');
      valid = false;
    } else if (!url.startsWith('https://')) {
      setUrlError('L\'URL doit commencer par https://');
      valid = false;
    } else {
      setUrlError('');
    }
    if (selectedEvents.length === 0) {
      setEventsError('Sélectionnez au moins un événement.');
      valid = false;
    } else {
      setEventsError('');
    }
    return valid;
  };

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    if (!validate()) return;
    setLoading(true);
    try {
      const data = await gqlClient.mutate<CreateWebhookEndpointMutationResponse>(
        CREATE_WEBHOOK_ENDPOINT_MUTATION,
        { workspaceId: WORKSPACE_ID, url: url.trim(), events: selectedEvents },
      );
      onCreated(data.createWebhookEndpoint);
      setUrl('');
      setSelectedEvents([]);
      toast('Endpoint créé.', 'success');
    } catch (err) {
      toast(err instanceof Error ? err.message : 'Erreur lors de la création.', 'error');
    } finally {
      setLoading(false);
    }
  };

  const handleClose = () => {
    setUrl('');
    setSelectedEvents([]);
    setUrlError('');
    setEventsError('');
    onClose();
  };

  return (
    <Dialog open={open} onClose={handleClose} title="Ajouter un endpoint webhook">
      <form onSubmit={handleSubmit} noValidate style={{ display: 'flex', flexDirection: 'column', gap: '16px' }}>
        <Input
          label="URL de l'endpoint"
          id="webhook-url"
          type="url"
          placeholder="https://your-server.com/webhook"
          value={url}
          onChange={(e) => setUrl(e.target.value)}
          error={urlError}
          hint="HTTPS requis"
          autoFocus
          required
          aria-required="true"
        />

        {/* Event multi-select */}
        <div role="group" aria-labelledby="events-label">
          <div
            id="events-label"
            style={{ fontSize: '12px', fontWeight: 500, color: 'var(--text-2)', marginBottom: '8px' }}
          >
            Événements souscrits
          </div>
          <div
            style={{
              display: 'flex',
              flexWrap: 'wrap',
              gap: '6px',
            }}
          >
            {AVAILABLE_EVENTS.map((evt) => {
              const checked = selectedEvents.includes(evt.value);
              return (
                <label
                  key={evt.value}
                  style={{
                    display: 'inline-flex',
                    alignItems: 'center',
                    gap: '6px',
                    padding: '5px 10px',
                    borderRadius: '6px',
                    border: `1px solid ${checked ? 'var(--accent)' : 'var(--border)'}`,
                    backgroundColor: checked ? 'var(--accent-soft)' : 'var(--surface)',
                    fontSize: '12px',
                    color: checked ? 'var(--accent-text)' : 'var(--text-2)',
                    cursor: 'pointer',
                    userSelect: 'none',
                    fontFamily: 'var(--font-geist-mono, monospace)',
                    transition: 'all 0.1s',
                  }}
                >
                  <input
                    type="checkbox"
                    checked={checked}
                    onChange={() => toggleEvent(evt.value)}
                    style={{ width: '12px', height: '12px', accentColor: 'var(--accent)' }}
                    aria-label={evt.label}
                  />
                  {evt.label}
                </label>
              );
            })}
          </div>
          {eventsError && (
            <span role="alert" style={{ fontSize: '11px', color: 'var(--danger)', marginTop: '6px', display: 'block' }}>
              {eventsError}
            </span>
          )}
        </div>

        <div style={{ display: 'flex', gap: '8px', justifyContent: 'flex-end' }}>
          <Button type="button" variant="ghost" onClick={handleClose}>
            Annuler
          </Button>
          <Button type="submit" variant="primary" loading={loading} disabled={loading}>
            Créer l'endpoint
          </Button>
        </div>
      </form>
    </Dialog>
  );
}

/* --------------------------------------------------------------------------- */
/* Delete confirm dialog                                                          */
/* --------------------------------------------------------------------------- */
interface DeleteDialogProps {
  endpoint: WebhookEndpoint | null;
  onClose: () => void;
  onConfirm: () => Promise<void>;
}

function DeleteDialog({ endpoint, onClose, onConfirm }: DeleteDialogProps) {
  const [loading, setLoading] = useState(false);

  const handleConfirm = async () => {
    setLoading(true);
    await onConfirm();
    setLoading(false);
  };

  return (
    <Dialog open={endpoint !== null} onClose={onClose} title="Supprimer l'endpoint webhook">
      <div style={{ display: 'flex', flexDirection: 'column', gap: '16px' }}>
        <p style={{ fontSize: '13px', color: 'var(--text-2)', lineHeight: 1.6 }}>
          Êtes-vous sûr de vouloir supprimer l'endpoint{' '}
          <strong style={{ color: 'var(--text)', fontFamily: 'var(--font-geist-mono, monospace)', fontSize: '12px' }}>
            {endpoint?.url}
          </strong>{' '}
          ? Fleece cessera d'envoyer des événements à cette URL.
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
            Supprimer
          </Button>
        </div>
      </div>
    </Dialog>
  );
}

/* --------------------------------------------------------------------------- */
/* Endpoint row — expandable to show details                                     */
/* --------------------------------------------------------------------------- */
function EndpointRow({
  endpoint,
  onDelete,
}: {
  endpoint: WebhookEndpoint;
  onDelete: (ep: WebhookEndpoint) => void;
}) {
  const [expanded, setExpanded] = useState(false);

  return (
    <>
      <TableRow hoverable>
        <TableCell>
          <button
            onClick={() => setExpanded((v) => !v)}
            aria-expanded={expanded}
            aria-label={`${expanded ? 'Réduire' : 'Développer'} les détails de ${endpoint.url}`}
            style={{
              background: 'none',
              border: 'none',
              cursor: 'pointer',
              color: 'var(--text)',
              fontSize: '13px',
              fontFamily: 'var(--font-geist-mono, monospace)',
              textAlign: 'left',
              display: 'flex',
              alignItems: 'center',
              gap: '6px',
              padding: 0,
            }}
          >
            <span aria-hidden="true" style={{ fontSize: '10px', color: 'var(--text-3)' }}>
              {expanded ? '▼' : '▶'}
            </span>
            {endpoint.url}
          </button>
        </TableCell>
        <TableCell>
          <div style={{ display: 'flex', flexWrap: 'wrap', gap: '4px' }}>
            {endpoint.events.slice(0, 3).map((evt) => (
              <Badge key={evt} variant="info" style={{ fontSize: '10px', padding: '2px 6px' }}>
                {evt}
              </Badge>
            ))}
            {endpoint.events.length > 3 && (
              <Badge variant="neutral" style={{ fontSize: '10px', padding: '2px 6px' }}>
                +{endpoint.events.length - 3}
              </Badge>
            )}
          </div>
        </TableCell>
        <TableCell>
          <ActiveBadge active={endpoint.active} />
        </TableCell>
        <TableCell style={{ color: 'var(--text-2)', fontSize: '12px' }}>
          {formatDate(endpoint.createdAt)}
        </TableCell>
        <TableCell>
          {/* Secret masked — DEBT: not in SDL */}
          <code
            style={{
              fontFamily: 'var(--font-geist-mono, monospace)',
              fontSize: '12px',
              color: 'var(--text-3)',
              letterSpacing: '0.1em',
            }}
            title="Secret non disponible — voir DEBT BFF"
          >
            ••••••••
          </code>
        </TableCell>
        <TableCell style={{ textAlign: 'right' }}>
          <Button
            variant="danger"
            size="sm"
            onClick={() => onDelete(endpoint)}
            aria-label={`Supprimer l'endpoint ${endpoint.url}`}
          >
            Supprimer
          </Button>
        </TableCell>
      </TableRow>

      {/* Expanded details */}
      {expanded && (
        <TableRow>
          <td
            colSpan={6}
            style={{
              padding: '12px 16px 16px 32px',
              backgroundColor: 'var(--surface)',
              borderBottom: '1px solid var(--border-soft)',
            }}
          >
            <div style={{ display: 'flex', flexDirection: 'column', gap: '10px' }}>
              <div>
                <div style={{ fontSize: '11px', color: 'var(--text-3)', marginBottom: '4px' }}>
                  Tous les événements
                </div>
                <div style={{ display: 'flex', flexWrap: 'wrap', gap: '4px' }}>
                  {endpoint.events.map((evt) => (
                    <Badge key={evt} variant="info" style={{ fontSize: '11px' }}>
                      {evt}
                    </Badge>
                  ))}
                </div>
              </div>
              <div>
                <div style={{ fontSize: '11px', color: 'var(--text-3)', marginBottom: '4px' }}>
                  Secret HMAC
                </div>
                <div
                  style={{
                    display: 'flex',
                    alignItems: 'center',
                    gap: '8px',
                    backgroundColor: 'var(--surface-2)',
                    border: '1px solid var(--border)',
                    borderRadius: '6px',
                    padding: '8px 12px',
                    fontSize: '12px',
                  }}
                >
                  <code style={{ fontFamily: 'monospace', color: 'var(--text-3)', flex: 1 }}>
                    ••••••••••••••••••••••••••••••••
                  </code>
                  <span
                    style={{
                      fontSize: '10px',
                      color: 'var(--warn)',
                      backgroundColor: 'var(--warn-soft)',
                      padding: '2px 6px',
                      borderRadius: '4px',
                    }}
                  >
                    DEBT: champ absent du SDL
                  </span>
                </div>
              </div>
            </div>
          </td>
        </TableRow>
      )}
    </>
  );
}

/* --------------------------------------------------------------------------- */
/* Main page                                                                     */
/* --------------------------------------------------------------------------- */
export default function WebhooksPage() {
  const [endpoints, setEndpoints] = useState<WebhookEndpoint[]>([]);
  const [loading, setLoading] = useState(true);
  const [fetchError, setFetchError] = useState<string | null>(null);
  const [addModalOpen, setAddModalOpen] = useState(false);
  const [endpointToDelete, setEndpointToDelete] = useState<WebhookEndpoint | null>(null);
  const { toast } = useToast();

  const fetchEndpoints = useCallback(async () => {
    setLoading(true);
    setFetchError(null);
    try {
      const data = await gqlClient.query<WebhookEndpointsQueryResponse>(
        WEBHOOK_ENDPOINTS_QUERY,
        { workspaceId: WORKSPACE_ID },
      );
      setEndpoints(data.webhookEndpoints);
    } catch (err) {
      setFetchError(err instanceof Error ? err.message : 'Impossible de charger les endpoints.');
    } finally {
      setLoading(false);
    }
  }, []);

  useEffect(() => { fetchEndpoints(); }, [fetchEndpoints]);

  const handleCreated = (endpoint: WebhookEndpoint) => {
    setEndpoints((prev) => [endpoint, ...prev]);
    setAddModalOpen(false);
  };

  const handleDelete = async () => {
    if (!endpointToDelete) return;
    try {
      await gqlClient.mutate<DeleteWebhookEndpointMutationResponse>(
        DELETE_WEBHOOK_ENDPOINT_MUTATION,
        { workspaceId: WORKSPACE_ID, id: endpointToDelete.id },
      );
      setEndpoints((prev) => prev.filter((ep) => ep.id !== endpointToDelete.id));
      toast('Endpoint supprimé.', 'success');
    } catch (err) {
      toast(err instanceof Error ? err.message : 'Erreur lors de la suppression.', 'error');
    } finally {
      setEndpointToDelete(null);
    }
  };

  const activeCount = endpoints.filter((e) => e.active).length;

  return (
    <>
      <Header
        breadcrumb={[{ label: 'Fleece', href: '/dashboard' }, { label: 'Webhooks' }]}
        actions={
          <Button
            variant="primary"
            size="sm"
            onClick={() => setAddModalOpen(true)}
            aria-label="Ajouter un endpoint webhook"
          >
            + Ajouter un endpoint
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
            Webhooks
          </h1>
          <p style={{ fontSize: '13px', color: 'var(--text-3)', marginTop: '4px' }}>
            Configurez les endpoints qui recevront les événements Fleece signés.
          </p>
        </div>

        {/* Stats */}
        {!loading && !fetchError && endpoints.length > 0 && (
          <div style={{ display: 'flex', gap: '12px', flexWrap: 'wrap' }}>
            {[
              { label: 'Endpoints actifs', value: activeCount, color: 'var(--accent-text)' },
              { label: 'Total', value: endpoints.length, color: 'var(--text-2)' },
            ].map((stat) => (
              <div
                key={stat.label}
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
                    color: stat.color,
                    fontVariantNumeric: 'tabular-nums',
                  }}
                >
                  {stat.value}
                </div>
                <div style={{ fontSize: '11px', color: 'var(--text-3)', marginTop: '2px' }}>
                  {stat.label}
                </div>
              </div>
            ))}
          </div>
        )}

        {/* Endpoints table */}
        <Card>
          <CardHeader>
            <CardTitle>Endpoints</CardTitle>
            <Button
              variant="primary"
              size="sm"
              onClick={() => setAddModalOpen(true)}
              aria-label="Ajouter un endpoint webhook"
            >
              + Ajouter
            </Button>
          </CardHeader>
          <CardContent style={{ padding: 0 }}>
            {loading && <LoadingCard />}
            {!loading && fetchError && (
              <ErrorState message={fetchError} onRetry={fetchEndpoints} />
            )}
            {!loading && !fetchError && endpoints.length === 0 && (
              <EmptyState
                title="Aucun endpoint configuré"
                description="Ajoutez un endpoint webhook HTTPS pour recevoir les événements de messages et de wallet en temps réel."
                action={{ label: '+ Ajouter un endpoint', onClick: () => setAddModalOpen(true) }}
                icon="↗"
              />
            )}
            {!loading && !fetchError && endpoints.length > 0 && (
              <Table>
                <TableHeader>
                  <TableRow>
                    <TableHead>URL</TableHead>
                    <TableHead>Événements</TableHead>
                    <TableHead>Statut</TableHead>
                    <TableHead>Créé le</TableHead>
                    <TableHead>Secret</TableHead>
                    <TableHead style={{ textAlign: 'right' }}>Actions</TableHead>
                  </TableRow>
                </TableHeader>
                <TableBody>
                  {endpoints.map((ep) => (
                    <EndpointRow
                      key={ep.id}
                      endpoint={ep}
                      onDelete={setEndpointToDelete}
                    />
                  ))}
                </TableBody>
              </Table>
            )}
          </CardContent>
        </Card>

        {/* Delivery history — DEBT */}
        <Card>
          <CardHeader>
            <CardTitle>Historique des livraisons</CardTitle>
          </CardHeader>
          <CardContent style={{ padding: 0 }}>
            <EmptyState
              title="Historique non disponible"
              description="L'historique des livraisons sera disponible une fois que le BFF exposera le champ WebhookEndpoint.deliveries ou une query dédiée."
              icon="📬"
              compact
            />
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
                WebhookEndpoint.deliveries: [WebhookDelivery!]!
              </code>{' '}
              or{' '}
              <code style={{ color: 'var(--warn)' }}>
                Query.webhookDeliveries(endpointId: ID!, ...): WebhookDeliveryPage!
              </code>
            </div>
          </CardContent>
        </Card>
      </div>

      {/* Modals */}
      <AddEndpointModal
        open={addModalOpen}
        onClose={() => setAddModalOpen(false)}
        onCreated={handleCreated}
      />
      <DeleteDialog
        endpoint={endpointToDelete}
        onClose={() => setEndpointToDelete(null)}
        onConfirm={handleDelete}
      />
    </>
  );
}
