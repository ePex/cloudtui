# Compose a message from a library snippet and send it from inside the queue.
# Run from the repo root after docs/demo/setup.sh:  vhs docs/demo/send-snippet.tape
# Writes docs/demo/send-snippet.gif.
Output "docs/demo/send-snippet.gif"

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
Type "/"
Sleep 400ms
Type "orders.created"
Sleep 600ms
Enter
Sleep 1s
# past dlq.orders.created
Down
Sleep 800ms
Enter
Sleep 2s
Type "c"
Sleep 1.2s
Tab@150ms 7
# Load snippet...
Enter
Sleep 1.2s
# orders/
Down
Sleep 600ms
Enter
Sleep 800ms
# order-created.json
Down 2
Sleep 600ms
Enter
Sleep 1.8s
# Submit
Shift+Tab
Shift+Tab
Sleep 500ms
Enter
Sleep 3s
