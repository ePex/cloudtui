# Home -> queue list -> filter -> messages -> message detail.
# Run from the repo root after docs/demo/setup.sh:  vhs docs/demo/queues.tape
# Writes docs/demo/queues.gif.
Output "docs/demo/queues.gif"

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

Sleep 1.5s
Enter
Sleep 2s
Type "/"
Sleep 400ms
Type "orders"
Sleep 800ms
Enter
Sleep 1s
# past dlq.orders.created
Down
Sleep 800ms
Enter
Sleep 2s
Down
Sleep 500ms
Down
Sleep 800ms
Enter
Sleep 3.5s
Escape
Sleep 1s
Escape
Sleep 1.5s
