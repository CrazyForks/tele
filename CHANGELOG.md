# Changelog

All notable changes to this project are documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/).

A human title for a release is written as an em-dash suffix on its heading,
e.g. `## [1.2.0] - 2026-06-11 — Archived folders & image layout fixes`.
Older releases are at <https://github.com/sorokin-vladimir/tele/releases>.

## [Unreleased]

## [1.3.0] - 2026-06-11 — Mute-aware notifications, incoming edits & proxy support

### Added

- Chat list now shows muted (dim `×`) and manual-unread (`[•]`) indicators so
  these states are visible at a glance (#117)
- Connect through a system proxy via the `ALL_PROXY` environment variable
  (SOCKS5/HTTP) (#121)
- Messages edited on another client now update in place without a history
  reload (#42)

### Fixed

- Desktop notifications are no longer shown for muted chats or chats in the
  Archive folder (archived chats are now treated as muted)
- Mute/unmute performed on another device is now reflected at runtime, so muted
  chats stop notifying without needing an app restart
- In-place message updates (edits, reactions) no longer jump the scroll position
  when the message's height changes while viewing the latest messages
- Emoji reaction picker now responds to non-Latin keyboard layouts (e.g. the
  Russian `hjkl` navigation keys), matching the remap used everywhere else

## [1.2.0] - 2026-06-11 — Reliable updates and history scrolling

### Fixed

- Messages and updates keep arriving after the app has been idle for a long
  time, instead of silently stalling until restart (#119)
- Fixed history scrollback looping between two dates instead of loading older messages (#120)
