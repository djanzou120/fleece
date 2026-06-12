// Couche 4 — Configuration du framework Better Auth + Drizzle (schéma "identity").
//
// L'initialisation réelle (betterAuth({...}) avec l'adapter Drizzle) est branchée
// ici, au plus loin du domaine. Atlas reste propriétaire des migrations :
// Drizzle n'exécute pas les siennes (voir .ia/ARCHITECTURE.md §6.4).

export const identitySchema = "identity";

export const betterAuthConfig = {
  // database: drizzleAdapter(db, { provider: "pg", schema: identitySchema }),
  // emailAndPassword: { enabled: true },
  // session: { ... },
} as const;
