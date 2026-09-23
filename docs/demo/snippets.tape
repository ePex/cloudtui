# The snippet library: browse with preview, then import a plain JSON file.
# Run from the repo root after docs/demo/setup.sh:  vhs docs/demo/snippets.tape
# Writes docs/demo/snippets.gif.
Output "docs/demo/snippets.gif"

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
Type "snippets"
Sleep 500ms
Enter
Sleep 300ms
Enter
Sleep 1.8s
Down
Sleep 1.2s
Enter
Sleep 800ms
Down
Sleep 1.5s
Down
Sleep 1.8s
Backspace
Sleep 600ms
# payments/
Down
Sleep 800ms
Enter
Sleep 800ms
Down
Sleep 2s
Backspace
Sleep 1s
Type "i"
Sleep 800ms
Type "/tmp/cloudtui-demo/files/order-ORD-1042.json"
Sleep 600ms
Enter
Sleep 1.2s
# accept the suggested name
Enter
Sleep 2s
Type "OrderCreated"
Sleep 600ms
Enter
Sleep 2.5s
