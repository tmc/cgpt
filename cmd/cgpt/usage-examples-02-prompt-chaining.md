# Meta-Prompting: chaining

```shell
# Generate a program to extract and make executable the inferred prompt
$ echo "Create a bash script that extracts the content of <inferred-prompt> tags from stdin and makes it executable" | cgpt -s "You are an expert bash programmer. Write a concise, efficient bash script that does the following:

1. Reads input from stdin
2. Extracts the content between <inferred-prompt> and </inferred-prompt> tags
3. Outputs ONLY the extracted content, with no additional text
4. Ensures the output is directly executable (i.e., can be piped to 'bash' or saved and executed)

The script should handle potential edge cases, such as multiple occurrences of the tags or nested tags. It should output nothing if no valid tags are found.

Output only the bash script, with no additional explanation or comments." --prefill "#!/usr/bin/env bash

"
```

```shell
# Generate a program that enhances a simple cgpt invocation
$ echo 'Create a bash script that takes a simple cgpt command as input and outputs an enhanced version' | cgpt -s "You are an expert in both bash programming and AI prompt engineering. Write a bash script that does the following:

1. Reads a simple cgpt command from stdin, typically in the form of 'echo \"query\" | cgpt'
2. Enhances the command by:
   a. Adding a relevant system prompt (-s) based on the query
   b. Selecting an appropriate model (-m) if not specified
   c. Adding a reasonable token limit (-t)
   d. Setting an appropriate temperature (-T)
   e. Incorporating other relevant flags or techniques that could improve the response

3. Outputs the enhanced command as a single line, executable bash command

The script should significantly improve the effectiveness of the original command while keeping its core intent. Handle potential input variations gracefully.

Use the following cgpt help output for reference:
<cgpt-help-output>$(cgpt --help 2>&1)</cgpt-help-output>

Output only the bash script, with no additional explanation or comments." --prefill "#!/usr/bin/env bash

"
