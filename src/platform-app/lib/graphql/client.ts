/**
 * Fleece GraphQL Client — lightweight fetch-based client.
 *
 * DEBT: No external dependency (no Apollo, no urql) — all offline-safe.
 * This is intentional: the monorepo is built offline. Replace with urql or
 * Apollo Client once npm install is available online.
 *
 * Usage:
 *   const result = await gqlClient.query<MyQuery>(MY_QUERY, { workspaceId: 'xxx' });
 *
 * Endpoint: env NEXT_PUBLIC_GRAPHQL_URL (fallback: http://localhost:4000/graphql)
 * Auth: session cookie is forwarded automatically by the browser (same-origin).
 * For server-side calls, pass the cookie manually via the `headers` option.
 */

const GRAPHQL_URL =
  process.env.NEXT_PUBLIC_GRAPHQL_URL ?? 'http://localhost:4000/graphql';

export interface GqlError {
  message: string;
  locations?: { line: number; column: number }[];
  path?: string[];
  extensions?: Record<string, unknown>;
}

export interface GqlResponse<T> {
  data?: T;
  errors?: GqlError[];
}

export class GraphQLClientError extends Error {
  constructor(
    message: string,
    public readonly errors: GqlError[],
  ) {
    super(message);
    this.name = 'GraphQLClientError';
  }
}

interface RequestOptions {
  /** Additional headers (e.g. Cookie for server-side calls). */
  headers?: Record<string, string>;
}

async function request<T>(
  query: string,
  variables?: Record<string, unknown>,
  options?: RequestOptions,
): Promise<T> {
  const res = await fetch(GRAPHQL_URL, {
    method: 'POST',
    headers: {
      'Content-Type': 'application/json',
      Accept: 'application/json',
      ...options?.headers,
    },
    credentials: 'include',
    body: JSON.stringify({ query, variables }),
  });

  if (!res.ok) {
    throw new GraphQLClientError(
      `GraphQL request failed: HTTP ${res.status} ${res.statusText}`,
      [],
    );
  }

  const json: GqlResponse<T> = await res.json();

  if (json.errors && json.errors.length > 0) {
    throw new GraphQLClientError(
      json.errors.map((e) => e.message).join('; '),
      json.errors,
    );
  }

  if (json.data === undefined) {
    throw new GraphQLClientError('GraphQL response contained no data.', []);
  }

  return json.data;
}

export const gqlClient = {
  query: <T>(
    query: string,
    variables?: Record<string, unknown>,
    options?: RequestOptions,
  ) => request<T>(query, variables, options),

  mutate: <T>(
    mutation: string,
    variables?: Record<string, unknown>,
    options?: RequestOptions,
  ) => request<T>(mutation, variables, options),
};
