// Couche 3 — Client REST vers le service auth-api (Identity), driven.

import { IdentityClient } from "../../application/ports/output/clients";

export class IdentityRestClient implements IdentityClient {
  constructor(private readonly baseUrl: string) {}

  async getWorkspace(id: string): Promise<{ id: string; name: string } | null> {
    // TODO: appel REST interne — GET {baseUrl}/workspaces/{id}
    void this.baseUrl;
    void id;
    return null;
  }
}
