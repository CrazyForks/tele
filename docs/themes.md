# Themes

Every color tele renders is a named token. A **theme** is a set of values for
those tokens, and you can write your own.

## Slots

tele holds two themes at once, one per **slot**: the dark slot is used when the
terminal background is dark, the light slot when it is light. tele follows the
terminal, switching as your OS or terminal does.

Configure the slots under `ui.theme`, which takes either a name or a map:

```yaml
ui:
  # Nothing set: the built-in tele-dark and tele-light.

  theme: gruvbox-dark        # this theme in both slots, whatever the background

  theme:
    dark: gruvbox-dark       # dark slot from a file
    light: solarized-light   # light slot from another

  theme:
    dark: gruvbox-dark       # dark slot from a file, light slot stays built-in
```

Naming a single theme fills both slots with it, which is how you say "this
theme, always". There is no separate switch to turn following off — both slots
are simply holding the same theme.

The built-ins are called **`tele-dark`** and **`tele-light`**. They are the root
of every chain: a token no theme sets comes from the built-in for that slot, so a
theme never has to be complete and never breaks when tele adds a token.

> `ui.theme: default` was in every config tele wrote before themes existed. It
> named a pair, and pairs are gone. It is ignored with a warning; delete the
> line.

## Writing a theme

Theme files live next to the config, in `~/.config/tele/themes/`, one theme per
file, named by the file: `themes/mine.yml` is the theme `mine`. Both `.yml` and
`.yaml` are read. In a name, `-` and `_` are the same and case does not matter,
so `my-theme`, `my_theme` and `MyTheme` all name one theme.

The quickest start is to dump what you are already running and edit it:

```console
$ tele --theme-dump > ~/.config/tele/themes/mine.yml
$ tele --theme-dump=light > ~/.config/tele/themes/mine-light.yml
```

To change a few colors, inherit instead. `base:` names the theme this one takes
everything it does not set from:

```yaml
# ~/.config/tele/themes/mine.yml
base: tele-dark
border_pane_active: "#8ec07c"
accent: bright-blue
```

`base:` may name a built-in or another file, and chains are followed as far as
they go. A loop is reported and the slot keeps its built-in.

## Colors

A token takes any of:

| Form | Example | Notes |
|---|---|---|
| Long hex | `"#8ec07c"` | Quote it, or YAML reads `#` as a comment |
| Short hex | `"#8ec"` | Expands to `#88eecc` |
| Palette index | `240` | 0–255 |
| ANSI name | `bright-blue` | The sixteen below |
| `none` | `none` | No color at all |

The sixteen names are `black`, `red`, `green`, `yellow`, `blue`, `magenta`,
`cyan`, `white` and the `bright-` form of each.

An index or a name in the range 0–15 is **not** shorthand for a hex value: those
resolve against the palette your terminal is configured with. Writing a theme in
them makes tele follow your terminal instead of overriding it.

`none` means the attribute is not set. As a foreground the text takes the
terminal's own text color; as a background nothing is painted, which is the only
way to let terminal transparency show through:

```yaml
surface_overlay: none
```

`none` is refused on `highlight_accent`, `highlight_error`,
`highlight_base_chat` and `highlight_base_bubble`. Those four are interpolated
rather than rendered, and "no color" would quietly mean black.

## The two list tokens

`sender_palette` picks a color per sender by id, and takes any number of entries:

```yaml
sender_palette: ["#ff5f5f", "#5fd75f", bright-blue, none]
```

`logo_gradient` is the ramp the splash logo waves through. It needs at least two
stops, ascending, starting at 0 and ending at 1:

```yaml
logo_gradient:
  - {pos: 0.0, color: "#3c5aa0"}
  - {pos: 0.5, color: "#82aae6"}
  - {pos: 1.0, color: "#d7eeff"}
```

Setting either of these **replaces** the list inherited from the base rather
than merging into it. To change one sender color, write all of them.

## When something looks wrong

```console
$ tele --theme-check
```

prints what each slot resolved to, the chain it inherited through, and how many
tokens came from where — including the ones that fell back to the built-in
because nothing in your chain set them. `tele --theme-check=mine` inspects one
theme by name.

Anything tele could not use — a missing theme, an unreadable file, a color it
could not parse, a key it does not know — is a warning, never a failure to
start. Warnings appear as toasts when tele opens and in
`~/.local/state/tele/tele.log`. An unknown key is ignored and the rest of the
file is still used, so a theme written for a later version of tele keeps working
here.

## Editing a theme while tele runs

The `reload_themes` action re-reads the files and reapplies them, without a
restart. It ships with **no key bound**, because it is only useful while writing
a theme. Bind it when you need it:

```yaml
keybindings:
  global:
    reload_themes: ctrl+t
```

## Tokens

Run `tele --theme-dump` for the current value of every token. What each one
paints:

### Surfaces — filled areas drawn behind content

| Token | Where |
|---|---|
| `surface_overlay` | popup menus, reaction picker, mention popup |
| `surface_help` | help modal panel |
| `surface_toast` | toast panel |
| `surface_status_bar` | status bar |
| `surface_selected` | selected row fill, mention-popup border, search prompt |
| `surface_self_mention` | an @mention of you |
| `surface_code` | inline code and code blocks |

### Text

| Token | Where |
|---|---|
| `text_dim` | timestamps, quotes, separators |
| `text_muted` | muted chats |
| `text_faint` | "no results", "empty", overlay hint descriptions |
| `text_subtle` | toast overflow line |
| `text_on_surface` | help modal body |
| `text_status_bar` | status bar body |
| `text_on_selected` | text over `surface_selected` |
| `text_on_selected_muted` | secondary text over `surface_selected` |
| `text_on_toast` | toast body |
| `text_mode_label` | the NORMAL/INSERT label |
| `text_code` | inline code and code blocks |

Body text has no token: it is left unstyled and takes the terminal's foreground.

### Accents

There are three accents because the accent is drawn on three different
backgrounds, and in a light theme they do not agree: the terminal background is
light, but the status bar and (in most themes) the popup panels are not.

| Token | Where |
|---|---|
| `accent` | on the terminal background: the photo, video and search hints, which have no panel behind them |
| `accent_on_surface` | on a panel the app paints: help modal, popup menus, picker numbers, toast action |
| `accent_status_bar` | on the status bar, in NORMAL |
| `accent_insert` | on the status bar, in INSERT |
| `accent_mode_normal` | NORMAL label fill |
| `accent_mode_insert` | INSERT label fill |

If your theme paints a dark status bar under a light terminal — `tele-light`
does — keep `accent_status_bar` and `accent_insert` light. They are the only two
accents whose background does not follow the terminal.

### Status and message state

| Token | Where |
|---|---|
| `status_error`, `status_warning`, `status_info` | toasts and status messages |
| `status_online` | the online dot |
| `tick_sent`, `tick_outbox`, `tick_read`, `tick_failed` | delivery ticks |
| `name_incoming` | incoming sender name |
| `name_editing` | the name while editing |
| `indicator` | selection bar beside a bubble |
| `unread_separator` | the unread divider |
| `waveform_played` | played part of a voice waveform |
| `reaction_chosen` | a reaction you gave |
| `unread_reaction` | unread-reaction glyph in the chat list |
| `unread_mention` | unread-mention glyph in the chat list |

### Borders

| Token | Where |
|---|---|
| `border_pane_active` | the focused pane |
| `border_bubble_in`, `border_bubble_out` | incoming and outgoing bubbles |
| `border_overlay` | help modal |
| `border_composer_focused` | the composer with focus |
| `border_composer_flash` | the composer at its length limit |
| `border_status_sep` | status bar separators |

### Message markup

| Token | Where |
|---|---|
| `markup_link` | urls, emails, phone numbers, cards |
| `markup_ref` | mentions, hashtags, cashtags, bot commands |
| `markup_self_mention_fg` | text of an @mention of you |

### Highlights

| Token | Where |
|---|---|
| `highlight_accent` | the jump-to cue, fading toward its base |
| `highlight_error` | a rolled-back action |
| `highlight_base_chat` | tone the chat-row highlight fades toward |
| `highlight_base_bubble` | tone the bubble highlight fades toward |
| `overlay_dim` | content behind a modal |

The first four are interpolated, so `none` is refused on them.

### Composer

| Token | Where |
|---|---|
| `composer_counter_dim` | the character counter at rest |
| `composer_glyph_idle` | the composer glyph at rest |
| `composer_glyph_ready` | the composer glyph with something to send |

### Lists

| Token | Where |
|---|---|
| `sender_palette` | per-sender name colors, picked by id |
| `logo_gradient` | the splash logo's wave ramp |
