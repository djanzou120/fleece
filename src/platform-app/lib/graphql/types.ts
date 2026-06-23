/**
 * TypeScript types derived from the GraphQL schema at
 * src/graphql-api/adapters/graphql/schema.graphql
 *
 * These are hand-authored to mirror the SDL exactly (offline — no codegen).
 * DEBT: Once npm install is available, wire up @graphql-codegen/typescript
 * (configured in src/graphql/) to auto-generate this file.
 */

/* --------------------------------------------------------------------------- */
/* Scalars & shared                                                               */
/* --------------------------------------------------------------------------- */

export type ID = string;

export interface PageInfo {
  nextCursor: string | null;
  hasNextPage: boolean;
}

/* --------------------------------------------------------------------------- */
/* Workspace                                                                     */
/* --------------------------------------------------------------------------- */

export interface Workspace {
  id: ID;
  name: string;
  country: string;
  slug: string;
  plan: string;
  createdAt: string;
}

/* --------------------------------------------------------------------------- */
/* API Keys                                                                      */
/* --------------------------------------------------------------------------- */

export interface ApiKey {
  id: ID;
  workspaceId: ID;
  name: string;
  prefix: string;
  active: boolean;
  createdAt: string;
  expiresAt: string | null;
}

export interface CreateApiKeyResult {
  /** Raw key — displayed ONCE, never stored in plain text. */
  rawKey: string;
  apiKey: ApiKey;
}

/* --------------------------------------------------------------------------- */
/* Wallet                                                                        */
/* --------------------------------------------------------------------------- */

export interface WalletBalance {
  workspaceId: ID;
  /** Amount in smallest currency unit (centimes / kobo / etc.) */
  balance: number;
  /** ISO 4217 currency code (e.g. XOF, EUR) */
  currency: string;
}

export interface Transaction {
  id: ID;
  workspaceId: ID;
  type: string;
  amount: number;
  currency: string;
  description: string;
  createdAt: string;
}

export interface TransactionPage {
  items: Transaction[];
  pageInfo: PageInfo;
}

/* --------------------------------------------------------------------------- */
/* Messages                                                                      */
/* --------------------------------------------------------------------------- */

export interface Message {
  id: ID;
  workspaceId: ID;
  to: string;
  channel: string;
  content: string;
  status: string;
  createdAt: string;
  updatedAt: string;
}

export interface MessagePage {
  items: Message[];
  pageInfo: PageInfo;
}

/* --------------------------------------------------------------------------- */
/* Webhooks                                                                      */
/* --------------------------------------------------------------------------- */

export interface WebhookEndpoint {
  id: ID;
  workspaceId: ID;
  url: string;
  events: string[];
  active: boolean;
  createdAt: string;
  // NOTE: `secret` is NOT in the current SDL (see schema.graphql).
  // DEBT BFF: add `secret: String` to WebhookEndpoint in the SDL and expose it
  // via the resolver so the dashboard can display the masked signing secret.
}

/* --------------------------------------------------------------------------- */
/* Query response shapes                                                          */
/* --------------------------------------------------------------------------- */

export interface WorkspaceQueryResponse {
  workspace: Workspace | null;
}

export interface ApiKeysQueryResponse {
  apiKeys: ApiKey[];
}

export interface WalletBalanceQueryResponse {
  walletBalance: WalletBalance;
}

export interface MessagesQueryResponse {
  messages: MessagePage;
}

export interface WebhookEndpointsQueryResponse {
  webhookEndpoints: WebhookEndpoint[];
}

/* --------------------------------------------------------------------------- */
/* Mutation response shapes                                                       */
/* --------------------------------------------------------------------------- */

export interface CreateApiKeyMutationResponse {
  createApiKey: CreateApiKeyResult;
}

export interface RevokeApiKeyMutationResponse {
  revokeApiKey: boolean;
}

export interface CreateWebhookEndpointMutationResponse {
  createWebhookEndpoint: WebhookEndpoint;
}

export interface DeleteWebhookEndpointMutationResponse {
  deleteWebhookEndpoint: boolean;
}
