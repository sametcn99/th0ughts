# Single-file themes

The same UTF-8 HTML file can be uploaded at runtime or added to `themes/` in a pull request. Start with two lines.

```html
<meta name="theme" content="violet" title="Violet">
<style>.entry { border-color: #7666a1; border-radius: 2px; }</style>
```

The metadata names the stable theme ID and its display name. The default provides every layout you omit. Selecting a file creates a session-bound preview. Saving installs and activates the file. When replacing a theme with the same ID, its previous source remains available for rollback. Site preferences are stored by theme ID.

## Change a layout

A named `template` wraps slots in semantic HTML. Keep every slot of the layout exactly once. The engine iterates over the page of posts, renders the `item` layout for each, and bounds rendering to that page.

| Template | Slots |
| --- | --- |
| `feed` | `header`, `filters`, `posts`, `composer`, `settings`, `pagination` |
| `post` | `header`, `posts`, `composer`, `settings`, `pagination` |
| `item` | `byline`, `title`, `body`, `media`, `tags`, `actions` |
| `login` | `header`, `login` |

```html
<template id="item">
  <header class="entry-heading">
    <slot name="title"></slot>
    <slot name="byline"></slot>
  </header>
  <section data-if="media"><slot name="media"></slot></section>
  <section><slot name="body"></slot></section>
  <footer><slot name="tags"></slot><slot name="actions"></slot></footer>
</template>
```

Use `div`, `section`, `aside`, `header`, `main`, `footer`, or `article` as containers. `data-if` supports `owner`, `pinned`, `draft`, `title`, `media`, and `tags`. A theme can supply its own localized text with `<theme-text en="…" tr="…"></theme-text>`. The application owns the SEO head, authentication controls, and interactive slot contents.

## Style and options

CSS accepts class selectors and descendant elements or classes. Supported properties cover typography, spacing, borders, color, flex, and grid. Functions include `var`, `minmax`, `repeat`, `clamp`, and `linear-gradient`. Use a separate style element with `media="(max-width: 650px)"` for a breakpoint. The validator scopes rules to the theme, rejects network-loading declarations and active content, and caps source size at 64 KiB and layout nesting at 12 levels.

Declare color controls inside the same file. The selected value is available as a CSS custom property. Option labels fall back to English and then to an available theme label.

```html
<script type="application/json" id="options">
[{"key":"accent","labels":{"en":"Accent color","tr":"Vurgu rengi"},"default":"#7666a1"}]
</script>
<style>.entry-heading { color: var(--accent); }</style>
```

Preview a theme and its unsaved options from the timeline before activation. The preview ticket for an uploaded file lasts five minutes. Close and reopen the preview to refresh it. Its preview frame is sandboxed and contains no session controls or executable theme code. Open `/?recover=1` in an owner session to restore a usable view. Offline recovery is available through the CLI. The complete Paper and Index files demonstrate different timelines and entry layouts.

The option type defaults to `color`. Use `select` with localized `choices` to offer fonts or layout presets, `range` with `min`, `max`, and an optional `px`, `rem`, or `%` unit for spacing, and `toggle` for a numeric CSS switch. Defaults and persisted values pass the same type-specific checks. Paper includes working font and spacing controls.
