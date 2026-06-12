// Configuration Atlas — migrations de la base unifiée Fleece.
// Voir .ia/ARCHITECTURE.md §2.1 (base unifiée) et §6.4 (migrations).

env "local" {
  url = getenv("DATABASE_URL")
  dev = "docker://postgres/16/dev"

  migration {
    dir    = "file://migrations"
    format = atlas
  }
}
