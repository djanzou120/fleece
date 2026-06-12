import { Resend } from "resend"

export enum TemplateID {
  Register = "register",
  ForgotPassword = "forgot_password",
}

type RegisterParams = {
  name: string
  otp: number
}

type ForgotPasswordParams = {
  name: string
  otp: number
}

type Template =
  | {
      id: TemplateID.Register
      params: RegisterParams
    }
  | {
      id: TemplateID.ForgotPassword
      params: ForgotPasswordParams
    }

export class Mail {
  private instance: Mail | null = null
  private key: string
  private fromEmail: string
  private resend: Resend

  constructor(key: string, fromEmail: string) {
    this.key = key
    this.fromEmail = fromEmail
    this.resend = new Resend(key)
  }

  public getInstance(): Mail {
    if (this.instance == null) {
      this.instance = new Mail(this.key, this.fromEmail)
    }
    return this.instance
  }

  public send(to: string[], template: Template): Promise<any> {
    return this.resend.emails.send({
      from: this.fromEmail,
      to: to,
      replyTo: this.fromEmail,
      subject: subjectByTemplateID[template.id],
      html: baseEmail(contentByTemplate(template)),
    })
  }
}

const subjectByTemplateID: Record<TemplateID, string> = {
  register: "Bienvenu(e) sur Win Market",
  forgot_password: "Mot de passe oublié",
}

const contentByTemplate = (template: Template): string => {
  switch (template.id) {
    case TemplateID.Register:
      return `<p>Votre code est <span class="otp-code">${template.params.otp}</span>. Veuillez entrer ce code pour completé la vérification.</p>`
    case TemplateID.ForgotPassword:
      return `<p>Votre code est <span class="otp-code">${template.params.otp}</span>. Veuillez entrer ce code pour completé la vérification.</p>`
    default:
      throw new Error("unknown template ID")
  }
}

const baseEmail = (body: string): string => `
  <!DOCTYPE html>
  <html lang="en">
  <head>
      <meta charset="UTF-8">
      <meta name="viewport" content="width=device-width, initial-scale=1.0">
      <title>Mobile OTP Verification</title>
      <style>
          body {
              font-family: Arial, sans-serif;
              margin: 0;
              padding: 20px;
              background-color: #f4f4f4;
          }
          .container {
              max-width: 600px;
              margin: 0 auto;
              background-color: #ffffff;
              border-radius: 8px;
              box-shadow: 0 0 10px rgba(0, 0, 0, 0.1);
              overflow: hidden;
          }
          .header {
              background-color: #007bff;
              color: #ffffff;
              padding: 20px;
              text-align: center;
          }
          .content {
              padding: 20px;
              text-align: left;
          }
          .otp-code {
              font-size: 24px;
              font-weight: bold;
              color: #333333;
          }
          .footer {
              text-align: center;
              padding: 20px;
              font-size: 12px;
              color: #777777;
          }
          @media (max-width: 600px) {
              .container {
                  width: 100%;
                  box-shadow: none;
              }
          }
      </style>
  </head>
  <body>
      <div class="container">
          <div class="header">
              <h1>Verification d'email</h1>
          </div>
          <div class="content">
              ${body}
              <p>Thank you!</p>
          </div>
          <div class="footer">
              <p>&copy; 2023 Your Company. All rights reserved.</p>
          </div>
      </div>
  </body>
  </html>

`
