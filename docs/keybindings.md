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
| `Ctrl+J`  | Select next (newer) message    |
| `Ctrl+K`  | Select previous (older) message|
| `gg`      | Scroll to top                  |
| `G`       | Scroll to bottom               |
| `i` / `a` | Compose message (insert mode)  |
| `r`       | Reply to message               |
| `t`       | React to message               |
| `e`       | Edit own message               |
| `d`       | Delete own message             |
| `f`       | Forward message to another chat|
| `u`       | Attach a file to send          |
| `x`       | Cancel staged upload / clear reply or edit |
| `s`       | Download the selected file     |
| `g`       | Jump to original (for replies) |
| `o`       | Open / view media — photo in the OS viewer, video inline (Kitty + ffmpeg) or in the system player |
| `O`       | Open the selected video in the external player |
| `p`       | Play voice message (in-app)    |
| `Space`   | Context menu                   |

## Compose (insert mode)

| Key      | Action                                            |
| -------- | ------------------------------------------------- |
| `Enter`  | Send message                                      |
| `Ctrl+T` | Toggle send-as type for a staged attachment       |
| `Esc`    | Unfocus the composer (keeps reply / edit / attachment) |

## Chat menu (on a chat-list row, `Space`)

| Key     | Action                  |
| ------- | ----------------------- |
| `r`     | Mark read               |
| `u`     | Mark unread             |
| `m`     | Mute / unmute           |
| `f`     | Add to folder           |
| `a`     | Archive / unarchive     |
| `Esc`   | Close menu              |

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
| `cursor_down`      | Move the active-message cursor to the next (newer) bubble |
| `cursor_up`        | Move the active-message cursor to the previous (older) bubble |
| `confirm`          | Confirm / open the selected item     |

### Chat & messages — context `chat`

| Action              | Description                              |
| ------------------- | ---------------------------------------- |
| `insert`            | Enter insert mode (focus the composer)   |
| `normal`            | Leave insert mode / close the chat       |
| `search`            | Open chat search                         |
| `open_context_menu` | Open the message context menu            |
| `open_in_viewer`    | Open / view the selected media — photo in the OS viewer, video inline or in the system player |
| `open_external`     | Open the selected video in the external player |
| `play_voice`        | Play the selected voice message in-app    |
| `reply`             | Reply to the selected message            |
| `react`             | React to the selected message            |
| `edit`              | Edit the selected (own) message          |
| `forward`           | Forward the selected message to another chat |
| `attach`            | Stage a file from disk to send           |
| `cancel_upload`     | Cancel a staged upload / clear an active reply or edit |
| `download_file`     | Download the selected file to the Downloads folder |

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

### Composer — context `composer`

| Action           | Description                                  |
| ---------------- | -------------------------------------------- |
| `confirm`        | Send the message                             |
| `normal`         | Unfocus the composer (keeps reply/edit/attachment) |
| `toggle_send_as` | Toggle the send-as type for a staged attachment |

### Chat-list menu — contexts `chat_menu`, `folder_submenu`

| Action          | Description                          |
| --------------- | ------------------------------------ |
| `mark_read`     | Mark the selected chat as read       |
| `mark_unread`   | Mark the selected chat as unread     |
| `mute`          | Mute the selected chat               |
| `unmute`        | Unmute the selected chat             |
| `add_to_folder` | Add the chat to a folder             |
| `archive`       | Archive the selected chat            |
| `unarchive`     | Unarchive the selected chat          |

### File picker — context `filepicker`

| Action    | Description                       |
| --------- | --------------------------------- |
| `up`      | Move selection up                 |
| `down`    | Move selection down               |
| `confirm` | Pick the highlighted file         |
| `cancel`  | Dismiss the file picker           |

> Key tokens use the terminal names: letters/digits as-is (`r`, `G`, `2`),
> modifiers like `ctrl+d`, and named keys `enter`, `esc`, `space`, `up`, `down`,
> `left`, `right`.

> **Keyboard layout:** letter bindings also fire on the **same physical key**
> under a Russian (ЙЦУКЕН) layout — e.g. `r`/Reply works whether the key types
> `r` or `к`. Bindings are still written with Latin keys; no duplication needed.
> (Only the Russian layout is mapped today.)
