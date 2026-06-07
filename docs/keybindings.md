# Keybindings

## Global

| Key                       | Action          |
| ------------------------- | --------------- |
| `0`                       | Focus folders   |
| `1` / `h` / `←`           | Focus chat list |
| `2` / `l` / `→`           | Focus chat      |
| `q` / `Ctrl+Q` / `Ctrl+C` | Quit            |

## Chat list

| Key                 | Action                     |
| ------------------- | -------------------------- |
| `j` / `↓`           | Next chat                  |
| `k` / `↑`           | Previous chat              |
| `G`                 | Last chat                  |
| `Ctrl+D` / `Ctrl+U` | Scroll half-page down / up |
| `Enter`             | Open chat                  |
| `/`                 | Search chats               |

## Chat (normal mode)

| Key       | Action                         |
| --------- | ------------------------------ |
| `j` / `↓` | Scroll down                    |
| `k` / `↑` | Scroll up                      |
| `gg`      | Scroll to top                  |
| `G`       | Scroll to bottom               |
| `i` / `a` | Compose message (insert mode)  |
| `r`       | Reply to message               |
| `t`       | React to message               |
| `e`       | Edit own message               |
| `d`       | Delete own message             |
| `g`       | Jump to original (for replies) |
| `o`       | Open photo/video in external app |
| `p`       | Play voice message (in-app)    |
| `Space`   | Context menu                   |

## Compose (insert mode)

| Key     | Action              |
| ------- | ------------------- |
| `Enter` | Send message        |
| `Esc`   | Back to normal mode |

## Configurable actions

These are the action names usable as YAML keys in the `keybindings:` section of
`~/.config/tele/config.yml` (grouped by `context`). Listing keys for an action
replaces that action's defaults in that context; unlisted actions keep theirs.
A chord is space-separated key tokens (`"g g"` = press `g` then `g`).

### Focus & app — context `global`

| Action          | Description                |
| --------------- | -------------------------- |
| `focus_folders` | Focus the folders sidebar  |
| `focus_chatlist`| Focus the chat list        |
| `focus_chat`    | Focus the chat pane        |
| `focus_prev`    | Focus the previous pane    |
| `focus_next`    | Focus the next pane        |
| `quit`          | Quit the app               |

### Navigation & scrolling — contexts `folders`, `chatlist`, `chat`, `context_menu`, `delete_submenu`, `search`

| Action             | Description                          |
| ------------------ | ------------------------------------ |
| `up`               | Move selection up / scroll up        |
| `down`             | Move selection down / scroll down    |
| `go_top`           | Jump to the top (first / oldest)     |
| `go_bottom`        | Jump to the bottom (last / newest)   |
| `scroll_half_down` | Scroll half a page down              |
| `scroll_half_up`   | Scroll half a page up                |
| `confirm`          | Confirm / open the selected item     |

### Chat & messages — context `chat`

| Action              | Description                              |
| ------------------- | ---------------------------------------- |
| `insert`            | Enter insert mode (focus the composer)   |
| `normal`            | Leave insert mode / close the chat       |
| `search`            | Open chat search                         |
| `open_context_menu` | Open the message context menu            |
| `open_in_viewer`    | Open the selected photo/video in an external app |
| `play_voice`        | Play the selected voice message in-app    |
| `reply`             | Reply to the selected message            |
| `edit`              | Edit the selected (own) message          |

### Context menu — contexts `context_menu`, `delete_submenu`

| Action             | Description                           |
| ------------------ | ------------------------------------- |
| `cancel`           | Dismiss the current menu or picker    |
| `react`            | React to the selected message         |
| `play_voice`       | Play the selected voice message       |
| `edit`             | Edit the selected message             |
| `delete`           | Delete the selected message           |
| `delete_revoke`    | Delete for everyone                   |
| `delete_me`        | Delete only for me                    |
| `jump_to_original` | Jump to the original (replied-to) message |

> Key tokens use the terminal names: letters/digits as-is (`r`, `G`, `2`),
> modifiers like `ctrl+d`, and named keys `enter`, `esc`, `space`, `up`, `down`,
> `left`, `right`.
