import * as graphql from 'graphql'


export type Any = any
export type Boolean = boolean
export type Email = string
export type Float = number
export type ID = string
export type Int = number
export type SignedURL = string
export type StorageKey = string
export type String = string
export type Upload = any

export type ForgotPasswordInput = {
  email: Email
}

export type LoginInput = {
  email: Email
  password: String
}

export type MutationForgotPasswordArgs = {
  input: ForgotPasswordInput
}

export type MutationLoginArgs = {
  input: LoginInput
}

export type MutationRegisterArgs = {
  input: RegisterInput
}

export type MutationResetPasswordArgs = {
  input: ResetPasswordInput
}

export type MutationVerifyEmailArgs = {
  input: VerifyEmailInput
}

export type RegisterInput = {
  firstname: String
  lastname?: String
  email: Email
  phone: String
  password: String
  countryCode: String
  countryAbbreviation: String
  promoCode?: String
}

export type ResetPasswordInput = {
  email: Email
  otp: Int
  password: String
}

export type VerifyEmailInput = {
  email: Email
  otp: Int
}

export type LoginOutput = {
  accessToken: String
  refreshToken: String
  type: String
}

export type ResetPasswordOutput = {
  accessToken: String
  refreshToken: String
  type: String
}

export type User = {
  createdAt: Date
  email: String
  firstname: String
  id: String
  lastname?: String
  phone: String
  updatedAt: Date
}

export type VerifyEmailOutput = {
  accessToken: String
  refreshToken: String
  type: String
}



export type MutationResolver = {

  forgotPassword(parent: unknown, args: MutationForgotPasswordArgs, context: any): Promise<Boolean>
  login(parent: unknown, args: MutationLoginArgs, context: any): Promise<LoginOutput>
  register(parent: unknown, args: MutationRegisterArgs, context: any): Promise<Boolean>
  resetPassword(parent: unknown, args: MutationResetPasswordArgs, context: any): Promise<ResetPasswordOutput>
  verifyEmail(parent: unknown, args: MutationVerifyEmailArgs, context: any): Promise<VerifyEmailOutput>

}

export type QueryResolver = {

  me(parent: unknown, args: unknown, context: any): Promise<User>

}

export type SubscriptionResolver = {

  me: {
    subscribe(parent: unknown, args: unknown, context: any): AsyncIterator<any>
    resolve(payload: any): Promise<User | null | undefined>
  }

}


export interface Resolvers {
  Mutation: MutationResolver
  Query: QueryResolver
  Subscription: SubscriptionResolver
  Email: graphql.GraphQLScalarType
  ID: graphql.GraphQLScalarType
}
