" cgpt.vim
" Minimal Vim plugin to run cgpt commands with streaming output and filetype context.

Here's the commented version:
" Check if the plugin is already loaded
if exists('g:loaded_cgpt')
  " If it's already loaded, exit early to avoid redefining everything
  finish
endif
" Set a global variable to indicate that the plugin is now loaded
let g:loaded_cgpt = 1

let g:cgpt_backend = get(g:, 'cgpt_backend', 'anthropic')
let g:cgpt_model = get(g:, 'cgpt_model', 'claude-3-5-sonnet-20240620')
let g:cgpt_system_prompt = get(g:, 'cgpt_system_prompt', 'The user is submitting a visual selection in the vim editor to you to help them with a programming task. Prefer to output code directly without surrounding commentary.')
let g:cgpt_config_file = get(g:, 'cgpt_config_file', '')
let g:cgpt_include_filetype = get(g:, 'cgpt_include_filetype', 0)

function! s:handle_output(channel, msg)
  let l:lines = split(a:msg, "\n")
  if s:first_output
    " Replace the "Processing..." line with the first line of output
    call setline(s:range_start, l:lines[0])
    let l:lines = l:lines[1:]
    let s:first_output = 0
  endif
  for l:line in l:lines
    call append(s:current_line - 1, l:line)
    let s:current_line += 1
  endfor
  " Update the visual selection
  call setpos("'>", [0, s:current_line - 1, 1, 0])
  normal! gv
  redraw
endfunction

function! s:job_exit(channel, msg)
  let s:range_end = s:current_line - 1
  " Restore visual selection
  call setpos("'<", [0, s:range_start, 1, 0])
  call setpos("'>", [0, s:range_end, 1, 0])
  normal! gv
endfunction

function! RunCgpt() range
  let s:range_start = line("'<")
  let s:range_end = line("'>")
  let s:current_line = s:range_start
  let s:first_output = 1

  " Capture the selected lines
  let l:selected_lines = getline(s:range_start, s:range_end)
  let l:input = join(l:selected_lines, "\n")

  " Remove the visual selection and add a processing message
  execute s:range_start . "," . s:range_end . "delete _"
  call append(s:range_start - 1, "Processing with cgpt...")
  let s:current_line = s:range_start + 1

  " Include filetype in the prompt if enabled
  if g:cgpt_include_filetype && !empty(&filetype)
    let l:input = "filetype: " . &filetype . "\n\n" . l:input
  endif

  " Build the command with system prompt and config file options
  let l:command = ['cgpt', '--backend', g:cgpt_backend, '--model', g:cgpt_model]
  if !empty(g:cgpt_system_prompt)
    let l:command += ['--system-prompt', g:cgpt_system_prompt]
  endif
  if !empty(g:cgpt_config_file)
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

  " Send the input to the job
  call ch_sendraw(l:channel, l:input . "\n")
  " Close the stdin of the job to indicate no more input
  call ch_close_in(l:channel)
endfunction

" Commands
command! -range CgptRun <line1>,<line2>call RunCgpt()

" Map the visual selection command to 'cg'
vnoremap <silent> cg :call RunCgpt()<CR>
