/**
 * GraphQL query/mutation documents.
 * Kept as plain strings — no gql tag dependency (offline).
 * DEBT: Replace with typed documents from @graphql-codegen once online.
 *
 * All operations are aligned to src/graphql-api/adapters/graphql/schema.graphql.
 * Zero phantom operations — every field/argument listed here exists in the SDL.
 */

/* --------------------------------------------------------------------------- */
/* Queries                                                                        */
/* --------------------------------------------------------------------------- */

export const WORKSPACE_QUERY = /* GraphQL */ `
  query GetWorkspace($id: ID!) {
    workspace(id: $id) {
      id
      name
      country
      slug
      plan
      createdAt
    }
  }
`;

export const API_KEYS_QUERY = /* GraphQL */ `
  query GetApiKeys($workspaceId: ID!) {
    apiKeys(workspaceId: $workspaceId) {
      id
      workspaceId
      name
      prefix
      active
      createdAt
      expiresAt
    }
  }
`;

export const WALLET_BALANCE_QUERY = /* GraphQL */ `
  query GetWalletBalance($workspaceId: ID!) {
    walletBalance(workspaceId: $workspaceId) {
      workspaceId
      balance
      currency
    }
  }
`;

/**
 * DASH-03 — Transaction history (paginated, cursor-based).
 * SDL: Query.transactions(workspaceId: ID!, cursor: String, limit: Int): TransactionPage!
 * Closes D-E01 / T-008.1b.
 */
export const TRANSACTIONS_QUERY = /* GraphQL */ `
  query GetTransactions($workspaceId: ID!, $cursor: String, $limit: Int) {
    transactions(workspaceId: $workspaceId, cursor: $cursor, limit: $limit) {
      items {
        id
        workspaceId
        type
        amount
        currency
        description
        createdAt
      }
      pageInfo {
        nextCursor
        hasNextPage
      }
    }
  }
`;

export const MESSAGES_QUERY = /* GraphQL */ `
  query GetMessages($workspaceId: ID!, $cursor: String, $limit: Int) {
    messages(workspaceId: $workspaceId, cursor: $cursor, limit: $limit) {
      items {
        id
        workspaceId
        to
        channel
        content
        status
        createdAt
        updatedAt
      }
      pageInfo {
        nextCursor
        hasNextPage
      }
    }
  }
`;

/**
 * DASH-04 — Webhook endpoints list.
 * Includes secretPreview field added to WebhookEndpoint in the SDL.
 * Closes D-E02 / T-008.1c.
 */
export const WEBHOOK_ENDPOINTS_QUERY = /* GraphQL */ `
  query GetWebhookEndpoints($workspaceId: ID!) {
    webhookEndpoints(workspaceId: $workspaceId) {
      id
      workspaceId
      url
      events
      active
      createdAt
      secretPreview
    }
  }
`;

/* --------------------------------------------------------------------------- */
/* Mutations                                                                      */
/* --------------------------------------------------------------------------- */

/**
 * DASH-01 — Create a new workspace during onboarding.
 * SDL: Mutation.createWorkspace(name: String!, country: String!): Workspace!
 * Closes T-008.1f.
 */
export const CREATE_WORKSPACE_MUTATION = /* GraphQL */ `
  mutation CreateWorkspace($name: String!, $country: String!) {
    createWorkspace(name: $name, country: $country) {
      id
      name
      country
      slug
      plan
      createdAt
    }
  }
`;

export const CREATE_API_KEY_MUTATION = /* GraphQL */ `
  mutation CreateApiKey($workspaceId: ID!, $name: String!) {
    createApiKey(workspaceId: $workspaceId, name: $name) {
      rawKey
      apiKey {
        id
        workspaceId
        name
        prefix
        active
        createdAt
        expiresAt
      }
    }
  }
`;

export const REVOKE_API_KEY_MUTATION = /* GraphQL */ `
  mutation RevokeApiKey($workspaceId: ID!, $keyId: ID!) {
    revokeApiKey(workspaceId: $workspaceId, keyId: $keyId)
  }
`;

/**
 * DASH-02 — Atomic API key rotation (replaces revoke+create two-step).
 * SDL: Mutation.rotateApiKey(workspaceId: ID!, keyId: ID!): CreateApiKeyResult!
 * Closes D-E03 / T-008.1a.
 */
export const ROTATE_API_KEY_MUTATION = /* GraphQL */ `
  mutation RotateApiKey($workspaceId: ID!, $keyId: ID!) {
    rotateApiKey(workspaceId: $workspaceId, keyId: $keyId) {
      rawKey
      apiKey {
        id
        workspaceId
        name
        prefix
        active
        createdAt
        expiresAt
      }
    }
  }
`;

export const CREATE_WEBHOOK_ENDPOINT_MUTATION = /* GraphQL */ `
  mutation CreateWebhookEndpoint($workspaceId: ID!, $url: String!, $events: [String!]!) {
    createWebhookEndpoint(workspaceId: $workspaceId, url: $url, events: $events) {
      id
      workspaceId
      url
      events
      active
      createdAt
      secretPreview
    }
  }
`;

export const DELETE_WEBHOOK_ENDPOINT_MUTATION = /* GraphQL */ `
  mutation DeleteWebhookEndpoint($workspaceId: ID!, $id: ID!) {
    deleteWebhookEndpoint(workspaceId: $workspaceId, id: $id)
  }
`;
