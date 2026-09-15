Output docs/videos/install.gif

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
Show
Sleep 1s

Type "cat VERSION"
Enter
Sleep 1s

Type "just build"
Enter
Sleep 3s

Type "./dist/update-cli version"
Enter
Sleep 2s

Type `export UPDATE_CLI_INSTALL_BIN_DIR="$PWD/.demo-install/bin"`
Enter
Type "just install"
Enter
Sleep 3s

Type `"$UPDATE_CLI_INSTALL_BIN_DIR/update-cli" version`
Enter
Sleep 2s
