# ModuleForge style-package contract

A **style package** is the versioned artifact that supplies a brand's look — colors,
typography, radius, and brand assets — to a ModuleForge application **at runtime**, without
rebuilding the app or `mod-core`. This document is the fixed contract for that artifact type:
what a style package contains, how it declares the token contract it targets, how the
loader/app-shell behaves on a version mismatch, and how a package author builds one.

It builds directly on [CONTRACT.md](./CONTRACT.md) — the token-consumption and `data-mf-theme`
scoping contract — and never contradicts it. Where CONTRACT.md defines the *consumption* side
(how components resolve `var(--mf-x, var(--mf-x-default))` and how mod-core bakes the
`--mf-x-default` twins), this document defines the *supply* side (how an independently-published
package sets the `--mf-x` levers). Read CONTRACT.md first.

Two artifacts are built against this contract and must follow it exactly:

- the **Liquid Labs style package** (Phase 4) — the first real brand package, and the reference
  implementation of the layout/build convention below;
- the **runtime loader** (Phase 3 task `002`) — the `@moduleforge/core-gui` code that injects a
  style package's bundle and reads its manifest.

This is a specification. It defines conventions and a build template; it does not ship a style
package or a loader.

## The style-package artifact — exactly three parts

A style package supplies **exactly three** things, and nothing else. It is never an arbitrary
stylesheet or an arbitrary bundle of code (see [Security posture](#security-posture--typed-values-manifest-never-arbitrary-css)).

### 1. Compiled `--mf-*` override bundle

A single compiled CSS file — a **sparse `--mf-*` diff** — that sets only the bare `--mf-x`
levers the brand changes, scoped under the unified `data-mf-theme` attribute from
[CONTRACT.md](./CONTRACT.md#unified-scoping--data-mf-theme).

Hard rules, all inherited from CONTRACT.md's settable-vs-internal table:

- **Sets `--mf-x` only, never `--mf-x-default`.** `--mf-x` is the single runtime-settable lever;
  `--mf-x-default` is compiler-internal. A package that sets a `-default` twin fights the compiled
  bundle and breaks mode/scope switching — it is outside the contract and its behavior is
  undefined.
- **Always partial / sparse.** A package overrides only the roles its brand changes; every omitted
  token degrades gracefully to mod-core's `--mf-x-default` via the fallback chain. A style package
  can never leave a token undefined, and never needs to enumerate the full surface.
- **No `@property`, no `@theme`, no component/selector rules.** The bundle contains only `--mf-x`
  custom-property declarations inside the fixed `data-mf-theme` scope selectors below. The
  `@property` typing and the Tailwind `@theme inline` mapping are mod-core's compiled surface; a
  package neither re-declares nor overrides them.
- **Scoped exactly like mod-core's `-default` sets.** The bundle mirrors mod-core's scope selectors
  so brand and mode compose through the one attribute (per CONTRACT.md's "runtime brand selection"
  case). See [Emission rules](#emission-rules-what-the-override-bundle-must-look-like).

**The layout-token lever family — a second axis.** Every token above this point in the contract is a
single scalar per mode/scope; the content-margin tokens (contract `1.1.0`) add a second axis —
breakpoint band — on top of the ordinary style-package-override axis:

- `--mf-content-margins-lr` / `--mf-content-margins-tb` are the **base-scale levers**. Setting
  either rescales every band through that band's compiler-baked multiplier — the same relationship
  `--mf-radius` has to the derived radius steps.
- `--mf-content-margins-{lr,tb}-{base,sm,md,lg,xl,2xl}` are the **per-band levers**. Setting one
  replaces the derived value for that band alone; the base lever still governs every other band.
  **This diverges from the radius rule**: for radius the derived `--mf-radius-{sm,md,lg,xl}` steps
  are never settable, but for content margins a per-band derived step is exactly the intended escape
  hatch (see [Emission rules](#emission-rules-what-the-override-bundle-must-look-like) for the
  explicit statement of this divergence).
- `--mf-max-content-width` is an ordinary single scalar with no second axis, exactly like every
  color/typography role.

**Band-span semantics.** A per-band override governs its band's *span* — from that breakpoint up to
the next declared band — not every width above it. For example, setting
`--mf-content-margins-lr-sm` alone changes the inline gutter for the `40rem`–`48rem` span (the `sm`
band, up to the next declared `md` band) and nowhere else; a brand wanting the change to persist
across several bands must set each of them, or set the base lever instead.

The bundle is what the loader injects as a versioned `<link rel="stylesheet">` (per the
`gui-design-tokens` plan's `runtime-theming.md`, held in the plan branch rather than shipped here).
Because it
sits *below* the `@theme inline` layer in the cascade — it sets the `--mf-x` inputs that mod-core's
`--color-*`/`--font-*`/`--radius-*` theme keys already read through the fallback chain — every
existing `cva` utility (`bg-primary`, `text-secondary-foreground`, `rounded-md`, …) picks up the
brand with zero per-component churn.

### 2. Brand-asset manifest

DTCG has no standardized asset-token type (logos, font files), and this contract does not wait on
one — the manifest is a **first-party convention**. It is a single JSON file, `style-package.json`,
that is also the loader's **entry point**: given a style package's base location, the loader fetches
`<base>/style-package.json` first, then injects the CSS bundle it names and exposes the brand assets
to the app shell.

**Schema** (JSON Schema, draft 2020-12; `formatVersion` lets the manifest format itself evolve):

```json
{
  "$schema": "https://json-schema.org/draft/2020-12/schema",
  "$id": "https://moduleforge.dev/schemas/style-package.json",
  "title": "ModuleForge style-package manifest",
  "type": "object",
  "required": ["formatVersion", "name", "version", "targetContractVersion", "styleBundle"],
  "additionalProperties": false,
  "properties": {
    "formatVersion": {
      "type": "integer",
      "const": 1,
      "description": "Manifest schema version. Bumped only if this manifest shape changes."
    },
    "name": {
      "type": "string",
      "description": "Style-package identifier, e.g. \"liquid-labs\"."
    },
    "version": {
      "type": "string",
      "pattern": "^\\d+\\.\\d+\\.\\d+",
      "description": "The style package's own semver, independent of the token contract."
    },
    "targetContractVersion": {
      "type": "string",
      "description": "semver RANGE of the mod-core token contract this package was built against, e.g. \"^1.0.0\". See Token-contract versioning."
    },
    "styleBundle": {
      "type": "string",
      "description": "URL of the compiled sparse --mf-* override bundle, relative to the manifest."
    },
    "assets": {
      "type": "object",
      "additionalProperties": false,
      "description": "Brand assets. DTCG has no asset-token type; this is a first-party convention.",
      "properties": {
        "logos": {
          "type": "object",
          "description": "Logo roles → URL (or light/dark variant map). Roles are brand-defined; \"mark\" and \"wordmark\" are the conventional two.",
          "additionalProperties": {
            "oneOf": [
              { "type": "string", "description": "A single URL (mode-independent)." },
              {
                "type": "object",
                "additionalProperties": false,
                "properties": {
                  "light": { "type": "string" },
                  "dark": { "type": "string" }
                },
                "description": "Per-mode variants, keyed to match data-mf-theme light/dark."
              }
            ]
          }
        },
        "fonts": {
          "type": "array",
          "description": "@font-face descriptors for families the override bundle points --mf-font-* at.",
          "items": {
            "type": "object",
            "required": ["family", "src"],
            "additionalProperties": false,
            "properties": {
              "family": { "type": "string" },
              "src": {
                "type": "array",
                "items": { "type": "string" },
                "description": "Font file URLs (relative to the manifest), in @font-face src order."
              },
              "weight": { "type": "string", "description": "e.g. \"400\" or \"100 900\"." },
              "style": { "type": "string", "enum": ["normal", "italic", "oblique"] },
              "display": { "type": "string", "enum": ["auto", "block", "swap", "fallback", "optional"] }
            }
          }
        }
      }
    }
  }
}
```

**How the two asset kinds connect to the token bundle:**

- **Fonts.** A brand that changes a font sets the corresponding family lever in its override bundle
  (`--mf-font-sans`, `--mf-font-mono`, or `--mf-font-heading` — the token-side of typography) *and*
  lists the `@font-face` descriptor under `assets.fonts` so the loader can register the family
  before it is referenced. The token bundle names the family; the manifest supplies the file. This
  keeps the font *value* inside the typed `--mf-*` contract while the font *file* rides the
  first-party asset channel.
- **Logos.** Logos have no `--mf-*` token role (they are content, not style), so they live only in
  the manifest. The loader hands `assets.logos` to the app shell, which passes URLs to whatever
  brand/header component renders them. Per-mode logo variants are keyed `light`/`dark` to align with
  `data-mf-theme`.

**Example manifest** (Liquid Labs, illustrative):

```json
{
  "$schema": "https://json-schema.org/draft/2020-12/schema",
  "formatVersion": 1,
  "name": "liquid-labs",
  "version": "1.0.0",
  "targetContractVersion": "^1.0.0",
  "styleBundle": "./style.css",
  "assets": {
    "logos": {
      "mark": { "light": "./logo-mark-light.svg", "dark": "./logo-mark-dark.svg" },
      "wordmark": "./logo-wordmark.svg"
    },
    "fonts": [
      { "family": "Liquid Sans", "src": ["./fonts/liquid-sans.woff2"], "weight": "100 900", "display": "swap" }
    ]
  }
}
```

### 3. Declared target version of the token contract

`targetContractVersion` (above) is a semver **range** naming the mod-core token contract the package
was compiled against. It is the single machine-readable link between a published brand and the
`--mf-*` surface it assumes. Its semantics — and the mismatch policy — are the next section.

## Token-contract versioning

The `--mf-*` surface + `data-mf-theme` scoping mechanism defined across CONTRACT.md is a **versioned
contract**, distinct from the `@moduleforge/core-gui` npm package version (the package may rev for
reasons unrelated to the token surface). The contract version is a semver owned by mod-core; its
proposed initial value is **`1.0.0`**, corresponding to the Phase 1–2 surface (the 35 color roles,
`--mf-radius`, the typography families and type scale, and the `data-mf-theme` light/dark/inverse
scoping).

**Current value: `1.1.0`.** The spacing/container token category — `--mf-max-content-width`, the two
base levers `--mf-content-margins-{lr,tb}`, and the twelve per-band levers
`--mf-content-margins-{lr,tb}-{base,sm,md,lg,xl,2xl}` — is a **MINOR** bump under the table below: new
roles only, nothing existing removed, renamed, or revalued. The authoritative constant is
`MF_TOKEN_CONTRACT_VERSION`, exported from
[`../src/lib/token-contract-version.ts`](../src/lib/token-contract-version.ts); its own doc comment
records the same `1.1.0` / MINOR characterization, and the two must be kept in agreement.

**Where it lives (for the loader to read).** The runtime loader ships in `@moduleforge/core-gui` and
runs alongside the compiled defaults, so it reads the active contract version from mod-core directly
— exposed as an exported constant (e.g. `MF_TOKEN_CONTRACT_VERSION`). Task `002` fixes the exact
export name and wires the read; this document only requires that a single such constant exist and be
the authority. (The style package, by contrast, cannot import a constant — it is loaded over HTTP —
so it carries its target range in its manifest instead.)

**Version bump semantics** — what a change to the contract version means, and why fallback chaining
makes every combination *render*:

| Change | Meaning | Old package on new core | New package on old core |
|--------|---------|-------------------------|-------------------------|
| **MAJOR** | A `--mf-x` role removed or renamed, or a `data-mf-theme` value/selector changed. | A removed/renamed `--mf-x` the package still sets is **inert** — the consuming site falls back to `--mf-x-default`. Renders, minus that override. | The package may set `--mf-x` roles the old core does not consume — **inert**, no effect. Renders. |
| **MINOR** | New `--mf-x` role(s) added; existing roles unchanged. | Package simply does not set the new roles → they use `--mf-x-default`. Renders, fully. | Package sets new roles the old core ignores → inert. Renders. |
| **PATCH** | A baked `--mf-x-default` value changed, or a build fix; no surface change. | No effect on the package's own `--mf-x` overrides. Renders. | Renders. |

This table's **MINOR** row is exactly the case governing the `1.1.0` bump above: an older package
built against `1.0.0` that sets none of the new spacing roles renders unaffected on `1.1.0` core (the
roles it doesn't set simply resolve to mod-core's `-default` twins), and a newer package that does set
them, run against a `1.0.0` core, has those roles silently ignored — inert, not broken. Both
directions hold; the table needs no change for this bump.

The through-line: **an out-of-date package always renders.** Fallback chaining
(`var(--mf-x, var(--mf-x-default))`) guarantees any missing, removed, or renamed token resolves to
mod-core's baked default rather than to the CSS guaranteed-invalid value; any *extra* token a package
sets is simply unread. There is no combination that produces an unstyled or broken page.

**Mismatch policy (loader / app-shell behavior).** Because degradation is graceful by construction,
the loader's version check is advisory, not a gate:

- **Load, always, by default.** The loader injects the bundle regardless of the version relationship;
  the render is safe either way.
- **On MAJOR mismatch** (the package's `targetContractVersion` range does not admit the runtime
  contract's major) — emit a **developer-facing warning** (console in dev; a telemetry/log hook in
  production) naming both versions, so the drift is visible and the package can be rebuilt. Do not
  block the render.
- **On MINOR/PATCH satisfied** — load silently.
- **Optional strict mode** (app-shell opt-in, off by default): refuse to load on MAJOR mismatch and
  fall through to mod-core's baked defaults only. This exists for apps that would rather show the
  unbranded-but-correct default surface than a partially-branded one; it is a policy lever, not the
  default, and task `002` decides whether to implement it now or stub it.

Task `002` owns the concrete comparison code and the warning/telemetry surface; this section fixes
the *policy* it must implement.

## Declaring the dependency — the `gui.peers` analog, and where it does not fit

The existing [`gui.peers`](../../docs/mf-standards/manifest-spec.md#gui) convention lets a module's
GUI package declare, in its own `moduleforge.module.yaml`, the peer modules' GUI packages it imports
from — an explicit, named, declarative-only dependency (no `mfgen` validation), mirroring
`migrations.after`. This contract reuses that *spirit* where it fits and flags the one direction
where it does not.

### Style package → token contract: reuse the `gui.peers` spirit

The dependency of a style package on the token contract is a genuine, named, versioned dependency —
exactly the kind `gui.peers` was built to make visible. It is declared in **two complementary
places**, each the natural home for its consumer:

1. **`peerDependencies` in the style package's `package.json`** — `@moduleforge/core-gui` at the
   compatible version range. This is the npm-native analog of a `gui.peers` entry (an explicit,
   named package + version the artifact is built against), and it is what a package author, CI, or
   `bun install` sees. It does not create a runtime import; it documents the build-time contract
   binding, just as `gui.peers` documents a GUI import binding without `mfgen` enforcing it.
2. **`targetContractVersion` in `style-package.json`** — the same binding expressed as a semver
   *range against the token contract version*, readable by the **runtime** loader (which cannot read
   `package.json` over HTTP). This is the field the [mismatch policy](#token-contract-versioning)
   acts on.

The two must agree; the [build convention](#style-package-project-layout-and-build-convention) derives
`targetContractVersion` from the `@moduleforge/core-gui` peerDependency so they cannot drift.

### App → style package: the `gui.peers` mechanism does not fit — flagged gap

`gui.peers` models a **build-time npm import** between two module GUI packages. An app's relationship
to a style package is fundamentally different: the app **loads a style package at runtime** via an
injected `<link>`, chosen by app-shell config/env — there is no npm import, no build-time edge, and
(per the plan's `runtime-theming.md`) "which style package is active is an
**app-shell** concern, not an `mfgen`/manifest-generation concern in v1." Forcing this into a
`gui.peers`-style manifest field would misrepresent a runtime configuration choice as a static import
dependency.

**Therefore this direction is deliberately *not* modeled by `gui.peers`, and there is no
manifest-declared app→style-package dependency in v1.** The minimal analogous declaration, documented
here as the intended shape rather than built now:

- **v1 (built):** the active style package is app-shell configuration — an env var or config value
  the hand-authored app shell reads and passes to the mod-core loader. No manifest field.
- **Future (flagged, not built — a convention decision for the Phase 5 standard doc):** an optional
  `moduleforge.app.yaml` `gui.style` field naming a *default* style package for an app. This would be
  the true app-side analog of `gui.peers` (an app declaring the style package it depends on), but it
  is a documentation/convention decision, not a v1 mechanism, and `mfgen` does not generate the React
  app shell that would consume it. **Gap flagged for the manager / Phase 5.**

## Security posture — typed values manifest, never arbitrary CSS

A style package may **only ever** supply a *typed values manifest* against the fixed contract — a set
of `--mf-x` declarations plus the brand-asset manifest — and **never an arbitrary stylesheet**. This
is the property that keeps the door open to less-trusted / user-authored packages later (an end-user
theme picker, a package marketplace, sandboxing — all explicitly out of scope now, per CONTRACT.md's
[Security posture](./CONTRACT.md#security-posture), but not architecturally foreclosed).

**How the typed `@property` declarations constrain override values.** Phase 1 registers every color /
dimension / number / fontWeight token as a typed CSS `@property` — on the `-default` twin, for the
reason CONTRACT.md documents (registering on the bare `--mf-x` would defeat the fallback and break
dark mode). That typed registration is nonetheless the **machine-readable type of each token in the
contract**: `--mf-primary` is a `<color>`, `--mf-radius` is a `<length>`, a weight is a `<number>`,
and so on. The type per token is what any accept/reject check — build-time today, a loader-side
validator for untrusted packages tomorrow — measures an override value against.

**Content margins under the same posture.** Every spacing token introduced with contract `1.1.0` —
`--mf-max-content-width`, both base levers, and all twelve per-band levers — is registered
`<length>`, exactly like `--mf-radius`, so the per-band levers are type-constrained on exactly the
same footing as every other token in this contract. **The new breakpoint-band axis adds surface, not
a new trust boundary**: a per-band lever is still a single typed value in a fixed, closed, enumerated
set of names, so the structural argument above (a trusted package is a values manifest *by
construction*) is unweakened — a package gains no new degree of freedom beyond "set this one more
named, typed slot." Note that `<length>` admits `clamp()` with a `vw` term (a valid `<length>` per
the CSS syntax), so a brand may supply a fluid value for a band, e.g.
`--mf-content-margins-lr-lg: clamp(1rem, 4vw, 3rem);` — that is a feature inside the typed contract,
not an escape from it.

The posture is enforced in layers, tightening as trust decreases:

- **Today (trusted packages, build-time enforcement).** The build convention below emits the override
  bundle from **DTCG token sources** through the **same Style Dictionary pipeline** mod-core uses.
  DTCG `$type` on each override is validated at compile time, and the pipeline emits **only** `--mf-x`
  declarations inside the fixed `data-mf-theme` scope selectors — it is structurally incapable of
  emitting a selector rule, an `@import`, a `url()` outside a typed token, or any other arbitrary CSS.
  A trusted package is thus a values manifest *by construction*.
- **Later (less-trusted packages, loader-side validation — not built now, not foreclosed).** For a
  package whose build pipeline is not trusted, the loader can parse the submitted bundle and re-emit
  **only** the `--mf-x` tokens it recognizes, each value type-checked against that token's `@property`
  syntax, discarding anything else. Because the contract surface is closed (a fixed token list) and
  typed (a known syntax per token), this validator is well-defined. Nothing in this contract requires
  building it now; nothing in this contract forecloses it.

Registering the typed `@property` on `-default` rather than on `--mf-x` does **not** weaken this: the
type lives with the token identity, and both the build pipeline and a future validator read it from
the contract, independent of which twin carries the runtime `@property` registration.

**Origin pinning for the manifest's own URLs.** Separately from the values-manifest posture above —
which constrains *what a package may set* — the loader also pins *where a manifest's own referenced
URLs may point*. `styleBundle` and every `assets.logos`/`assets.fonts[].src` entry are resolved
relative to the manifest's own URL (`new URL(value, manifestUrl)`), and the loader rejects a resolved
URL by default unless it shares the manifest's origin. This matters because a style package is
independently hosted (fetched over HTTP from wherever `options.baseUrl`/`source` names) — without this
check, an absolute URL in a compromised, spoofed, or simply misconfigured manifest response would
silently win over the manifest's own origin, letting the injected `<link rel="stylesheet">` (or a
logo/font asset) point at an arbitrary third-party origin. The default is **same-origin-only**; a
caller with a legitimate cross-origin hosting setup (e.g. serving the compiled bundle from a CDN
origin distinct from the manifest's) opts in explicitly via `loadStylePackage`'s
`allowCrossOriginAssets: true`, which skips the check for that call.

## Style-package project layout and build convention

This section is the concrete template Phase 4 task `001` follows to produce a style package. It is
specified tightly enough to require no further design decisions.

### Repository shape — an independent sibling repo

A style package is its **own independent git repository at the aggregate root** — e.g.
`style-liquid-labs/`, a sibling of `mod-*/` and `app-*/` — with its own history, branches, remotes,
and `package.json`. It has **no** submodule or subtree relationship to `mod-core` or the aggregate
(see the aggregate `AGENTS.md`, "Git repository boundaries"). This mirrors the established
`mod-*`/`app-*` independent-repo pattern; the build/layout below assumes this shape, **not** a
subdirectory of an existing module.

The naming convention is `style-<brand>/` (e.g. `style-liquid-labs/`).

### Directory layout

```
style-liquid-labs/
  package.json                 name @moduleforge/style-liquid-labs; peerDependencies:
                               { "@moduleforge/core-gui": "^1.0.0" }; a "build" script.
  tokens/
    overrides/
      color.light.json         SPARSE DTCG --mf-* color overrides, LIGHT set (only changed roles)
      color.dark.json          SPARSE DTCG --mf-* color overrides, DARK set (only changed roles)
      radius.json              OPTIONAL sparse --mf-radius override (mode-independent)
      typography.json          OPTIONAL sparse --mf-font-* / --mf-text-* overrides (mode-independent)
      layout.json              OPTIONAL sparse --mf-max-content-width / --mf-content-margins-*
                               overrides (mode-independent)
  assets/
    logo-mark-light.svg, logo-mark-dark.svg, logo-wordmark.svg
    fonts/liquid-sans.woff2
  style-dictionary/
    build-style.mjs            emits the sparse override bundle + copies/derives style-package.json
  dist/
    style.css                  compiled sparse --mf-* override bundle (the loader's <link> target)
    style-package.json         the brand-asset manifest (the loader's entry point)
    assets/…                   assets, copied/hashed for publishing
```

The DTCG override sources use the **same `mf.*` token paths** as mod-core's
`tokens/semantic/color.{light,dark}.json` and `tokens/typography/*`, but list **only the roles the
brand changes** — the sparseness is intrinsic to authoring against these paths, not a post-build
filter.

### Build pipeline — the same Style Dictionary, different emitter

The package builds its `dist/style.css` with **Style Dictionary** (the same dependency and
resolution mod-core's `build-tokens.mjs` uses — parse the DTCG override sources, resolve any DTCG
aliases into the brand's own base values), then a thin emitter renders the resolved tokens as bare
`--mf-x` declarations. It reuses mod-core's build *shape*; it does **not** reuse mod-core's emitter
(which bakes `--mf-x-default`, `@property`, and `@theme` — none of which a package emits).

The emitter differs from `build-tokens.mjs` in exactly three ways, and is otherwise parallel:

1. it renders `--mf-x: <value>;` (the runtime lever) instead of `--mf-x-default: <value>;`;
2. it emits **no** `@property` block and **no** `@theme inline` block;
3. it includes only the brand's overridden tokens, not the full surface.

Everything else — DTCG parsing, alias resolution, deterministic sort-by-name, the `data-mf-theme`
scope-selector structure — matches mod-core, which is what keeps brand and mode composing correctly.

### Emission rules — what the override bundle must look like

The bundle uses the **same scope selectors** as mod-core's compiled `tokens/dist/tokens.css`, so a
brand override lands in the same cascade slot as the default it replaces:

- **Color overrides are per-mode**, emitted in the scoped selectors (mirroring CONTRACT.md's "only
  color roles are re-emitted in scoped selectors"):
  - `:root` — the brand's **light** color overrides (light is the baseline, active when
    `data-mf-theme` is absent);
  - `[data-mf-theme="light"]` — the same light color overrides (re-asserted for a light island
    nested in a dark region);
  - `[data-mf-theme="dark"], .dark` — the brand's **dark** color overrides (the legacy `.dark` class
    is bridged, exactly as mod-core does);
  - `[data-mf-theme="inverse"]` — the brand's dark color overrides (inverse in a light context);
  - `[data-mf-theme="dark"] [data-mf-theme="inverse"], .dark [data-mf-theme="inverse"]` — the brand's
    light color overrides (inverse nested in dark; wins on specificity).

  A brand that overrides only light, or only some roles, emits only the selectors/tokens it actually
  sets — the rest fall through to mod-core defaults. The inverse selectors reuse the brand's
  dark/light color sets the same way mod-core's do; a brand needs no special inverse authoring unless
  it deliberately wants distinct inverse values.
- **Mode-independent overrides live once in `:root`.** Radius (`--mf-radius` — the *single* settable
  radius lever; never the derived `--mf-radius-{sm,md,lg,xl}` steps, per CONTRACT.md), font families
  (`--mf-font-sans` / `-mono` / `-heading`), type-scale sub-tokens (`--mf-text-<level>-<axis>`), and
  the layout family — `--mf-max-content-width`, both base levers (`--mf-content-margins-lr`,
  `--mf-content-margins-tb`), and every per-band lever
  (`--mf-content-margins-{lr,tb}-{base,sm,md,lg,xl,2xl}`) — are mode-independent and are **not**
  re-emitted in the `data-mf-theme` scope selectors.

  **Content margins diverge from the radius rule stated above, in exactly one respect: for content
  margins, the derived per-band forms (`--mf-content-margins-{lr,tb}-{base,sm,md,lg,xl,2xl}`) *are*
  settable — that is the intended per-band escape hatch — where the equivalent derived radius steps
  (`--mf-radius-{sm,md,lg,xl}`) are never settable.** Do not generalize the radius "never the derived
  steps" rule to content margins.

Illustrative fragment of a compiled `dist/style.css`:

```css
/* AUTO-GENERATED style-package override bundle — sparse --mf-* diff. Do not edit by hand.
 * Sets the runtime --mf-x levers only; scoped on data-mf-theme to compose with mode.
 * Built against ModuleForge token contract ^1.0.0. See mod-core gui/tokens/STYLE-PACKAGE-CONTRACT.md. */

:root {
  --mf-primary: oklch(0.55 0.22 265);       /* brand primary (light) */
  --mf-brand-highlight: oklch(0.72 0.18 190);
  --mf-radius: 0.5rem;                        /* mode-independent: once here only */
  --mf-font-sans: "Liquid Sans", ui-sans-serif, system-ui, sans-serif;
  --mf-content-margins-lr: 1.25rem;           /* base lever: rescales every band */
  --mf-content-margins-lr-lg: 3rem;           /* per-band lever: lg band only */
}

[data-mf-theme="light"] {
  --mf-primary: oklch(0.55 0.22 265);
  --mf-brand-highlight: oklch(0.72 0.18 190);
}

[data-mf-theme="dark"],
.dark {
  --mf-primary: oklch(0.68 0.20 265);
  --mf-brand-highlight: oklch(0.78 0.16 190);
}

[data-mf-theme="inverse"] {
  --mf-primary: oklch(0.68 0.20 265);
  --mf-brand-highlight: oklch(0.78 0.16 190);
}

[data-mf-theme="dark"] [data-mf-theme="inverse"],
.dark [data-mf-theme="inverse"] {
  --mf-primary: oklch(0.55 0.22 265);
  --mf-brand-highlight: oklch(0.72 0.18 190);
}
```

Note there is no `@property`, no `@theme inline`, and no `--mf-x-default` anywhere — those are
mod-core's compiled surface. The bundle sets only `--mf-x`, and only for the roles this brand
changes.

### Distribution note (carried from planning)

The compiled **default** token bundle folds into the already-distributed `@moduleforge/core-gui`; the
style package's override bundle is a genuinely separate distributable artifact (that is the point of
runtime swappability). Its CI/publishing story depends on the deployment-track
`bun install --frozen-lockfile` yalc-resolution gap (followups `9gJq`/`fXLI`) and should be validated
via Ladle + native yalc dev, not blocked on the CI fix — see the plan-branch
`scope-and-plan-split.md` (held in the plan branch, not shipped here). This is a distribution
observation for Phase 4, not a change to the contract above.

## Related documents

- [CONTRACT.md](./CONTRACT.md) — the token-consumption (fallback-chaining) and `data-mf-theme`
  scoping contract this document supplies against. The authority for the `--mf-*` surface, the
  settable-vs-internal rule, the scope selectors, and the `@property` security posture.
- [README.md](./README.md) — the DTCG token *sources* and tiering convention a style package's
  override sources mirror.
- [`../style-dictionary/build-tokens.mjs`](../style-dictionary/build-tokens.mjs) — mod-core's token
  compiler; the reference the style-package build pipeline parallels (with the three emitter
  differences noted above).
- [`../../docs/mf-standards/manifest-spec.md#gui`](../../docs/mf-standards/manifest-spec.md#gui) and
  the GUI rule in [`../../docs/mf-standards/architecture.md`](../../docs/mf-standards/architecture.md)
  — the `gui.peers` convention this contract reuses (style-package → contract direction) and
  deliberately does not force-fit (app → style-package direction).
- Plan-branch notes (held in the `gui-design-tokens` plan branch, not shipped here):
  `runtime-theming.md` (artifact definition, runtime loading model, `gui.peers` reuse, security
  posture, out-of-scope boundaries) and `scope-and-plan-split.md` (distribution/CI split).
