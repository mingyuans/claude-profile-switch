# ccs fish integration. Source via: ccs init fish | source
function ccs
    set -l cmd $argv[1]
    switch "$cmd"
        case use switch
            set -l _ccs_out (command ccs $argv --export)
            or return $status
            eval $_ccs_out
        case ""
            command ccs --help
        case '*'
            command ccs $argv
    end
end
