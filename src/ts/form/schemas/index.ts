export * from "./default"

import Joi from "joi"

import { schema } from "./default"

export const ForgotPasswordForm = Joi.object({
  email: schema.email,
})

export const LoginForm = Joi.object({
  email: schema.email,
  password: schema.password_min,
})

export const UpdateProfileForm = Joi.object({
  firstname: schema.firstnameMin,
  lastname: schema.lastname,
  dfa: Joi.bool(),
})

export const UpdateEmailForm = Joi.object({
  newEmail: schema.email,
  otp: schema.code,
})

export const UpdatePhoneForm = Joi.object({
  phone: schema.phone,
  otp: schema.code,
})

export const UpdatePasswordForm = Joi.object({
  newPassword: schema.password,
  otp: schema.code,
})

export const CreatePostForm = Joi.object({
  content: schema.postContent,
  publish: schema.publish,
  amount: schema.amount,
  images: schema.images,
})

export const RegisterForm = Joi.object({
  firstname: schema.firstname,
  lastname: schema.lastname,
  email: schema.email,
  phone: schema.phone_min,
  password: schema.password,
  countryAbbreviation: schema.countryAbbreviation,
  countryCode: schema.countryCode,
  promoCode: schema.promoCode,
})

export const VerifyEmailForm = Joi.object({
  email: schema.email,
  otp: schema.code,
})

export const ResetPasswordForm = Joi.object({
  otp: schema.code,
  email: schema.email,
  password: schema.password_min,
})

export const SearchForm = Joi.object({
  q: schema.q,
})
