// Package golog fournit un logger structuré partagé entre les services Go.
// Il enveloppe [log/slog] de la bibliothèque standard et ne dépend d'aucun
// paquet externe — seule la stdlib est importée.
// Cette lib est transverse : elle ne contient AUCUNE règle métier.
package golog

import (
	"context"
	"log/slog"
	"os"
)

// ContextKey est la clé utilisée pour stocker des attributs de log dans un
// [context.Context]. Valeur attendue : []any (paires clé/valeur slog).
const ContextKey = "log.fields"

// Logger enveloppe [*slog.Logger] pour offrir une API légèrement plus riche
// (WithContext) et permettre d'étendre le type sans casser les appelants.
type Logger struct {
	*slog.Logger
}

// Init construit un Logger selon le niveau et le format demandés.
//
// level accepte "debug", "info", "warn" ou "error" (insensible à la casse).
// Toute valeur inconnue est remplacée par [slog.LevelInfo].
//
// format accepte "json" (handler JSON) ou "text" (handler texte lisible).
// Toute valeur inconnue produit un handler texte.
//
// Les deux handlers écrivent sur [os.Stderr].
func Init(level, format string) *Logger {
	var lvl slog.Level
	switch level {
	case "debug":
		lvl = slog.LevelDebug
	case "warn":
		lvl = slog.LevelWarn
	case "error":
		lvl = slog.LevelError
	default:
		lvl = slog.LevelInfo
	}

	opts := &slog.HandlerOptions{Level: lvl}

	var handler slog.Handler
	if format == "json" {
		handler = slog.NewJSONHandler(os.Stderr, opts)
	} else {
		handler = slog.NewTextHandler(os.Stderr, opts)
	}

	return &Logger{Logger: slog.New(handler)}
}

// With retourne un nouveau Logger enrichi des attributs supplémentaires fournis.
// Il enveloppe [slog.Logger.With] en conservant le type *Logger.
func (l *Logger) With(args ...any) *Logger {
	return &Logger{Logger: l.Logger.With(args...)}
}

// Info enregistre un message au niveau [slog.LevelInfo].
func (l *Logger) Info(msg string, args ...any) {
	l.Logger.Info(msg, args...)
}

// Warn enregistre un message au niveau [slog.LevelWarn].
func (l *Logger) Warn(msg string, args ...any) {
	l.Logger.Warn(msg, args...)
}

// Error enregistre un message au niveau [slog.LevelError].
func (l *Logger) Error(msg string, args ...any) {
	l.Logger.Error(msg, args...)
}

// Debug enregistre un message au niveau [slog.LevelDebug].
func (l *Logger) Debug(msg string, args ...any) {
	l.Logger.Debug(msg, args...)
}

// WithContext retourne un nouveau Logger enrichi des attributs stockés dans le
// contexte sous la clé [ContextKey]. Si le contexte ne contient pas de champs
// ou que la valeur n'est pas du bon type, le Logger original est retourné
// sans modification.
func (l *Logger) WithContext(ctx context.Context) *Logger {
	attrs, _ := ctx.Value(ContextKey).([]any)
	return l.With(attrs...)
}
