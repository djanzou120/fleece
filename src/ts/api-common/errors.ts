// Codes et types d'erreur partagés entre les deux gateways.
// Utilisés pour produire des réponses d'erreur cohérentes (REST JSON ou GraphQL extensions).

export type ApiErrorCode =
  | "UNAUTHORIZED"
  | "FORBIDDEN"
  | "NOT_FOUND"
  | "VALIDATION_ERROR"
  | "RATE_LIMITED"
  | "INSUFFICIENT_FUNDS"
  | "INTERNAL_ERROR";

export class ApiError extends Error {
  constructor(
    public readonly code: ApiErrorCode,
    message: string,
    public readonly statusHint: number = 400
  ) {
    super(message);
    this.name = "ApiError";
  }
}
