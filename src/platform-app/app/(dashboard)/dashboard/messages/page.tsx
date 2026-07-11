'use client';
/**
 * Messages page — lists sent messages.
 * Consumes messages(workspaceId, cursor, limit) Query.
 */
import React, { useState, useCallback, useEffect } from 'react';
import { Header } from '@/components/Header';
import { Card, CardHeader, CardTitle, CardContent } from '@/components/ui/Card';
import {
  Table, TableHeader, TableBody, TableRow, TableHead, TableCell,
} from '@/components/ui/Table';
import { Badge, StatusBadge } from '@/components/ui/Badge';
import { Button } from '@/components/ui/Button';
import { LoadingCard } from '@/components/states/LoadingState';
import { EmptyState } from '@/components/states/EmptyState';
import { ErrorState } from '@/components/states/ErrorState';
import { gqlClient } from '@/lib/graphql/client';
import { MESSAGES_QUERY } from '@/lib/graphql/queries';
import type { Message, MessagesQueryResponse } from '@/lib/graphql/types';
import { formatDateTime, truncate } from '@/lib/utils';
import { useWorkspaceId } from '@/lib/context/WorkspaceContext';

const PAGE_SIZE = 20;

export default function MessagesPage() {
  const workspaceId = useWorkspaceId();
  const [messages, setMessages] = useState<Message[]>([]);
  const [nextCursor, setNextCursor] = useState<string | null>(null);
  const [hasNextPage, setHasNextPage] = useState(false);
  const [loading, setLoading] = useState(true);
  const [loadingMore, setLoadingMore] = useState(false);
  const [fetchError, setFetchError] = useState<string | null>(null);

  const fetchMessages = useCallback(async (cursor?: string) => {
    if (!workspaceId) return;
    if (!cursor) setLoading(true);
    else setLoadingMore(true);
    setFetchError(null);
    try {
      const data = await gqlClient.query<MessagesQueryResponse>(MESSAGES_QUERY, {
        workspaceId,
        cursor: cursor ?? null,
        limit: PAGE_SIZE,
      });
      if (cursor) {
        setMessages((prev) => [...prev, ...data.messages.items]);
      } else {
        setMessages(data.messages.items);
      }
      setNextCursor(data.messages.pageInfo.nextCursor);
      setHasNextPage(data.messages.pageInfo.hasNextPage);
    } catch (err) {
      setFetchError(err instanceof Error ? err.message : 'Impossible de charger les messages.');
    } finally {
      setLoading(false);
      setLoadingMore(false);
    }
  }, [workspaceId]);

  useEffect(() => { fetchMessages(); }, [fetchMessages]);

  const channelColor: Record<string, string> = {
    whatsapp: 'var(--accent)',
    sms: 'var(--info)',
    telegram: '#3aa6e0',
  };

  return (
    <>
      <Header
        breadcrumb={[{ label: 'Fleece', href: '/dashboard' }, { label: 'Messages' }]}
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
            Messages
          </h1>
          <p style={{ fontSize: '13px', color: 'var(--text-3)', marginTop: '4px' }}>
            Historique des messages envoyés via votre workspace.
          </p>
        </div>

        <Card>
          <CardHeader>
            <CardTitle>Tous les messages</CardTitle>
          </CardHeader>
          <CardContent style={{ padding: 0 }}>
            {loading && <LoadingCard />}
            {!loading && fetchError && (
              <ErrorState message={fetchError} onRetry={() => fetchMessages()} />
            )}
            {!loading && !fetchError && messages.length === 0 && (
              <EmptyState
                title="Aucun message envoyé"
                description="Les messages envoyés via l'API REST apparaîtront ici."
                icon="✉"
              />
            )}
            {!loading && !fetchError && messages.length > 0 && (
              <>
                <Table>
                  <TableHeader>
                    <TableRow>
                      <TableHead>Destinataire</TableHead>
                      <TableHead>Canal</TableHead>
                      <TableHead>Contenu</TableHead>
                      <TableHead>Statut</TableHead>
                      <TableHead>Envoyé le</TableHead>
                    </TableRow>
                  </TableHeader>
                  <TableBody>
                    {messages.map((msg) => (
                      <TableRow key={msg.id} hoverable>
                        <TableCell>
                          <code
                            style={{
                              fontFamily: 'var(--font-geist-mono, monospace)',
                              fontSize: '12px',
                              color: 'var(--text-2)',
                            }}
                          >
                            {msg.to}
                          </code>
                        </TableCell>
                        <TableCell>
                          <Badge
                            variant="neutral"
                            style={{
                              color: channelColor[msg.channel.toLowerCase()] ?? 'var(--text-2)',
                              backgroundColor: 'var(--surface)',
                            }}
                          >
                            {msg.channel}
                          </Badge>
                        </TableCell>
                        <TableCell style={{ color: 'var(--text-2)', maxWidth: '240px' }}>
                          <span className="truncate" style={{ display: 'block' }}>
                            {truncate(msg.content, 60)}
                          </span>
                        </TableCell>
                        <TableCell>
                          <StatusBadge status={msg.status} />
                        </TableCell>
                        <TableCell style={{ color: 'var(--text-3)', fontSize: '12px' }}>
                          {formatDateTime(msg.createdAt)}
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
                      onClick={() => fetchMessages(nextCursor ?? undefined)}
                      loading={loadingMore}
                      disabled={loadingMore}
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
    </>
  );
}
