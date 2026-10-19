# Update CLI shell wrapper (zsh); installed by update-cli wrapper
u() {
  local cmd="${1:-}" root="$PWD" target
  case "$cmd" in
    update|"") ;;
    *) command update-cli "$@"; return $? ;;
  esac
  while [[ "$root" != "/" && ! -f "$root/.update-cli/config.json" ]]; do
    root="$(dirname "$root")"
  done
  if [[ ! -f "$root/.update-cli/config.json" ]]; then
    command update-cli "$@"
    return $?
  fi
  case "$PWD/" in
    "$root/current/"*) ;;
    *) command update-cli "$@"; return $? ;;
  esac
  (cd "$root" && command update-cli "$@") || return $?
  target="$root/current"
  [[ -d "$target" ]] && cd "$target"
}
