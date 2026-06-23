/**
 * GraphQL query/mutation documents.
 * Kept as plain strings — no gql tag dependency (offline).
 * DEBT: Replace with typed documents from @graphql-codegen once online.
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

export const WEBHOOK_ENDPOINTS_QUERY = /* GraphQL */ `
  query GetWebhookEndpoints($workspaceId: ID!) {
    webhookEndpoints(workspaceId: $workspaceId) {
      id
      workspaceId
      url
      events
      active
      createdAt
    }
  }
`;

/* --------------------------------------------------------------------------- */
/* Mutations                                                                      */
/* --------------------------------------------------------------------------- */

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

export const CREATE_WEBHOOK_ENDPOINT_MUTATION = /* GraphQL */ `
  mutation CreateWebhookEndpoint($workspaceId: ID!, $url: String!, $events: [String!]!) {
    createWebhookEndpoint(workspaceId: $workspaceId, url: $url, events: $events) {
      id
      workspaceId
      url
      events
      active
      createdAt
    }
  }
`;

export const DELETE_WEBHOOK_ENDPOINT_MUTATION = /* GraphQL */ `
  mutation DeleteWebhookEndpoint($workspaceId: ID!, $id: ID!) {
    deleteWebhookEndpoint(workspaceId: $workspaceId, id: $id)
  }
`;
