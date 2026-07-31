// @fleece/model — point d'entrée public.
//
// Ré-exporte les définitions Drizzle du schéma `identity`. Voir README.md pour
// la règle cardinale de ce paquet : Atlas est la source de vérité des
// migrations, Drizzle n'est qu'un query builder + un typage.

export * from "./schema/identity.js";
