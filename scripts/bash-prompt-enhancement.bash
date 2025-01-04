expand_claude_prompt() {
    local current_line="${READLINE_LINE}"
    local cursor_pos="${READLINE_POINT}"
    local pipeline_start=$(echo "${current_line}" | grep -b -o '|[[:space:]]*cgpt' | cut -d: -f1)
    
    if [[ -n "$pipeline_start" ]]; then
        local pipeline="${current_line:0:$pipeline_start}"
        local expansion=$(generate-input-description-for-claude "$pipeline")
        
        # Save current cursor position
        local old_stty=$(stty -g)
        stty raw -echo min 0 time 0
        echo -en "\e[6n" > /dev/tty
        IFS=';' read -r -d R -a pos < /dev/tty
        stty "$old_stty"
        
        # Calculate new cursor position after expansion
        local new_cursor_pos=$((cursor_pos + ${#expansion}))
        
        # Move cursor down and show hint
        echo -en "\e[1B\e[K\e[2m${expansion}\e[0m"
        
        # Move cursor back to original position
        echo -en "\e[1A\e[${pos[1]}G"
        
        # Set up a timer to clear the hint after 5 seconds
        ( sleep 5; echo -en "\e[1B\e[K\e[1A\e[${pos[1]}G" > /dev/tty ) &
        
        # Bind a key to accept the hint
        bind '"\C-m": "\C-e\C-u'"${current_line:0:$cursor_pos}${expansion}${current_line:$cursor_pos}"'\C-m"'
    fi
}