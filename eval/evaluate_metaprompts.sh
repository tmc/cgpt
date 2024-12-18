#!/bin/bash
set -euo pipefail

# Configuration paths
EVAL_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
EVAL_CONFIG="${EVAL_DIR}/devin_eval.yaml"
METAPROMPTS_CONFIG="${EVAL_DIR}/metaprompts.yaml"
TIMEOUT=60

DEBUG=true
log_debug() {
    if [ "${DEBUG}" = true ]; then
        echo "[DEBUG] $1" >&2
    fi
}

handle_error() {
    echo "Error occurred in script at line $1" >&2
    exit 1
}
trap 'handle_error ${LINENO}' ERR

cat > "${EVAL_DIR}/parse_yaml.py" << 'EOL'
import yaml
import sys

def load_yaml(file_path):
    with open(file_path, 'r') as f:
        return yaml.safe_load(f)

if __name__ == '__main__':
    config_file = sys.argv[1]
    query = sys.argv[2]

    data = load_yaml(config_file)
    if query == 'tasks':
        for task in data['tasks']:
            print(task['name'])
    elif query == 'metaprompts':
        for prompt in data['metaprompts']:
            print(prompt['prompt'])
EOL

chmod +x "${EVAL_DIR}/parse_yaml.py"

while IFS= read -r metaprompt; do
    log_debug "Processing metaprompt: ${metaprompt}"

    while IFS= read -r task; do
        log_debug "Processing task: ${task}"
        echo "Processing task: ${task} with metaprompt template"

        mkdir -p "${EVAL_DIR}/results"

        if [ -f "${EVAL_DIR}/test_cases/${task}.txt" ]; then
            log_debug "Found test case file: ${EVAL_DIR}/test_cases/${task}.txt"

            test_case=$(cat "${EVAL_DIR}/test_cases/${task}.txt")
            log_debug "Test case content: ${test_case}"

            output_file="${EVAL_DIR}/results/${task}_${metaprompt// /_}.txt"
            log_debug "Writing output to: ${output_file}"

            timeout ${TIMEOUT} cgpt -s "${metaprompt}" -i "${test_case}" > "${output_file}" 2>/dev/null || {
                echo "Warning: cgpt command timed out or failed for task ${task}" >&2
                echo "EVALUATION_FAILED: Timeout or error occurred" > "${output_file}"
            }

            log_debug "Completed evaluation for task: ${task}"
        else
            echo "Warning: Test case file ${EVAL_DIR}/test_cases/${task}.txt not found"
        fi

    done < <(python3 "${EVAL_DIR}/parse_yaml.py" "${EVAL_CONFIG}" "tasks")
done < <(python3 "${EVAL_DIR}/parse_yaml.py" "${METAPROMPTS_CONFIG}" "metaprompts")

echo "Evaluation complete. Results are in ${EVAL_DIR}/results/"
