" cgpt.vim
" Vim plugin to run AI-assisted coding commands with streaming output and comprehensive context.

if exists('g:loaded_cgpt')
  finish
endif
let g:loaded_cgpt = 1

" Plugin configuration options with default values
let g:cgpt_backend = get(g:, 'cgpt_backend', 'anthropic')
let g:cgpt_model = get(g:, 'cgpt_model', 'claude-3-5-sonnet-20240620')
let g:cgpt_system_prompt = get(g:, 'cgpt_system_prompt', 'You will receive input structured in XML tags. The <ai_input> tag contains all input. Inside, you may find <file_info>, <file_context>, <editor_info>, <version_control>, <code_analysis>, and <user_selection> tags. The user is submitting a selection from their code editor to get help with a programming task. Analyze all provided context and respond appropriately. Your output will replace the user''s selection. Prefer to output code directly without surrounding commentary unless explanation is necessary. If you output commentary, make sure it is in a comment appropriate for the language.')


let g:cgpt_config_file = get(g:, 'cgpt_config_file', '')

let g:cgpt_include_filetype = get(g:, 'cgpt_include_filetype', 1)
let g:cgpt_include_filepath = get(g:, 'cgpt_include_filepath', 1)

let g:cgpt_include_surrounding_context = get(g:, 'cgpt_include_surrounding_context', 10)

let g:cgpt_include_git_status = get(g:, 'cgpt_include_git_status', 1)
let g:cgpt_include_quickfix = get(g:, 'cgpt_include_quickfix', 1)

let g:cgpt_include_ale_lint = get(g:, 'cgpt_include_ale_lint', 1)
let g:cgpt_max_issues = get(g:, 'cgpt_max_issues', 5)

let g:cgpt_max_output_lines = get(g:, 'cgpt_max_output_lines', 1000)
let g:cgpt_timeout = get(g:, 'cgpt_timeout', 60)
let g:cgpt_selected_text_position = get(g:, 'cgpt_selected_text_position', 'end')

function! s:escape_xml(text)
  let l:escaped = substitute(a:text, '&', '&amp;', 'g')
  let l:escaped = substitute(l:escaped, '<', '&lt;', 'g')
  let l:escaped = substitute(l:escaped, '>', '&gt;', 'g')
  return l:escaped
endfunction

function! s:get_git_status()
  if !exists('*FugitiveStatusline')
    return ''
  endif
  let l:git_status = FugitiveStatusline()
  if empty(l:git_status)
    return ''
  endif
  let l:git_status = substitute(l:git_status, '^\[Git(\(.*\))\]$', '\1', '')
  return l:git_status
endfunction

function! s:get_relevant_issues(start, end, selection_start, selection_end)
  let l:issues = {}
  let l:selection_issues = {}

  " Include quickfix items if enabled
  if g:cgpt_include_quickfix
    for l:qf_item in getqflist()
      if l:qf_item.bufnr == bufnr('%') && l:qf_item.lnum >= a:start && l:qf_item.lnum <= a:end
        let l:key = l:qf_item.lnum . ':' . l:qf_item.text
        let l:issue = {'type': 'Quickfix', 'lnum': l:qf_item.lnum, 'text': l:qf_item.text}
        if l:qf_item.lnum >= a:selection_start && l:qf_item.lnum <= a:selection_end
          let l:selection_issues[l:key] = l:issue
        else
          let l:issues[l:key] = l:issue
        endif
      endif
    endfor
  endif

  " Include ALE lint issues if enabled
  if g:cgpt_include_ale_lint && exists('*ale#engine#GetLoclist')
    let l:ale_list = g:ale_set_quickfix ? getqflist() : ale#engine#GetLoclist(bufnr('%'))
    for l:item in l:ale_list
      if l:item.lnum >= a:start && l:item.lnum <= a:end
        let l:key = l:item.lnum . ':' . l:item.text
        " Only add ALE issue if it's not already present
        if !has_key(l:selection_issues, l:key) && !has_key(l:issues, l:key)
          let l:issue = {'type': 'ALE', 'lnum': l:item.lnum, 'text': l:item.text}
          if l:item.lnum >= a:selection_start && l:item.lnum <= a:selection_end
            let l:selection_issues[l:key] = l:issue
          else
            let l:issues[l:key] = l:issue
          endif
        endif
      endif
    endfor
  endif

  " Combine and sort issues
  let l:combined_issues = sort(values(l:selection_issues) + values(l:issues), {i1, i2 -> i1.lnum - i2.lnum})

  " Return the limited number of issues
  return l:combined_issues[:g:cgpt_max_issues - 1]
endfunction

function! s:handle_output(channel, msg)
  let l:lines = split(a:msg, "\n")
  if s:first_output
    " Replace the "Processing..." line with the first line of output
    call setline(s:range_start, l:lines[0])
    let l:lines = l:lines[1:]
    let s:first_output = 0
  endif
  for l:line in l:lines
    if s:current_line - s:range_start >= g:cgpt_max_output_lines
      call append(s:current_line - 1, "... (output truncated)")
      break
    endif
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


" Function to process cgpt output
function! s:process_cgpt_output(channel, msg)
  let l:output = a:msg
  " Remove any ANSI escape sequences
  let l:output = substitute(l:output, '\e\[[0-9;]*[mK]', '', 'g')
  " Append the processed output to the buffer
  call append(line('.'), split(l:output, "\n"))
endfunction

function! RunCgpt() range
  let [s:range_start, s:range_end, s:current_line, s:first_output] = [a:firstline, a:lastline, a:firstline, 1]
  let l:selected_text = join(getline(s:range_start, s:range_end), "\n")

  " Remove the visual selection and add a processing message
  execute s:range_start . "," . s:range_end . "delete _"
  call append(s:range_start - 1, "Processing with cgpt...")
  let s:current_line = s:range_start + 1

  " Prepare the input components
  let l:input_components = {}
  
  if g:cgpt_include_filetype && !empty(&filetype)
    let l:input_components.filetype = "  <file_info><type>" . &filetype . "</type></file_info>\n"
  endif

  if g:cgpt_include_filepath
    let l:input_components.filepath = "  <file_info><path>" . s:escape_xml(expand('%:p')) . "</path></file_info>\n"
  endif

  let l:context_lines = g:cgpt_include_surrounding_context
  let [l:start, l:end] = l:context_lines > 0 
        \ ? [max([1, s:range_start - l:context_lines]), min([line('$'), s:range_end + l:context_lines])]
        \ : [1, line('$')]

  let l:context = map(range(l:start, l:end), {idx, lnum -> printf('%d:%s', lnum, getline(lnum))})
  let l:filename = expand('%:t')

  let l:input_components.file_context = "  <file_context>\n" 
        \ . "    <filename>" . s:escape_xml(l:filename) . "</filename>\n"
        \ . "    <context_type>" . (l:context_lines > 0 ? "surrounding" : "whole_file") . "</context_type>\n"
        \ . "    <start_line>" . l:start . "</start_line>\n"
        \ . "    <end_line>" . l:end . "</end_line>\n"
        \ . "    <content>\n" 
        \ . join(map(l:context, '"      " . s:escape_xml(v:val) . "\n"'), '')
        \ . "    </content>\n"
        \ . "  </file_context>\n"

  let l:issues = s:get_relevant_issues(l:start, l:end, s:range_start, s:range_end)
  if !empty(l:issues)
    let l:input_components.issues = "  <code_issues>\n"
    for l:issue in l:issues
      let l:in_selection = l:issue.lnum >= s:range_start && l:issue.lnum <= s:range_end
      let l:input_components.issues .= printf("    <issue type=\"%s\" in_selection=\"%s\">\n      <line>%d</line>\n      <description>%s</description>\n    </issue>\n",
            \ l:issue.type, l:in_selection ? "true" : "false", l:issue.lnum, s:escape_xml(l:issue.text))
    endfor
    let l:input_components.issues .= "  </code_issues>\n"
  endif

  if g:cgpt_include_git_status
    let l:git_status = s:get_git_status()
    if !empty(l:git_status)
      let l:input_components.git_status = "  <version_control><git_status>\n    " . s:escape_xml(l:git_status) . "\n  </git_status></version_control>\n"
    endif
  endif

  let l:input_components.selected_text = "  <user_selection>\n"
        \ . "    <start_line>" . s:range_start . "</start_line>\n"
        \ . "    <end_line>" . s:range_end . "</end_line>\n"
        \ . "    <content>\n"
        \ . join(map(range(s:range_start, s:range_end), {idx, lnum -> "      " . s:escape_xml(printf('%d:%s', lnum, getline(lnum))) . "\n"}), '')
        \ . "    </content>\n"
        \ . "  </user_selection>\n"

  let l:input = "<ai_input>\n" 
        \ . (g:cgpt_selected_text_position ==? 'beginning' ? l:input_components.selected_text : '')
        \ . join(filter(values(l:input_components), {idx, val -> val !=# l:input_components.selected_text}), '')
        \ . (g:cgpt_selected_text_position !=? 'beginning' ? l:input_components.selected_text : '')
        \ . "</ai_input>"

  " Start the job
  let l:command = ['cgpt', '--backend', g:cgpt_backend, '--model', g:cgpt_model]
  if !empty(g:cgpt_system_prompt)
    let l:command += ['--system-prompt', g:cgpt_system_prompt]
  endif
  if !empty(g:cgpt_config_file)
    let l:command += ['--config', g:cgpt_config_file]
  endif
  let l:command += ['-O', '/tmp/cgpt-vim.hist', '-show-spinner', 'false']

  " print out the l:command :

  let s:job_id = job_start(l:command, {
        \ 'in_io': 'pipe',
        \ 'out_io': 'pipe',
        \ 'err_io': 'pipe',
        \ 'out_cb': function('s:handle_output'),
        \ 'err_cb': function('s:handle_output'),
        \ 'exit_cb': function('s:job_exit'),
        \ 'noblock': 1,
        \ 'timeout': g:cgpt_timeout * 1000
        \ })

  if job_status(s:job_id) == "fail"
    echoerr "Failed to start cgpt job. Make sure cgpt is installed and properly configured."
    return
  endif

  " Send the input to cgpt
  call ch_sendraw(job_getchannel(s:job_id), l:input . "\n")
  call ch_close_in(job_getchannel(s:job_id))
endfunction

" Function to cancel the ongoing cgpt job
function! CancelCgpt()
  if exists('s:job_id') && job_status(s:job_id) == "run"
    call job_stop(s:job_id)
    echo "Cgpt job cancelled."
  else
    echo "No active cgpt job to cancel."
  endif
endfunction
" Function to set surrounding context
function! SetCgptSurroundingContext(lines)
  let g:cgpt_include_surrounding_context = a:lines
  if a:lines == 0
    echo "Cgpt will now include the whole file as context."
  elseif a:lines > 0
    echo "Cgpt will now include " . a:lines . " lines of surrounding context."
  else
    echo "Invalid input. Please use 0 for whole file or a positive number for surrounding lines."
  endif
endfunction
" Function to set selected text position
function! SetCgptSelectedTextPosition(position)
  if a:position ==? 'beginning' || a:position ==? 'end'
    let g:cgpt_selected_text_position = tolower(a:position)
    echo "Cgpt selected text position set to: " . g:cgpt_selected_text_position
  else
    echo "Invalid position. Use 'beginning' or 'end'."
  endif
endfunction
" Commands
"
command! -range CgptRun <line1>,<line2>call RunCgpt()
command! CgptCancel call CancelCgpt()
command! -nargs=1 CgptSetContext call SetCgptSurroundingContext(<args>)
command! -nargs=1 CgptSetSelectedTextPosition call SetCgptSelectedTextPosition(<q-args>)

" Map the visual selection command to 'cg'
vnoremap <silent> cg :call RunCgpt()<CR>
