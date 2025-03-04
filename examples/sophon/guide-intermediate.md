# Sophon Agent Intermediate Guide

This guide covers intermediate techniques for using the Sophon agent effectively in software development projects.

## Understanding the Agent Cycle

Sophon operates through an iterative cycle:

1. **Initialization**: Setting up context and goals
2. **Planning**: Analyzing requirements and creating an implementation plan
3. **Implementation**: Generating code using txtar format
4. **Application**: Applying changes to the project files
5. **Repetition**: Continuing the cycle until completion

Understanding this cycle helps you effectively guide and interact with the agent.

## Crafting Effective Instructions

The quality of your instructions significantly impacts agent performance:

### Good Instruction Examples

```
Implement a REST API with the following endpoints:
1. GET /users - Returns a list of users from a JSON file
2. GET /users/{id} - Returns a specific user by ID
3. POST /users - Creates a new user
4. PUT /users/{id} - Updates a user
5. DELETE /users/{id} - Deletes a user

Error handling should include appropriate status codes and validation.
```

### Tips for Better Instructions

1. **Be Specific**: Clearly define requirements and expected behavior
2. **Provide Structure**: Break down complex tasks into steps
3. **Include Examples**: Show expected inputs and outputs
4. **Define Boundaries**: Clarify what's in scope and out of scope
5. **Set Priorities**: Indicate which aspects are most important

## Customizing System Prompts

Create specialized agents by customizing the system prompt:

```yaml
backend: anthropic
messages:
- role: system
  text: |-
    You are Sophon, an expert in developing microservice architectures with Go.
    Your specializations include:
    
    1. Domain-Driven Design
    2. CQRS patterns
    3. Event-driven architecture
    4. Containerization with Docker
    5. Kubernetes deployment
    
    When implementing solutions:
    - Use standard Go project layouts
    - Implement proper error handling
    - Follow clean architecture principles
    - Include appropriate tests
    - Consider performance and scalability
    
    [Additional system prompt content...]
```

Place this content in `.h-sophon-agent-init` before initialization.

## Advanced txtar Usage

### Multi-file Projects

```
-- project/main.go --
package main

import (
    "fmt"
    "project/config"
)

func main() {
    cfg := config.Load()
    fmt.Println("Config loaded:", cfg.Name)
}

-- project/config/config.go --
package config

type Config struct {
    Name string
}

func Load() Config {
    return Config{
        Name: "Example Configuration",
    }
}

-- project/go.mod --
module project

go 1.18
```

### Managing Nested Content

For content containing txtar-like formatting, use escaping:

```
-- template.txt --
This template contains an example:

\-- example.txt --
Example content
\--
```

## Managing Long-running Projects

For complex projects:

1. **Directory Segmentation**: Break projects into subdirectories for focused development
2. **Checkpoint Commits**: Make git commits after successful iterations
3. **Staged Implementation**: Implement features in logical sequences
4. **Configuration Management**: Create consistent config files for different components

Example workflow:

```bash
# Initial setup
mkdir -p my-project/{frontend,backend,docs}
cd my-project

# Backend implementation
cd backend
../../sophon-agent-init.sh "Implement the backend API according to ../requirements/backend.txt"
../../sophon-agent.sh
git add .
git commit -m "Implement backend API"

# Frontend implementation
cd ../frontend
../../sophon-agent-init.sh "Implement the frontend according to ../requirements/frontend.txt"
../../sophon-agent.sh
git add .
git commit -m "Implement frontend"

# Integration
cd ..
../sophon-agent-init.sh "Integrate frontend and backend according to requirements/integration.txt"
../sophon-agent.sh
```

## Troubleshooting and Debugging

### Common Issues

1. **txtar Parsing Errors**:
   - Check for missing or malformed file headers
   - Look for unescaped txtar-like content

2. **Context Limitations**:
   - Break large projects into smaller, focused tasks
   - Use configuration files and documentation for shared context

3. **Inconsistent Output**:
   - Provide clearer instructions
   - Consider using a more specialized system prompt
   - Start with basic structure for the agent to build upon

4. **File Path Issues**:
   - Ensure all file paths are correctly specified
   - Use consistent path formats

### Debugging Techniques

1. **Examine Agent Files**:
   - Check the `.agent-instructions` file
   - Review the history file `.h-mk` to see agent responses
   - Look for patterns in issues

2. **Incremental Testing**:
   - Apply changes manually and test
   - Implement one feature at a time
   - Verify each component works before proceeding

## Integration with Development Workflows

### CI/CD Integration

Sample GitHub Actions workflow:

```yaml
name: Sophon Agent

on:
  issues:
    types: [opened, edited]

jobs:
  sophon-agent:
    if: contains(github.event.issue.body, '/sophon')
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v2
      
      - name: Setup Sophon
        run: |
          # Install dependencies
          apt-get update && apt-get install -y txtar yq
          
      - name: Parse requirements
        run: |
          echo "${{ github.event.issue.body }}" | sed -n '/```requirements/,/```/p' | sed '1d;$d' > requirements.txt
          
      - name: Run Sophon
        run: |
          ./scripts/sophon-agent-init.sh "Implement requirements from requirements.txt"
          ./scripts/sophon-agent.sh
          
      - name: Create PR
        uses: peter-evans/create-pull-request@v3
        with:
          title: "Sophon implementation: ${{ github.event.issue.title }}"
          body: "Implements #${{ github.event.issue.number }}"
          branch: "sophon/${{ github.event.issue.number }}"
```

## Advanced Examples

See the `examples/sophon/` directory for more advanced examples including:

1. Multi-service applications
2. Testing and CI integration
3. Complex refactoring
4. API development
5. Code generation scenarios

Each example demonstrates different Sophon capabilities and provides templates you can adapt for your projects.