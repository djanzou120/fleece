export * from "./schemas"

import Joi from "joi"

export interface FieldError {
  field?: string
  message: string
}

export class ValidateError extends Error {
  fields: FieldError[]

  constructor(fields: FieldError[], message: string) {
    super(message)
    this.fields = fields
  }
}

export interface ValidatorFn {
  <T>(schema: Joi.ObjectSchema, fields: T): void
}

export const validator: ValidatorFn = <T>(schema: Joi.ObjectSchema, fields: T): void => {
  const validation = schema.validate(fields, { abortEarly: false })

  let errors: FieldError[] = []
  if (validation.error) {
    for (const field of validation.error.details) {
      console.log(">>>>>> Field : ", field)
      if (field.context == undefined) {
        continue
      }
      errors.push({
        field: field.context.key,
        message: field.message,
      })
    }
    throw new ValidateError(errors, "Validation error")
  }
  return
}
