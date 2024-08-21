# Example creation/population

```shell
# Generate new usage examples
$ echo "Output a new sample shell one-liner that uses cgpt to [specific task or use case]" | cgpt -s "You are an expert in creating concise, practical command-line examples. Output a single line of shell code demonstrating an innovative use of cgpt, incorporating useful options and techniques from existing examples. Use creative tips from the examples and cgpt help output. Include the <cgpt-help-output> technique for context. Prefix the command with a # comment briefly explaining its purpose. The cgpt help output is: <cgpt-help-output>$(cgpt --help 2>&1)</cgpt-help-output>"
```
