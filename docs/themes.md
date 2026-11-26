# Single-file themes

The same UTF-8 HTML file can be uploaded at runtime or added to `themes/` in a pull request. Start with two lines.

```html
<meta name="theme" content="violet" title="Violet">
<style>.entry { border-color: #7666a1; border-radius: 2px; }</style>
```

The metadata names the stable theme ID and its display name. The default provides every layout you omit. Selecting a file creates a session-bound preview. Saving installs and activates the file. When replacing a theme with the same ID, its previous source remains available for rollback. Site preferences are stored by theme ID.

## Built-in layouts

Select a theme under **Settings → Appearance → Active theme**. Use **Preview** to compare it with your current layout before saving.

| Theme | Feed layout | Adjustable details |
| --- | --- | --- |
| Switchyard | Compact dark rows with a separate timestamp column and amber signal stripe | Signal color and row padding |
| Postmark | Cream letter sheets with airmail edging and a date stamp | Seal ink and space between letters |
| Contact Sheet | A dark three-column proof sheet with images above their captions, collapsing to a single column on phones | Frame color and gutter width |
| Lagoon | A mint sidebar alongside a stream of rounded speech bubbles | Sidebar color and bubble corners |
| Risograph | Bold masthead, ink outlines, and offset shadows on stacked print cards | Masthead stock color and print offset |

Paper, Index, Dusk, and Starter are also included. All nine themes are available as single files in the repository’s `themes/` directory.

## Share your theme

Download your HTML file from **Settings → Appearance → Download theme file**. Start a discussion with the theme name as the title and link the file or paste its source. A screenshot helps readers decide whether to try it. [Share a theme](https://github.com/sametcn99/th0ughts/discussions/new?category=show-and-tell)

To try someone else's theme, download their HTML file and open **Settings → Appearance → Upload a theme → Preview**. Reply in the original thread with feedback or screenshots. Authors can edit the opening post when they release an update. [Browse community themes](https://github.com/sametcn99/th0ughts/discussions/categories/show-and-tell)

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

### Layouts for short posts and Articles

A translation with a title or more than 280 visible characters is an Article. The timeline displays an excerpt and the permalink displays the full body. To give each a different structure, define `item-short`, `item-article`, `item-short-detail`, or `item-article-detail`. A permalink first tries its detail template, then the type-specific template, then `item`. Every variant accepts the same slots as `item`.

Use `data-if="article"`, `short`, `detail`, or `list` on an item container, slot, or localized label. For example, an Article card can place the author at the top and the permalink below its excerpt.

```html
<template id="item-article">
  <header><slot name="author"></slot><slot name="title"></slot></header>
  <section><slot name="body"></slot><slot name="media"></slot></section>
  <footer>
    <slot name="readmore"></slot><slot name="date"></slot>
    <slot name="tags"></slot><slot name="actions"></slot>
  </footer>
</template>
```

Replace `byline` with both `author` and `date` to position them independently. Add `avatar` and `status` wherever you want the initial, pin, or draft indicator. The default byline includes all four. When you omit `readmore`, the compiler places the Article link after `body`.

### Shared palette and responsive CSS

Set custom properties on `:root` to style the timeline, permalink, footer, and the controls inside the login, composer, and settings shadow roots. Ordinary selectors stay within the public themed layout. The following properties are shared across the application.

| Properties | Appearance |
| --- | --- |
| `--canvas`, `--surface`, `--subtle` | Page, panel and input backgrounds, secondary surfaces |
| `--ink`, `--muted`, `--line` | Primary text, secondary text and borders |
| `--accent`, `--on-accent`, `--accent-surface` | Buttons, links, selected states and their foregrounds |
| `--focus`, `--shadow`, `--panel-shadow`, `--backdrop` | Keyboard focus, menus, dialog shadows and overlays |
| `--footer-surface`, `--footer-ink` | Footer background and link text |
| `--ui-font`, `--scheme` | Local font stack and native control color scheme |

Define your own variables in a class or on the root. Use CSS comments, child and descendant selectors, and interactive states including `:hover`, `:focus-visible`, `:focus-within`, `:active`, `:checked`, and `:disabled`. A style element can contain regular `@media` blocks or use a `media` attribute. Both accept width breakpoints, preferred color schemes, and reduced-motion preferences. Option overrides are emitted after theme rules.

```css
:root { --accent: #85462e; --surface: #fffaf2; }
.read-post:focus-visible { outline: 2px solid var(--accent); }
@media (prefers-color-scheme: dark) {
  :root { --scheme: dark; --canvas: #171419; --surface: #211d24; --ink: #f7edf0; }
}
```

See `themes/dusk.html` for a complete light and dark palette. A range can use fractional bounds and a positive numeric `step`, such as `"min":0.5,"max":2,"step":0.25,"unit":"rem","default":"0.75"`. Steps are counted from the minimum. Omitting the step accepts any in-range decimal. The server validates overrides for every theme on save. When importing an updated schema in Appearance, compatible overrides carry forward.

### Validate, preview and manage revisions

Run `th0ughts -file ./my-theme.html theme-check` locally, before uploading anything. Add `-lang tr` before the command for Turkish diagnostics. The UI and API also report the line, the affected rule or field, and its token.

In **Settings → Appearance**, upload an HTML file, choose **Preview**, and select the timeline, a short permalink, an Article card or permalink, sign-in, composer, or settings. The controls shown in previews are inert snapshots rendered from the application components. **Save** commits the new active revision and site settings together. A failed save keeps the last published revision visible.

**Download theme file** exports the selected source. **Version history** lists earlier distinct revisions, newest first. Restoring a revision takes effect immediately and keeps the replaced version in history. To delete a custom theme, activate another theme first, then select the inactive custom theme and choose **Delete theme**. Built-in themes are protected. Site backups include custom themes and their revision histories, and the importer accepts earlier backups containing a single previous revision.

### Comment threads

A post template can place `<slot name="comments"></slot>` anywhere in its layout. Existing themes receive the thread after the posts slot automatically. Both short posts and Articles use the same thread. The preview of a post permalink includes a sample comment.

The thread uses the shared surface, text, accent, border, and focus tokens. Use class selectors such as `.comment-thread`, `.thread-heading`, `.thread-node`, `.comment-branch`, `.comment-meta`, `.comment-images`, and `.guest-comment` to change its layout. The reply tree and guest form render inside the themed document.

### Diagrams and the media viewer

Post bodies expose rendered diagrams through `.mermaid-figure`, with a focusable image button and a native source disclosure. The media dialog uses the shared palette (including `--surface`, `--ink`, `--line`, and `--focus`). Include the body slot in both timeline and detail views. Keep linked images reachable by keyboard and give date, tag, and action links enough space for touch. Test your palette with the keyboard focus ring visible and at a viewport width of 320 CSS pixels.
