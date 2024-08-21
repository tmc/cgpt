More Advanced Examples:
	# Generate a shell script using AI assistance
	$ echo "Write a script that analyzes the current local git branch, the recent activity, and suggest a meta-learning for making more effective progress." \
	     | cgpt --system-prompt "You are a self-improving unix toolchain assistant. Output a program that uses AI to the goals of the user. The current system is $(uname). The help for cgpt is <cgpt-help-output>$(cgpt --help 2>&1). Your output should be only valid bash. If you have commentary make sure it is prefixed with comment characters." \
     --prefill "#!/usr/bin/env" | tee suggest-process-improvement.sh

	# Analyze research notes with a high token limit
	$ cgpt -f research_notes.txt -s "You are a research assistant. Summarize the key points and suggest follow-up questions."

	# Analyze git commit history
	$ git log --oneline | cgpt -s "You are a git commit analyzer. Provide insights on commit patterns and suggest improvements."

	# Show even more advanced examples:
