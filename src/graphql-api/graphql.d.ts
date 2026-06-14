// Déclaration de module ambient pour les fichiers .graphql.
// Chargés en tant que chaîne de texte par esbuild (--loader:.graphql=text).
// Permet à tsc de ne pas broncher sur les imports de .graphql.
declare module "*.graphql" {
  const content: string;
  export default content;
}
