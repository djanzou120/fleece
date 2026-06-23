# Fleece — Design System & UI Reference

> **Source :** claude.ai/design projectId `97212a25-4baa-4c9b-99fc-862c182c8712`
> Fichiers HTML dans `.ia/design/` — ouvrir dans un navigateur ou via le MCP DesignSync pour une preview live.
> Pour rafraîchir : `DesignSync.get_file(projectId, "Fleece Dashboard.dc.html")`

---

## Philosophie

**Sombre · dense · technique** — pensé pour les développeurs.
Monochrome neutre, **un seul vert d'accent** pour toutes les actions, Geist Mono pour la donnée/code.
Sobre par défaut, lisible à haute densité d'information.

---

## Tokens CSS (thème sombre — défaut)

```css
/* Surfaces */
--bg:         #0a0a0a   /* fond global */
--bg-elev:    #0f0f0f   /* fond élevé (sidebar, header) */
--surface:    #141414   /* cartes, inputs */
--surface-2:  #1a1a1a   /* hover surface */

/* Bordures */
--border:     #242424
--border-soft:#1b1b1b

/* Texte */
--text:       #fafafa   /* primaire */
--text-2:     #9b9b9b   /* secondaire / labels */
--text-3:     #646464   /* tertiaire / placeholders */

/* Accent unique (vert) */
--accent:      #27cf7d
--accent-2:    #1fa863  /* hover */
--accent-soft: rgba(39,207,125,0.12)
--accent-text: #3ddb8c

/* États */
--danger:     #f06363   /* échec */
--danger-soft:rgba(240,99,99,0.12)
--warn:       #e3a93a   /* en file / en attente */
--warn-soft:  rgba(227,169,58,0.12)
--info:       #5e9bf0   /* envoyé / info */
--info-soft:  rgba(94,155,240,0.12)

--shadow: 0 1px 2px rgba(0,0,0,0.4)
```

**Thème clair disponible** (`data-theme="light"`) — fond `#f6f5f2`, accent `#0f9d5f`.

---

## Typographie

| Usage | Police | Taille | Poids |
|-------|--------|--------|-------|
| Display | Geist | 34–38px | 600 |
| H1 | Geist | 24–30px | 600 |
| H2 | Geist | 18px | 600 |
| Body | Geist | 14px | 400 |
| Caption | Geist | 12px | 400 |
| Donnée / Code | **Geist Mono** | 11–13px | 400–600 |

- `letter-spacing: -0.02em` sur les titres
- `font-variant-numeric: tabular-nums` sur tous les chiffres
- `-webkit-font-smoothing: antialiased` global

---

## Rayons (border-radius)

`6px` · `9px` · `12px` · `14px` (default cartes) · `24px` (pill/badge)

---

## Couleurs par canal

| Canal | Couleur | Hex |
|-------|---------|-----|
| WhatsApp | vert accent | `#27cf7d` |
| SMS | bleu info | `#5e9bf0` |
| Telegram | bleu Telegram | `#3aa6e0` |

---

## Composants

### Boutons
```
Primaire   : bg=--accent, color=#06210f, h=34–46px, br=8–11px, fw=600
Secondaire : border=--border, bg=--surface, color=--text
Ghost      : bg=none, color=--text-2
Danger     : border=--danger, bg=--danger-soft, color=--danger
```

### Statuts (badges pill)
```
Délivré   : color=#3ddb8c  bg=rgba(39,207,125,0.12)
Envoyé    : color=#5e9bf0  bg=rgba(94,155,240,0.12)
En file   : color=#e3a93a  bg=rgba(227,169,58,0.12)
Échec     : color=#f06363  bg=rgba(240,99,99,0.12)
```
Format : dot (6px) + label, `border-radius:20px`, `padding:3px 10px`.

### Inputs / Search
```
height:46px, br=11px, border=--border, bg=--bg-elev
focus: border-color=--text-3, bg=--surface-2
Search bar : bg=--bg-elev, icon ⌕, kbd ⌘K
```

### Cards KPI
```
bg=--bg-elev, border=--border, br=13px, p=15px
Valeur : 26px/600, tabular-nums
Badge delta : color=--accent-text, bg=--accent-soft, mono, br=20px
Sparkline SVG avec gradient fill accent
```

### Bloc de code
```
bg=--bg-elev, border=--border, br=11px, p=14px
Font: Geist Mono 12px, lh=1.7, color=--text-2
Tokens couleur : POST/verbes=--info  clés=--warn  valeurs=--accent-text
Tabs de langage : cURL · Node.js · Python · Go
```

### Sidebar
```
width: 230px, bg=--bg-elev, border-right=--border
Item actif : bg=--accent-soft, color=--accent-text
Item inactif : color=--text-3
Avatar : gradient #27cf7d→#1b7e9c, br=50%
Logo pill : w=28px, br=8px, bg=--accent, color=#06210f
```

### Header / Top bar
```
height: 56–60px, sticky, backdrop-blur(10px)
bg: color-mix(--bg 85%, transparent)
Breadcrumb : Geist Mono 13px, color=--text-3 / --text
```

### Segmented control
```
bg=--bg-elev, border=--border, br=9px, p=3px
Tab actif : bg=--accent, color=#06210f
Tab inactif : color=--text-2
```

---

## Layouts

### Dashboard (Fleece Dashboard.dc.html)
```
┌─ Sidebar 230px ─┬──────────── Main ─────────────────┐
│ Logo + nav      │ Header (breadcrumb + actions)      │
│ items           │ Content : KPI row + charts + table │
│ user footer     │ (padding: 24–32px)                 │
└─────────────────┴────────────────────────────────────┘
```
Vues : Overview · Messages · Logs · API Keys · Wallet · Webhooks · Settings

### Auth (Fleece Auth & Onboarding.dc.html)
```
Vues : signup → check-email → confirm → onboarding (4 steps)
       login (retour direct vers onboarding)
Onboarding steps : 1.Workspace 2.API Key 3.Wallet top-up 4.Send message
Card centré max-width:392px, radial-gradient bg
```

### Animations
```css
@keyframes flc-fade  { from { opacity:0; transform:translateY(6–8px) } to { opacity:1; transform:none } }
@keyframes flc-toast { from { opacity:0; transform:translate(16px,0) } to { opacity:1; transform:none } }
Durée standard : .3–.35s ease
```

---

## Personas de référence (design)
- **Awa Diop** — développeuse, awa.diop@example.com, workspace Acme Corp, région Afrique de l'Ouest + Europe

---

## Accès aux fichiers source

| Fichier | Rôle | Chemin local |
|---------|------|-------------|
| `Fleece Dashboard.dc.html` | Dashboard principal | `.ia/design/Fleece Dashboard.dc.html` |
| `Fleece Auth & Onboarding.dc.html` | Auth + onboarding 4 étapes | `.ia/design/Fleece Auth & Onboarding.dc.html` |
| `Fleece Design System.dc.html` | Design system interactif | `.ia/design/Fleece Design System.dc.html` |

Pour rafraîchir depuis la source :
```
DesignSync.get_file(projectId="97212a25-4baa-4c9b-99fc-862c182c8712", path="<nom>")
```
