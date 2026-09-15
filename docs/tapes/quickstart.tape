Output docs/videos/quickstart.gif

Set Shell "bash"
Set FontSize 18
Set Width 1400
Set Height 850
Set TypingSpeed 25ms
Set PlaybackSpeed 1.0
Set Theme "Catppuccin Frappe"

Hide
Type "clear"
Enter
Type "rm -rf .demo-quickstart && mkdir -p .demo-quickstart/Downloads .demo-quickstart/demo-app"
Enter
Type `./scripts/tapes/create-demo-release.sh "$PWD/.demo-quickstart/Downloads" demo-app 1.2.3 >/dev/null`
Enter
Type "cd .demo-quickstart/demo-app"
Enter
Show
Sleep 1s

Type "pwd"
Enter
Sleep 1s

Type `../../dist/update-cli init --project demo-app --downloads "../Downloads" --no-ui`
Enter
Sleep 4s

Type "../../dist/update-cli status --no-ui"
Enter
Sleep 2s

Type "find . -maxdepth 2 -type f | sort"
Enter
Sleep 3s
