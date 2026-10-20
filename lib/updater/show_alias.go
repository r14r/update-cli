package updater

// BashUpdateAlias returns a Bash function suitable for sourcing or eval.
// The subshell preserves the caller's working directory, including on failure.
func BashUpdateAlias() string {
	return `# Usage: eval "$(update-cli show-alias)"
# Then:  u update --setup --no-ui
# Replace a previous shell alias named u, if present.
unalias u 2>/dev/null || true
u() (
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
  if [[ "$#" -eq 0 ]]; then
    command update-cli update
  else
    command update-cli "$@"
  fi
)
`
}
