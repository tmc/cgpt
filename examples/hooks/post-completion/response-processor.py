#!/usr/bin/env python3
"""
Post-completion hook to process and analyze the response
This hook demonstrates response analysis and logging
"""

import json
import sys
import re
import os
from datetime import datetime

def analyze_response(context):
    """Analyze the response and extract insights."""
    try:
        response = context.get('response', '')
        messages = context.get('messages', [])
        config = context.get('config', {})

        # Count response characteristics
        word_count = len(response.split())
        line_count = len(response.split('\n'))
        code_blocks = len(re.findall(r'```[\s\S]*?```', response))

        # Detect language/framework mentions
        languages = []
        frameworks = []

        lang_patterns = {
            'python': r'\b(python|py)\b',
            'javascript': r'\b(javascript|js|node)\b',
            'go': r'\b(go|golang)\b',
            'rust': r'\b(rust|cargo)\b',
            'java': r'\b(java|jvm)\b',
            'cpp': r'\b(c\+\+|cpp)\b',
        }

        framework_patterns = {
            'react': r'\b(react|jsx)\b',
            'django': r'\b(django)\b',
            'flask': r'\b(flask)\b',
            'gin': r'\b(gin)\b',
            'express': r'\b(express)\b',
        }

        for lang, pattern in lang_patterns.items():
            if re.search(pattern, response, re.IGNORECASE):
                languages.append(lang)

        for fw, pattern in framework_patterns.items():
            if re.search(pattern, response, re.IGNORECASE):
                frameworks.append(fw)

        return {
            'word_count': word_count,
            'line_count': line_count,
            'code_blocks': code_blocks,
            'languages': languages,
            'frameworks': frameworks,
            'backend': config.get('backend', 'unknown'),
            'model': config.get('model', 'unknown'),
            'timestamp': context.get('timestamp', ''),
        }

    except Exception as e:
        print(f"Error analyzing response: {e}", file=sys.stderr)
        return None

def log_analysis(analysis):
    """Log the analysis results."""
    if not analysis:
        return

    # Create analysis log directory
    log_dir = os.path.expanduser('~/.cgpt/analysis')
    os.makedirs(log_dir, exist_ok=True)

    # Log to daily file
    today = datetime.now().strftime('%Y-%m-%d')
    log_file = os.path.join(log_dir, f'responses_{today}.jsonl')

    try:
        with open(log_file, 'a') as f:
            f.write(json.dumps(analysis) + '\n')
    except Exception as e:
        print(f"Error writing analysis log: {e}", file=sys.stderr)

def main():
    """Main hook execution."""
    try:
        # Read context from stdin
        context = json.load(sys.stdin)

        # Analyze the response
        analysis = analyze_response(context)

        if analysis:
            # Log analysis
            log_analysis(analysis)

            # Print summary to stderr (visible to user)
            print(f"Response analysis: {analysis['word_count']} words, "
                  f"{analysis['code_blocks']} code blocks", file=sys.stderr)

            if analysis['languages']:
                print(f"Languages detected: {', '.join(analysis['languages'])}", file=sys.stderr)

            if analysis['frameworks']:
                print(f"Frameworks detected: {', '.join(analysis['frameworks'])}", file=sys.stderr)

        # Exit successfully
        sys.exit(0)

    except Exception as e:
        print(f"Hook error: {e}", file=sys.stderr)
        sys.exit(1)

if __name__ == '__main__':
    main()