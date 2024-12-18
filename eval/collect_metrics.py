import yaml
import json
from pathlib import Path
import time
from typing import Dict, Any, List, Tuple
import re
from dataclasses import dataclass
from enum import Enum

class ComplexityLevel(Enum):
    LOW = "low"
    MEDIUM = "medium"
    HIGH = "high"

@dataclass
class TaskMetrics:
    complexity: float
    readability: float
    documentation: float
    task_completion: float
    response_coherence: float
    time_to_completion: float
    success_criteria_met: List[str]

def evaluate_code_quality(code: str) -> Dict[str, float]:
    metrics = {
        'complexity': 0.0,
        'readability': 0.0,
        'documentation': 0.0
    }

    nested_depth = len(re.findall(r'(\s+)(if|for|while)', code))
    function_count = len(re.findall(r'(def|func)\s+\w+', code))
    class_count = len(re.findall(r'class\s+\w+', code))

    complexity_score = (
        (nested_depth * 0.4) +
        (function_count * 0.3) +
        (class_count * 0.3)
    ) / 10.0
    metrics['complexity'] = min(1.0, complexity_score)

    lines = code.split('\n')
    long_lines = sum(1 for line in lines if len(line.strip()) > 80)
    empty_lines = sum(1 for line in lines if not line.strip())
    proper_spacing = sum(1 for line in lines if re.match(r'^\s*(def|class|if|for|while)', line))

    readability_score = (
        (1.0 - (long_lines / max(1, len(lines)))) * 0.4 +
        (empty_lines / max(1, len(lines))) * 0.3 +
        (proper_spacing / max(1, len(lines))) * 0.3
    )
    metrics['readability'] = readability_score

    doc_lines = len(re.findall(r'(#|"""|\'\'\')', code))
    function_docs = len(re.findall(r'"""[\s\S]*?"""', code))
    class_docs = len(re.findall(r'class.*?:\s*"""[\s\S]*?"""', code))

    documentation_score = (
        (doc_lines / max(1, len(lines))) * 0.4 +
        (function_docs / max(1, function_count)) * 0.3 +
        (class_docs / max(1, class_count)) * 0.3
    )
    metrics['documentation'] = min(1.0, documentation_score)

    return metrics

def evaluate_task_completion(response: str, task_file: Path) -> Tuple[float, List[str]]:
    with open(task_file) as f:
        content = f.read()
        success_criteria = re.findall(r'Success Criteria:\n-(.*?)(?:\n\n|$)', content, re.DOTALL)
        criteria_list = [c.strip() for c in success_criteria[0].split('\n-') if c.strip()]

    completion_indicators = [
        'completed', 'finished', 'done',
        'successfully', 'implemented', 'fixed'
    ]

    score = 0.0
    met_criteria = []

    for indicator in completion_indicators:
        if indicator in response.lower():
            score += 0.2

    for criterion in criteria_list:
        if any(part.lower() in response.lower() for part in criterion.split()):
            score += 0.3
            met_criteria.append(criterion)

    return min(1.0, score), met_criteria

def evaluate_response_coherence(response: str) -> float:
    structure_elements = {
        'thoughts': '# Thoughts' in response,
        'steps': any(f'step {i}' in response.lower() for i in range(10)),
        'explanation': any(keyword in response.lower() for keyword in ['because', 'therefore', 'since']),
        'conclusion': any(keyword in response.lower() for keyword in ['finally', 'in conclusion', 'to summarize'])
    }

    return sum(1 for v in structure_elements.values() if v) / len(structure_elements)

def evaluate_response(response: str, task_file: Path, start_time: float) -> TaskMetrics:
    code_blocks = re.findall(r'```[\w]*\n(.*?)```', response, re.DOTALL)
    code = '\n'.join(code_blocks) if code_blocks else ''

    code_quality = evaluate_code_quality(code) if code else {'complexity': 0.0, 'readability': 0.0, 'documentation': 0.0}
    task_completion_score, met_criteria = evaluate_task_completion(response, task_file)

    return TaskMetrics(
        complexity=code_quality['complexity'],
        readability=code_quality['readability'],
        documentation=code_quality['documentation'],
        task_completion=task_completion_score,
        response_coherence=evaluate_response_coherence(response),
        time_to_completion=time.time() - start_time,
        success_criteria_met=met_criteria
    )

def collect_metrics() -> Dict[str, Any]:
    results_dir = Path("results")
    metrics = {}

    for result_file in results_dir.glob("*.txt"):
        task_name = result_file.stem.split("_")[0]
        metaprompt_name = result_file.stem.split("_")[1]

        with open(result_file) as f:
            response = f.read()
            start_time = result_file.stat().st_mtime
            metrics[f"{task_name}_{metaprompt_name}"] = evaluate_response(response, result_file, start_time)

    with open('evaluation_results.json', 'w') as f:
        json.dump(metrics, f, indent=2)

    return metrics

if __name__ == '__main__':
    metrics = collect_metrics()
    print(json.dumps(metrics, indent=2))
