# ccs-cli fish integration. Source via: ccs-cli init fish | source
function ccs
    set -l cmd $argv[1]
    switch "$cmd"
        case use switch
            set -l _ccs_out (command ccs-cli $argv --export)
            or return $status
            eval $_ccs_out
        case ""
            command ccs-cli --help
        case '*'
            command ccs-cli $argv
    end
end
