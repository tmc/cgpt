# sb-exec: Docker Execution with Git Notes Integration

sb-exec is a utility script that executes commands in Docker containers and automatically attaches the execution output as git notes metadata. This provides a clean way to document runs, experiments, and tests directly within your git history.

## Features

- Run commands in isolated Docker containers
- Mount local directories for data access
- Capture complete execution output
- Automatically attach output as git notes for later reference
- Customizable Docker images, working directories, and note tags
- Clean, formatted output with timestamps

## Requirements

- Docker
- Git
- Bash

## Installation

Simply download the script and make it executable:

```bash
curl -O https://raw.githubusercontent.com/tmc/cgpt/examples/sophon/sb-exec.sh
chmod +x sb-exec.sh
```

## Usage

```
./sb-exec.sh [options] <command>

Options:
  -i, --image IMAGE     Docker image to use (default: ubuntu:latest)
  -w, --workdir DIR     Working directory inside container (default: /workspace)
  -v, --volume DIR      Mount local directory (default: current directory)
  -t, --tag TAG         Git note tag name (default: sb-exec)
  -m, --message MSG     Message to include with git note
  -n, --no-notes        Don't create git notes, just execute command
  -h, --help            Show this help message
```

## Examples

### Basic Usage

Run a simple command in the default Ubuntu container:

```bash
./sb-exec.sh "ls -la"
```

### Using a Specific Docker Image

Run Python commands in a Python container:

```bash
./sb-exec.sh --image python:3.9 "python -m pip list"
```

### Performance Testing

Run a benchmark and tag the notes for easy filtering:

```bash
./sb-exec.sh --tag "performance-test" "time python benchmark.py"
```

### Running Tests

Execute tests in a container with all dependencies:

```bash
./sb-exec.sh --image myproject-dev "go test ./..."
```

### Processing Large Datasets

Process data with proper volume mounting:

```bash
./sb-exec.sh --volume /path/to/data:/data --workdir /data "python process_data.py large_file.csv"
```

## Working with Git Notes

Git notes allow you to attach metadata to commits without changing the commit itself. 

### Viewing Notes

View notes for the current commit:

```bash
git notes --ref=sb-exec show
```

View notes for a specific commit:

```bash
git notes --ref=sb-exec show <commit-hash>
```

### Pushing/Pulling Notes

Push notes to a remote repository:

```bash
git push origin refs/notes/sb-exec
```

Pull notes from a remote repository:

```bash
git fetch origin refs/notes/sb-exec:refs/notes/sb-exec
```

## Integration Tips

### CI/CD Integration

You can use sb-exec in CI/CD pipelines to document builds and tests:

```yaml
# Example GitHub Actions workflow step
- name: Run tests in container
  run: |
    ./sb-exec.sh --image project-tests --tag "ci-test-run" "pytest -xvs tests/"
```

### Experiment Tracking

Track machine learning experiments by tagging each run:

```bash
./sb-exec.sh --image tensorflow/tensorflow:latest-gpu --tag "experiment-$RUN_ID" "python train_model.py --epochs 100"
```

## Customization

You can modify the script to add more features such as:

- Environment variable passing
- Additional Docker options
- Custom output formatting
- Integration with other metadata systems

## License

Same as the cgpt project.