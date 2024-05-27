" cgpt.vim
" Vim plugin to run cgpt commands.

function! s:handle_output(channel, msg)
  for line in split(a:msg, "\n")
    " Append the line after the visual selection end
    call append(line("'>"), line)
    " Move the end mark down to maintain the correct range
    let l:end_line = line("'>") + 1
    call setpos("'>", [0, l:end_line, 1, 0])
  endfor
endfunction

function! s:job_exit(channel, msg)
  " Handle job exit if needed
  echo "Job exited with message: " . a:msg

 " Restore visual selection:
  " Move the end mark back to the last line of the output
  " This is needed because the last line of the output is not included in the range
  " when the job exits
  let l:end_line = line("'>") - 1
  call setpos("'>", [0, l:end_line, 1, 0])
  " Restore visual selection
  normal! gv

endfunction

function! RunCgpt()
  " Get the visual selection range
  let l:range_start = line("'<")
  let l:range_end = line("'>")

  " Capture the selected lines
  let l:input = join(getline(l:range_start, l:range_end), "\n")
  " echom "Input to be sent: " . l:input
  
  " Build the command with system prompt and config file options
  let l:command = ['cgpt']
  if exists('g:cgpt_system_prompt') && !empty(g:cgpt_system_prompt)
    let l:command += ['--system-prompt', g:cgpt_system_prompt]
  endif
  if exists('g:cgpt_config_file') && !empty(g:cgpt_config_file)
    let l:command += ['--config', g:cgpt_config_file]
  endif

  " Start the job and handle the output incrementally
  let l:job_id = job_start(l:command, {
        \ 'in_io': 'pipe',
        \ 'out_io': 'pipe',
        \ 'err_io': 'pipe',
        \ 'out_cb': function('s:handle_output'),
        \ 'err_cb': function('s:handle_output'),
        \ 'exit_cb': function('s:job_exit'),
        \ 'noblock': 1
        \ })

  " Check if job status:
  if job_status(l:job_id) == "fail"
    echo "Failed to start job"
    return
  endif

  " Get the channel to send input
  let l:channel = job_getchannel(l:job_id)

  " Capture the cursor position
  let l:cursor_pos = getcurpos()

  " Delete the visual selection:
  execute l:range_start . "," . l:range_end . "d"

  " Restore the cursor to the starting line
  call setpos('.', l:cursor_pos)

  " Send the input to the job
  call ch_sendraw(l:channel, l:input . "\n")
  " Close the stdin of the job to indicate no more input
  call ch_close_in(l:channel)
endfunction

" Map the visual selection command to <leader>r
vnoremap <silent> <leader>r :<C-U>call RunCgpt()<CR>
