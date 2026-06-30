Output demo.gif

Require tele

Set Shell "bash"
Set FontSize 13
Set Width 1440
Set Height 720
Set Theme "Catppuccin Mocha"
Set Framerate 30
Set TypingSpeed 70ms
Set WindowBar "Colorful"
Set Padding 12
Set Margin 12
Set MarginFill "#6B50FF"
Set BorderRadius 8
Set CursorBlink false

# Launch tele (session must already exist at ~/.config/tele/session.json)
Type "tele"
Sleep 1000ms
Enter

# Wait for Telegram connection, splash/logo loader, and initial render
Sleep 4s

# ── Chat list: navigate (scroll bar appears on the right) ─────────────────────
Sleep 500ms
Type "j"
Sleep 260ms
Type "j"
Sleep 260ms
Type "j"
Sleep 260ms
Type "j"
Sleep 900ms
Type "k"
Sleep 280ms
Type "k"
Sleep 700ms

# Half-page scroll, then jump bottom / top with vim motions
Ctrl+D
Sleep 550ms
Type "G"
Sleep 700ms
Type "g"
Type "g"
Sleep 800ms

# ── Folder sidebar: browse down, open a folder, then return to All ───────────
# Selecting a folder moves focus to the chat list pane.
Type "0"
Sleep 650ms
Type "j"
Sleep 380ms
Type "j"
Sleep 380ms
Type "j"
Sleep 600ms
Enter
Sleep 1100ms
# Re-focus the sidebar and step back up to "All" (folders have no gg/G)
Type "0"
Sleep 500ms
Type "k"
Sleep 300ms
Type "k"
Sleep 300ms
Type "k"
Sleep 500ms
Enter
Sleep 900ms

# ── Search: fuzzy filter (background dims), open the demo chat ────────────────
Type "/"
Sleep 300ms
Type "Sorokin"
Sleep 1200ms
Enter
Sleep 2500ms

# ── Walk up through media, message by message (active-message cursor) ─────────
# Cursor starts on the newest message (a photo). Step up with ctrl+k.
Ctrl+K
Sleep 1200ms
# … round video note (renders borderless)
Ctrl+K
Sleep 1600ms
# … rich markdown message: entity coloring, code block, quote, link preview
Sleep 1400ms

# ── Act on the markdown message via the context menu, cursor-driven (j/k) ─────
# The cursor is on the rich markdown message after the ctrl+k walk above. It is
# NOT a reply, so its menu is: Reply(0), React(1), Forward(2), Edit, Delete.
# Do React + Forward here first, while the cursor stays put on this message.

# React — step the cursor down to "React" (index 1), pause, confirm
Space 1
Sleep 800ms
Type "j"
Sleep 700ms
Enter
Sleep 700ms
# Reaction grid (🔥 = row 3, col 1) — walk the cursor with j/l, pause, confirm
Type "j"
Sleep 280ms
Type "j"
Sleep 280ms
Type "j"
Sleep 400ms
Type "l"
Sleep 600ms
Enter
Sleep 1000ms

# Forward — step the cursor down to "Forward" (index 2), pause, confirm
Space 1
Sleep 800ms
Type "j"
Sleep 450ms
Type "j"
Sleep 700ms
Enter
Sleep 1100ms
# Target picker is open (dimmed background) — filter, then confirm
Type "Saved"
Sleep 1600ms
Enter
Sleep 1600ms

# Reply — "Reply" is the top entry; confirm, type, send
Space 1
Sleep 900ms
Enter
Sleep 500ms
Type "love the styling"
Sleep 500ms
Enter
Sleep 700ms
# Sending leaves the composer focused in INSERT — unfocus first
Escape
Sleep 700ms

# Sending jumps the cursor down onto the new reply. That reply's own menu has
# "Jump to original" as its top entry — use it to hop back to the styled message.
Space 1
Sleep 900ms
Enter
Sleep 1200ms

# One step up from the styled message lands on the forwarded voice
Ctrl+K
Sleep 1000ms

# Play — voice menu: Reply, React, Forward, Edit, Play(4), Download, Delete.
# Step down 4 → "Play", confirm (waveform animates)
Space 1
Sleep 800ms
Type "j"
Sleep 320ms
Type "j"
Sleep 320ms
Type "j"
Sleep 320ms
Type "j"
Sleep 600ms
Enter
Sleep 2000ms

# ── Chat-list menu: mark the chat unread — cursor-driven ──────────────────────
# Close the chat first (esc) — an open chat counts as read, so the action won't
# stick. "Mark as unread" is the top entry of a read chat's menu; dwell, confirm.
Escape
Sleep 700ms
Space 1
Sleep 1300ms
Enter
Sleep 1400ms

# ── Compose and send a message ───────────────────────────────────────────────
Enter
Sleep 800ms
Type "a"
Sleep 400ms
Type "Sent from tele in the terminal"
Sleep 600ms
Enter
Sleep 1000ms
Escape
Sleep 800ms

# ── Quit ──────────────────────────────────────────────────────────────────────
Type "q"
Sleep 500ms
