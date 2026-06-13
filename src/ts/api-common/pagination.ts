// Types de pagination partagés (cursor-based) utilisés par les deux gateways
// pour normaliser les listes paginées renvoyées par les services Go.

export interface PageArgs {
  cursor?: string;
  limit?: number;
}

export interface PageInfo {
  nextCursor: string | null;
  hasNextPage: boolean;
}

export interface Page<T> {
  items: T[];
  pageInfo: PageInfo;
}
