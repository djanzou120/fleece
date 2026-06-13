// Ports de sortie du REST Gateway : interfaces vers les services internes (Go).
// Les implémentations concrètes sont dans adapters/clients/.

import type { ApiContext } from "@fleece/api-common";

export interface MessagingClient {
  sendMessage(ctx: ApiContext, payload: unknown): Promise<unknown>;
}

export interface WalletClient {
  getBalance(ctx: ApiContext): Promise<unknown>;
  topUp(ctx: ApiContext, payload: unknown): Promise<unknown>;
}

export interface WebhookClient {
  listEndpoints(ctx: ApiContext): Promise<unknown>;
  createEndpoint(ctx: ApiContext, payload: unknown): Promise<unknown>;
}
