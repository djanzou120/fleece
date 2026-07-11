/**
 * TypeScript types derived from the GraphQL schema at
 * src/graphql-api/adapters/graphql/schema.graphql
 *
 * These are hand-authored to mirror the SDL exactly (offline — no codegen).
 * DEBT: Once npm install is available, wire up @graphql-codegen/typescript
 * (configured in src/graphql/) to auto-generate this file.
 *
 * Last updated: aligned to SDL after BFF added:
 *   - Query.transactions (DASH-03 / D-E01)
 *   - Mutation.rotateApiKey (DASH-02 / D-E03)
 *   - Mutation.createWorkspace (DASH-01 / T-008.1f)
 *   - WebhookEndpoint.secretPreview (DASH-04 / D-E02)
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
  /** Transaction type: credit | debit | refund */
  type: string;
  /** Amount in smallest currency unit (centimes). Always positive — sign derived from type. */
  amount: number;
  /** ISO 4217 currency code */
  currency: string;
  /** Human-readable description (e.g. "Message SMS to +221XXXXXXXXX") */
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
  /**
   * Masked preview of the HMAC signing secret.
   * Format: ••••<last 4 chars> or •••••••• if secret unavailable.
   * The full secret is NEVER exposed via the GraphQL API.
   * Added to SDL: BFF exposes WebhookEndpoint.secretPreview — closes D-E02.
   */
  secretPreview?: string | null;
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

/** DASH-03 / D-E01: Transaction history query response. */
export interface TransactionsQueryResponse {
  transactions: TransactionPage;
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

/** DASH-01 / T-008.1f: Create workspace during onboarding. */
export interface CreateWorkspaceMutationResponse {
  createWorkspace: Workspace;
}

export interface CreateApiKeyMutationResponse {
  createApiKey: CreateApiKeyResult;
}

export interface RevokeApiKeyMutationResponse {
  revokeApiKey: boolean;
}

/**
 * DASH-02 / D-E03: Atomic API key rotation.
 * Returns the same shape as createApiKey (rawKey displayed once).
 */
export interface RotateApiKeyMutationResponse {
  rotateApiKey: CreateApiKeyResult;
}

export interface CreateWebhookEndpointMutationResponse {
  createWebhookEndpoint: WebhookEndpoint;
}

export interface DeleteWebhookEndpointMutationResponse {
  deleteWebhookEndpoint: boolean;
}
