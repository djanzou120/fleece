// Package messaging est intentionnellement vide pour le service routing.
//
// Le service routing ne publie pas d'evenements asynchrones (pas de broker RabbitMQ).
// Les echanges inter-services sont geres via REST interne (adapters/clients).
// Ce fichier est conserve uniquement pour maintenir la coherence de la structure
// de packages entre les services Go du monorepo.
package messaging
