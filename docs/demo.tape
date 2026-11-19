# Regenerate docs/demo.gif from the repo root (needs Docker):
#   go build -o igc-sync . && docker run --rm -v "$PWD":/vhs ghcr.io/charmbracelet/vhs docs/demo.tape
# "Up" picks the event above whichever one is current; adjust to land on one with files.

Output docs/demo.gif
Set Shell bash
Set FontSize 16
Set Width 1000
Set Height 520
Set Padding 16
Set Theme "Catppuccin Mocha"
Set TypingSpeed 50ms
Hide
Type "export PATH=/vhs:$PATH PS1='~/scoring $ ' && mkdir -p /root/scoring && cd /root/scoring && clear"
Enter
Show
Type "igc-sync"
Sleep 300ms
Enter
Sleep 3s
Up
Sleep 800ms
Enter
Wait+Screen@40s /Next check in/
Sleep 3.5s
Type "q"
Sleep 300ms
Set TypingSpeed 15ms
Type "ls central-plateau-*/"
Enter
Sleep 3.5s
