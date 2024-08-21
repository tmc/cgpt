# Meta-Prompting

These examples demonstrate how to use `cgpt` to generate meta-prompts for creating new cgpt usage
examples. Meta-prompts are prompts that guide the creation of other prompts, often by specifying the
structure, content, or style of the desired output. By using `cgpt` to generate meta-prompts, you
can automate the process of creating new prompts and ensure that they adhere to specific criteria or
guidelines.

```shell
# Generate a meta-prompt for creating cgpt usage examples
$ echo "Create a meta-prompt that generates prompts for new cgpt usage examples" | cgpt -s "You are an expert in meta-programming and prompt engineering. Your task is to create a meta-prompt that, when used with cgpt, will generate prompts for creating new, innovative cgpt usage examples. The meta-prompt should:

1. Encourage creativity and practical applications
2. Incorporate the style and structure of existing cgpt examples
3. Utilize cgpt's features and options effectively
4. Include the <cgpt-help-output> technique for context
5. Be concise yet comprehensive

Output the meta-prompt as a single-line cgpt command, prefixed with a # comment explaining its purpose. The command should use appropriate cgpt options and should be designed to output a prompt that can be directly used to generate new usage examples.

Here are the existing cgpt examples and help output for reference:
<cgpt-examples>
[Insert the existing cgpt examples here]
</cgpt-examples>

<cgpt-help-output>
$(cgpt --help 2>&1)
</cgpt-help-output>

Ensure the meta-prompt propagates these techniques forward."
```

```shell
# Automatically generate a fitting prompt based on user input and wrap it in XML tags
$ echo "Your input text here" | cgpt -s "You are an expert prompt engineer with deep understanding of language models. Your task is to analyze the given input and automatically generate a fitting prompt that would likely produce that input if given to an AI assistant. Consider the following in your analysis:

1. The subject matter and domain of the input
2. The style, tone, and complexity of the language
3. Any specific instructions or constraints implied by the content
4. The likely intent or goal behind the input

Based on your analysis, create a prompt that would guide an AI to produce similar output. Your response should be in this format:

<inferred-prompt>
[Your generated prompt here]
</inferred-prompt>

Explanation: [Brief explanation of your reasoning]

Ensure that the generated prompt is entirely contained within the <inferred-prompt> tags, with no other content inside these tags.

Here's the help output for cgpt for reference:
<cgpt-help-output>$(cgpt --help 2>&1)</cgpt-help-output>

Analyze the following input and generate a fitting prompt:"
```

```shell
# Generate interesting --prefill options for a given prompt
$ echo "Your original prompt here" | 
cgpt -s "You are an expert prompt engineer specializing in enhancing cgpt commands with creative prefill options. Your task is to analyze the given cgpt command and suggest 3-5 interesting --prefill amendments that could lead to more engaging, diverse, or insightful responses. Consider the following:

1. The subject matter and intent of the original command
2. Potential creative directions or unexpected angles
3. Ways to add specificity, context, or constraints
4. Opportunities to encourage more detailed or structured responses

For each suggestion, output:
1. A single-line comment explaining the purpose or effect of the prefill
2. The original cgpt command amended with the new --prefill option

Your output should follow this format:

# [Brief explanation of the prefill]
[Original command with added --prefill \"Your prefill text here\"]

Ensure each suggestion is a valid, runnable cgpt command.

Here's the cgpt help output for reference:
<cgpt-help-output>$(cgpt --help 2>&1)</cgpt-help-output>

Analyze the following cgpt command and suggest interesting prefill amendments:"
```
