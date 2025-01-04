#!/bin/bash
# code-to-gpt.sh
# This script preps content to feed into a large language model.
# Read and print contents of text files in a directory, including untracked files by default

set -euo pipefail

# Usage function
DIRECTORY="."
EXCLUDE_DIRS=("node_modules" "venv" ".venv")
IGNORED_FILES=("go.sum" "go.work.sum" "yarn.lock" "yarn.error.log" "package-lock.json")
COUNT_TOKENS=false
VERBOSE=false
USE_XML_TAGS=true
INCLUDE_SVG=false
INCLUDE_XML=false
WC_LIMIT=10000
TRACKED_ONLY="${TRACKED_ONLY:-false}"

root_path=""

# Function to print usage information
print_usage() {
    echo "Usage: $0 [--count-tokens] [--exclude-dir <dir>] [--verbose] [--no-xml-tags] [--exclude-svg] [--exclude-xml] [--include-svg] [--include-xml] [--tracked-only] [<directory>]"
    echo "  --count-tokens: Count tokens instead of outputting file contents"
    echo "  --exclude-dir <dir>: Add a directory to the list of directories to exclude"
    echo "  --verbose: Enable verbose output"
    echo "  --no-xml-tags: Disable XML tags around content"
    echo "  --exclude-svg: Exclude SVG files from processing (default behavior)"
    echo "  --exclude-xml: Exclude XML files from processing (default behavior)"
    echo "  --include-svg: Explicitly include SVG files"
    echo "  --include-xml: Explicitly include XML files"
    echo "  --tracked-only: Only include tracked files in Git repositories"
    echo "  <directory>: Specify the directory to process (default: current directory)"
}

# Parse command-line arguments
while [[ $# -gt 0 ]]; do
    case $1 in
        --count-tokens)
            COUNT_TOKENS=true
            shift
            ;;
        --exclude-dir)
            EXCLUDE_DIRS+=("$2")
            shift 2
            ;;
        --verbose)
            VERBOSE=true
            shift
            ;;
        --no-xml-tags)
            USE_XML_TAGS=false
            shift
            ;;
        --exclude-svg)
            INCLUDE_SVG=false
            shift
            ;;
        --exclude-xml)
            INCLUDE_XML=false
            shift
            ;;
        --include-svg)
            INCLUDE_SVG=true
            shift
            ;;
        --include-xml)
            INCLUDE_XML=true
            shift
            ;;
        --tracked-only)
            TRACKED_ONLY=true
            shift
            ;;
        -h|--help)
            print_usage
            exit 0
            ;;
        *)
            DIRECTORY="$1"
            shift
            ;;
    esac
done

# Function to check if a directory should be excluded
is_excluded_dir() {
    local dir="$1"
    for excluded in "${EXCLUDE_DIRS[@]}"; do
        if [[ "$dir" == *"/$excluded"* ]]; then
            return 0
        fi
    done
    return 1
}

# Function to check if a file should be ignored
is_ignored_file() {
    local file="$1"
    for ignored in "${IGNORED_FILES[@]}"; do
        if [[ "$(basename "$file")" == "$ignored" ]]; then
            return 0
        fi
    done
    return 1
}

# Function to get relative path to a directory from home directory with tilde
get_home_relative_dirpath() {
    local dir="$1"
    local abs_path=$(cd "$dir" && pwd)
    local home_path=$HOME
    if [[ "$abs_path" == "$home_path" ]]; then
        echo "~"
    elif [[ "$abs_path" == "$home_path"/* ]]; then
        echo "~/${abs_path#$home_path/}"
    else
        echo "$abs_path"
    fi
}

get_home_relative_filepath() {
    local file="$1"
    local abs_path=$(realpath "$file")
    local home_path=$HOME
    if [[ "$abs_path" == "$home_path" ]]; then
        echo "~"
    elif [[ "$abs_path" == "$home_path"/* ]]; then
        echo "~/${abs_path#$home_path/}"
    else
        echo "$abs_path"
    fi
}

realpath() {
    [[ $1 = /* ]] && echo "$1" || echo "$PWD/${1#./}"
}

get_relative_path() {
    local file="$1"
    local root_path="$2"

    # Expand the tilde in root_path if present
    root_path="${root_path/#\~/$HOME}"

    local abs_file=$(realpath "$file")
    local rel_path="${abs_file#$root_path/}"

    if [ "$rel_path" = "$abs_file" ]; then
        echo "$abs_file"
    else
        echo "./$rel_path"
    fi
}

# Function to check if a file is a text file and should be included
is_text_file() {
    local file="$1"
    local mime_type=$(file -b --mime-type "$file")
    local extension="${file##*.}"

    if [[ "$mime_type" == text/* ]]; then
        return 0
    elif [[ "$mime_type" == application/x-empty ]] || [[ "$mime_type" == inode/x-empty ]]; then
        return 0
    elif [[ "$mime_type" == image/svg+xml ]] && $INCLUDE_SVG; then
        return 0
    elif [[ "$mime_type" == application/xml ]] && $INCLUDE_XML; then
        return 0
    else
        return 1
    fi
}

# Function to process a file
process_file() {
    local file="$1"
    local expanded_root_path="${root_path/#\~/$HOME}"
    local relative_path=$(get_relative_path "$file" "$expanded_root_path")
    local mime_type=$(file -b --mime-type "$file")
    local line_count=$(wc -l < "$file")

    if ! is_text_file "$file"; then
        if $VERBOSE; then
            echo "Skipping non-text file: $relative_path (MIME: $mime_type)" >&2
        fi
        return
    fi

    if [ "$line_count" -gt "$WC_LIMIT" ]; then
        if $VERBOSE; then
            echo "Skipping large file: $relative_path ($line_count lines)" >&2
        fi
        return
    fi

    if $VERBOSE; then
        echo "Processing file: $relative_path (MIME: $mime_type)" >&2
    fi

    if $USE_XML_TAGS; then
        echo "<file path=\"$relative_path\">"
    fi

    if $COUNT_TOKENS; then
        token_count=$(tokencount "$file")
        echo "$token_count $relative_path"
    else
        cat "$file"
    fi

    if $USE_XML_TAGS; then
        echo "</file>"
    fi
}

# Force off the use of XML tags if counting tokens:
if $COUNT_TOKENS; then
    USE_XML_TAGS=false
fi

# Main processing logic
if $USE_XML_TAGS; then
    root_path=$(get_home_relative_dirpath "$DIRECTORY")
    echo "<root path=\"$root_path\">"
fi

if [ -d "$DIRECTORY/.git" ]; then
    if $VERBOSE; then
        echo "Git repository detected. Using git commands." >&2
    fi
    cd "$DIRECTORY" || exit 1
    if $TRACKED_ONLY; then
        if $VERBOSE; then
            echo "Processing only tracked files." >&2
        fi
        git ls-files | while read -r file; do
            if [ -f "$file" ] && ! is_ignored_file "$file" && ! is_excluded_dir "$(dirname "$file")"; then
                process_file "$DIRECTORY/$file"
            fi
        done
    else
        if $VERBOSE; then
            echo "Processing all files, including untracked." >&2
        fi
        { git ls-files; git ls-files --others --exclude-standard; } | sort -u | while read -r file; do
            if [ -f "$file" ] && ! is_ignored_file "$file" && ! is_excluded_dir "$(dirname "$file")"; then
                process_file "$DIRECTORY/$file"
            fi
        done
    fi
else
    if $VERBOSE; then
        echo "No Git repository detected. Using find command." >&2
    fi
    find "$DIRECTORY" -type f | while read -r file; do
        if ! is_ignored_file "$file" && ! is_excluded_dir "$(dirname "$file")"; then
            process_file "$file"
        fi
    done
fi

if $USE_XML_TAGS; then
    echo "</root>"
fi
