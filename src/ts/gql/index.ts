export * from "./gql_gen"

export enum ErrorCode {
  AccountNotVerified = "account_not_verified",
  AlreadyUsedCredentials = "already_used_credentials",
  AuthenticationError = "authentication_error",
  BlockedAccount = "blocked_account",
  ExpiredAuthorization = "expired_authorization",
  ExpiredOtp = "expired_otp",
  InternalServerError = "internal_server_error",
  NotFound = "not_found",
  TemporaryBlockedAccount = "temporary_blocked_account",
  UnknownEmail = "unknown_email",
  ValidationError = "validation_error",
  WrongCredentials = "wrong_credentials",
  WrongOtp = "wrong_otp",
}
