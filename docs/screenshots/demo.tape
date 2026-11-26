# Renders the README screenshots from the demo ledger. Run it with: mise run screenshots
# The task copies demo/ to /tmp/konto-demo and sets $KONTO_BIN, so no real path shows in the tab bar.
Set Shell bash
Set FontSize 14
Set Width 1400
Set Height 820
Set Padding 16
Set Theme "TokyoNight"
Set TypingSpeed 0

Hide
Type "export KONTO_TODAY=2026-10-15 XDG_DOWNLOAD_DIR=/tmp/konto-demo/downloads && clear && $KONTO_BIN --file /tmp/konto-demo/main.journal" Enter
Sleep 3s
Show
Sleep 500ms
Screenshot docs/screenshots/dashboard.png
Sleep 500ms

Type "2"
Type "n"
Sleep 1s
Down
Enter
Sleep 3s
Screenshot docs/screenshots/import.png
Sleep 500ms

Down
Enter
Sleep 1s
Screenshot docs/screenshots/import-categorise.png
Sleep 500ms
Escape
Sleep 1s

Type "3"
Sleep 2s
Screenshot docs/screenshots/scheduled.png
Sleep 500ms

Type "q"
Sleep 500ms
