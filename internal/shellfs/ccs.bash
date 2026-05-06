# ccs bash integration. Source via: eval "$(ccs init bash)"
ccs() {
  local cmd="$1"
  case "$cmd" in
    use|switch)
      local _ccs_out
      _ccs_out="$(command ccs "$@" --export)" || return $?
      eval "$_ccs_out"
      ;;
    "")
      command ccs --help
      ;;
    *)
      command ccs "$@"
      ;;
  esac
}
