function! s:handle_output(channel, msg)
  " echom "Output received: " . a:msg
  for line in split(a:msg, "\n")
    " Append the line after the visual selection end
    call append(line("'>"), line)
    " Move the end mark down to maintain the correct range
    normal! gvG
  endfor
endfunction

function! s:job_exit(channel, msg)
  " Handle job exit if needed
  echo "Job exited with message: " . a:msg
endfunction

function! RunCgpt()
  " Get the visual selection range
  let l:range_start = line("'<")
  let l:range_end = line("'>")

  " Capture the selected lines
  let l:input = join(getline(l:range_start, l:range_end), "\n")
  " echom "Input to be sent: " . l:input
  
  " Start the job and handle the output incrementally
  let l:job_id = job_start(['cgpt'], {
        \ 'in_io': 'pipe',
        \ 'out_io': 'pipe',
        \ 'err_io': 'pipe',
        \ 'out_cb': function('s:handle_output'),
        \ 'err_cb': function('s:handle_output'),
        \ 'exit_cb': function('s:job_exit')
        \ })

  " Check if job status:
  if job_status(l:job_id) == "fail"
    echo "Failed to start job"
    return
  endif

  " Get the channel to send input
  let l:channel = job_getchannel(l:job_id)

  " Delete selected lines then re-enable visual mode
  execute l:range_start . "," . l:range_end . "d"
  normal! gv

  " Send the input to the job
  call ch_sendraw(l:channel, l:input . "\n")
  " Close the stdin of the job to indicate no more input
  call ch_close_in(l:channel)
endfunction

" Map the visual selection command to <leader>r
vnoremap <silent> <leader>r :<C-U>call RunCgpt()<CR>

