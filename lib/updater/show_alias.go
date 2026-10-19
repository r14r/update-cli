package updater

// BashUpdateAlias returns a Bash function suitable for sourcing or eval.
// The subshell preserves the caller's working directory, including on failure.
func BashUpdateAlias() string {
	return `# Usage: eval "$(update-cli show-alias)"
# Then:  uupdate --setup --no-ui
uupdate() (
  local dir="$PWD"
  local root=""
  while [[ "$dir" != "/" ]]; do
    if [[ "${dir##*/}" == "current" && -f "${dir%/*}/.update-cli/config.json" ]]; then
      root="${dir%/*}"
      break
    fi
    dir="${dir%/*}"
    [[ -n "$dir" ]] || dir="/"
  done

  if [[ -n "$root" ]]; then
    cd -- "$root" || return
  fi
  command update-cli update "$@"
)
`
}
