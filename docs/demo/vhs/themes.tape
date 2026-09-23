# Live theme switching with :theme (dark -> cyberpunk -> dark).
# The theme name is typed while hidden, so the autocomplete never shows
# every bundled theme on screen.
# Run from the repo root after docs/demo/setup.sh:  vhs docs/demo/vhs/themes.tape
# Writes docs/demo/vhs-themes.gif, next to the committed record.py GIF.
Output "docs/demo/vhs-themes.gif"

Set Shell "bash"
Set FontSize 13
Set Width 1280
Set Height 760
Set Padding 10
Set TypingSpeed 70ms

Hide
Type "HOME=/tmp/cloudtui-demo/home /tmp/cloudtui-demo/bin/cloudtui"
Enter
Sleep 2.5s
Show

Type ":"
Sleep 400ms
Type "queues"
Sleep 500ms
Enter
Sleep 300ms
Enter
Sleep 1.8s
Sleep 1s
Type ":"
Sleep 400ms
Hide
Type@0ms "theme cyberpunk"
Sleep 300ms
Show
Sleep 800ms
Enter
Sleep 300ms
Enter
Sleep 2.5s
Down
Sleep 400ms
Down
Sleep 1s
Type ":"
Sleep 400ms
Hide
Type@0ms "theme dark"
Sleep 300ms
Show
Sleep 800ms
Enter
Sleep 300ms
Enter
Sleep 2.5s
