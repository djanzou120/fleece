// Couche 4 — Serveur HTTP et composition root du gateway REST public.
// Responsabilités : démarrer le serveur, monter le middleware d'authentification
// (API Key via @fleece/api-common), attacher les routes (adapters/http/).

export function startServer(): void {
  // TODO: initialiser le serveur HTTP (Hono / Express),
  //       monter apiKeyMiddleware (partagé depuis @fleece/api-common),
  //       enregistrer les routers de adapters/http/,
  //       instancier les clients (adapters/clients/) et les injecter,
  //       écouter sur process.env.PORT.
  throw new Error("Not implemented");
}
