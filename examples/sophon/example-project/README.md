# Weather Data Analyzer

This example project demonstrates how to use the Sophon agent to implement a weather data analysis tool.

## Project Overview

The goal is to create a Python command-line tool that analyzes weather data from CSV files, calculates statistics, and generates reports.

## Getting Started

1. Review the requirements in `requirements.txt`
2. Examine the sample data in `sample_data.csv`
3. Initialize the Sophon agent:

```bash
cd examples/sophon/example-project
../sophon-agent-init.sh "Implement a weather data analysis tool according to the requirements.txt file"
```

4. Run the Sophon agent to implement the solution:

```bash
../sophon-agent.sh
```

## Project Structure

- `requirements.txt`: Project requirements specification
- `sample_data.csv`: Example weather data for testing
- `.agent-instructions`: Instructions for the Sophon agent
- `.h-sophon-agent-init`: System prompt for the agent

## Expected Implementation

The Sophon agent should create:

1. A main Python script for the command-line interface
2. Modules for data validation, statistics, and reporting
3. Unit tests for core functionality
4. Documentation

## Running the Tool

Once implemented, you can run the tool with:

```bash
python weather_analyzer.py --input sample_data.csv --output report.txt --format text --stats all
```

## Learning from This Example

This example demonstrates:

1. How the Sophon agent interprets and implements requirements
2. The iterative development process through agent cycles
3. How txtar format is used to specify file changes
4. Best practices for Python development