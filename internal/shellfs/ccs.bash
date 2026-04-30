# ccs-cli bash integration. Source via: eval "$(ccs-cli init bash)"
ccs() {
  local cmd="$1"
  case "$cmd" in
    use|switch)
      local _ccs_out
      _ccs_out="$(command ccs-cli "$@" --export)" || return $?
      eval "$_ccs_out"
      ;;
    "")
      command ccs-cli --help
      ;;
    *)
      command ccs-cli "$@"
      ;;
  esac
}
