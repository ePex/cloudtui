# Requeue a whole DLQ: mark all, move, the matching queue is offered first.
# Run from the repo root after docs/demo/setup.sh:  vhs docs/demo/dlq-requeue.tape
# Writes docs/demo/dlq-requeue.gif.
Output "docs/demo/dlq-requeue.gif"

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
Type "dlq"
Sleep 600ms
Enter
Sleep 1s
Enter
Sleep 2s
# mark all
Type "a"
Sleep 1s
# move: orders.created is offered first
Type "m"
Sleep 2s
Enter
Sleep 2.5s
Escape
Sleep 1s
# clear the filter
Type "/"
Sleep 400ms
Ctrl+U
Sleep 300ms
Enter
Sleep 300ms
Type "r"
Sleep 2.5s
