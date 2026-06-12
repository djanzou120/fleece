import Joi from "joi"

export const schema = {
  email: Joi.string().email().required(),
  password_min: Joi.string().required(),
  password: Joi.string()
    .required()
    .min(8)
    .pattern(new RegExp(/^(((?=.*[a-z])(?=.*[A-Z]))|((?=.*[a-z])(?=.*[0-9]))|((?=.*[A-Z])(?=.*[0-9])))(?=.{6,})/)),
  firstname: Joi.string().required().min(2),
  firstnameMin: Joi.string().min(2),
  lastname: Joi.string().min(2),
  phone: Joi.string().required().min(9),
  phone_min: Joi.string().min(9),
  code: Joi.number().required().min(100000).max(999999),
  id: Joi.number().required(),
  businessName: Joi.string(),
  roleName: Joi.string().required(),
  postContent: Joi.string().required().min(3),
  publish: Joi.boolean().default(true),
  amount: Joi.number(),
  q: Joi.string().required().min(3).max(120),
  countryAbbreviation: Joi.string().required().min(2).max(7),
  countryCode: Joi.string().required().min(2).max(7),
  promoCode: Joi.string().allow("").optional(),
  title: Joi.string(),
  content: Joi.string(),
  images: Joi.array(),
}
