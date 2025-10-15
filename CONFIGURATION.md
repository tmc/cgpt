# cgpt Configuration Reference

This document provides a complete reference for configuring cgpt, covering all configuration options, environment variables, example configurations, and backend-specific settings.

## Table of Contents

1. [Configuration Sources](#configuration-sources)
2. [Environment Variables](#environment-variables)
3. [Configuration File](#configuration-file)
4. [Command-Line Flags](#command-line-flags)
5. [Backend-Specific Configuration](#backend-specific-configuration)
6. [Example Configurations](#example-configurations)
7. [Advanced Configuration](#advanced-configuration)
8. [Configuration Troubleshooting](#configuration-troubleshooting)

## Configuration Sources

cgpt loads configuration from multiple sources in the following order of precedence (highest to lowest):

1. **Command-line flags** (highest priority)
2. **Environment variables**
3. **Configuration file**
4. **Default values** (lowest priority)

This allows you to set defaults in your config file, override them with environment variables for different environments, and use command-line flags for one-off changes.

## Environment Variables

### API Keys

Set your API keys for the AI providers you want to use:

```bash
# Anthropic (recommended default)
export ANTHROPIC_API_KEY="sk-ant-your-anthropic-key-here"

# OpenAI
export OPENAI_API_KEY="sk-your-openai-key-here"

# Google AI Studio
export GOOGLE_API_KEY="your-google-ai-key-here"

# OpenRouter (access to multiple providers)
export OPENROUTER_API_KEY="sk-or-your-openrouter-key-here"

# Alternative environment variable names (also supported)
export CGPT_ANTHROPIC_API_KEY="sk-ant-your-anthropic-key-here"
export CGPT_OPENAI_API_KEY="sk-your-openai-key-here"
export CGPT_GOOGLE_API_KEY="your-google-ai-key-here"
export CGPT_OPENROUTER_API_KEY="sk-or-your-openrouter-key-here"
```

### General Configuration

All configuration options can be set via environment variables with the `CGPT_` prefix:

```bash
# Core settings
export CGPT_BACKEND="anthropic"                    # AI provider
export CGPT_MODEL="claude-sonnet-4-20250514"       # AI model
export CGPT_MAX_TOKENS=8000                        # Response length limit
export CGPT_TEMPERATURE=0.05                       # Creativity (0.0-1.0)
export CGPT_STREAM=true                            # Enable streaming

# System behavior
export CGPT_SYSTEM_PROMPT="You are a helpful AI assistant."
export CGPT_DEBUG=false                            # Debug mode
export CGPT_VERBOSE=false                          # Verbose output

# Advanced features
export CGPT_PROMPT_CACHING=true                    # Cache prompts for cost savings
export CGPT_SHOW_USAGE=true                        # Display token usage
export CGPT_THINKING_MODE="medium"                 # Reasoning mode
export CGPT_THINKING_BUDGET=2048                   # Reasoning token budget
export CGPT_SHOW_REASONING=true                    # Show AI reasoning
export CGPT_INTERLEAVED_THINKING=false             # Advanced reasoning mode

# Timeouts and retries
export CGPT_COMPLETION_TIMEOUT="2m"                # API timeout
export CGPT_MAX_RETRIES=3                          # Retry attempts
export CGPT_RETRY_DELAY="1s"                       # Initial retry delay
export CGPT_DISABLE_RETRY=false                    # Disable all retries

# Configuration file location
export CGPT_CONFIG="~/.cgpt/config.yaml"           # Custom config path
```

### Boolean Environment Variables

For boolean values, you can use any of these formats:
- `true`, `false`
- `yes`, `no`
- `1`, `0`
- `on`, `off`

## Configuration File

cgpt looks for configuration files in these locations (in order):

1. Path specified by `--config` flag
2. Path specified by `CGPT_CONFIG` environment variable
3. `./config.yaml` (current directory)
4. `~/.cgpt/config.yaml`
5. `/etc/cgpt/config.yaml`

### Complete Configuration Example

```yaml
# config.yaml - Complete configuration example

# ============================================================================
# BASIC CONFIGURATION
# ============================================================================

# AI Provider and Model
backend: "anthropic"                          # anthropic, openai, googleai, ollama, openrouter
model: "claude-sonnet-4-20250514"             # Model identifier

# Response Configuration
maxTokens: 8000                               # Maximum response length
temperature: 0.05                             # Creativity: 0.0 (focused) to 1.0 (creative)
stream: true                                  # Enable streaming responses

# System Behavior
systemPrompt: "You are a helpful AI assistant specializing in programming and problem-solving."
debug: false                                  # Show detailed debug information
verbose: false                                # Show verbose output

# ============================================================================
# API KEYS (optional - can use environment variables)
# ============================================================================

anthropicAPIKey: "sk-ant-your-anthropic-key"
openaiAPIKey: "sk-your-openai-key"
googleAPIKey: "your-google-ai-key"
openrouterAPIKey: "sk-or-your-openrouter-key"

# ============================================================================
# ADVANCED AI FEATURES
# ============================================================================

# Prompt Caching (reduces costs for repeated prompts)
promptCaching: true

# Reasoning/Thinking Features (Claude 4+, o1+, Gemini 2.5+)
thinkingMode: "medium"                        # none, low, medium, high, auto
thinkingBudget: 2048                          # Explicit token budget (overrides thinkingMode)
showReasoning: true                           # Show AI's reasoning process
interleavedThinking: false                    # Advanced reasoning mode (Claude 4+ only)

# Usage Tracking
showUsage: true                               # Display token usage and costs

# ============================================================================
# NETWORK AND RELIABILITY
# ============================================================================

# Timeouts
completionTimeout: "2m"                       # Maximum wait for AI response

# Retry Configuration
maxRetries: 3                                 # Number of retry attempts
retryDelay: "1s"                              # Initial delay before retry (exponential backoff)
disableRetry: false                           # Disable all retry attempts

# ============================================================================
# BIAS AND FINE-TUNING (Advanced)
# ============================================================================

# Token bias (advanced feature - affects token probability)
logitBias:
  "yes": 0.1                                  # Slightly favor "yes"
  "no": -0.1                                  # Slightly disfavor "no"
  "python": 0.2                               # Favor Python-related tokens

# ============================================================================
# PROVIDER-SPECIFIC SETTINGS
# ============================================================================

# These settings only apply to specific providers
# Most users won't need to modify these

# OpenAI-specific
# openaiUseLegacyMaxTokens: false             # Use old max_tokens parameter

# Anthropic-specific settings are automatically configured
# Google AI settings are automatically configured
# Ollama settings are automatically configured
```

### Minimal Configuration Examples

#### Development Configuration
```yaml
# dev-config.yaml - For development work
backend: "anthropic"
model: "claude-sonnet-4-20250514"
maxTokens: 16000
temperature: 0.1
systemPrompt: "You are an expert software developer and code reviewer."
debug: true
verbose: true
showUsage: true
promptCaching: true
```

#### Production Configuration
```yaml
# prod-config.yaml - For production use
backend: "anthropic"
model: "claude-haiku-3-20240307"              # Faster, cheaper
maxTokens: 4000
temperature: 0.05
stream: true
debug: false
verbose: false
showUsage: false
promptCaching: true
maxRetries: 5
retryDelay: "2s"
```

#### Creative Writing Configuration
```yaml
# creative-config.yaml - For creative tasks
backend: "openai"
model: "gpt-4"
maxTokens: 8000
temperature: 0.8                              # More creative
systemPrompt: "You are a creative writing assistant with expertise in storytelling, character development, and prose."
stream: true
showUsage: true
```

## Command-Line Flags

All configuration options can be overridden using command-line flags. Here's a complete reference:

### Input/Output Flags
```bash
-i, --input STRING               # Direct text input (repeatable)
-f, --file PATH                  # Input file path, '-' for stdin (repeatable)
-p, --prefill STRING             # Start assistant's response with this text
-c, --continuous                 # Interactive mode
-v, --verbose                    # Verbose output
    --debug                      # Debug mode
```

### History & Sessions
```bash
-I, --history-in PATH            # Load conversation history from file
-O, --history-out PATH           # Save conversation history to file
-H, --history PATH               # Use same file for input/output ('auto' for timestamped)
-C, --continue                   # Continue most recent session
-n, --completions INT            # Number of completions for batch processing
```

### AI Model & Behavior
```bash
-b, --backend STRING             # AI provider (anthropic, openai, googleai, ollama)
-m, --model STRING               # AI model identifier
-s, --system-prompt STRING       # System instructions
-t, --max-tokens INT             # Maximum response length
-T, --temperature FLOAT          # Creativity level (0.0-1.0)
```

### Advanced Features
```bash
    --prompt-caching             # Enable prompt caching
    --thinking-mode STRING       # Reasoning mode (none, low, medium, high, auto)
    --thinking-budget INT        # Reasoning token budget
    --usage                      # Show usage statistics
    --show-reasoning             # Display AI reasoning process
    --interleaved-thinking       # Advanced reasoning (Claude 4+ only)
```

### Technical Options
```bash
    --completion-timeout DURATION # API timeout (e.g., "2m", "30s")
    --max-retries INT            # Retry attempts
    --retry-delay DURATION       # Initial retry delay
    --disable-retry              # Disable retries
    --config PATH                # Configuration file path
```

## Backend-Specific Configuration

### Anthropic (Claude)

**Recommended for**: Complex reasoning, code analysis, detailed explanations

**Available Models**:
- `claude-opus-4-1-20250805` - Most capable, highest cost
- `claude-sonnet-4-20250514` - Balanced performance and cost (recommended)
- `claude-3-5-sonnet-20241022` - Fast, cost-effective
- `claude-haiku-3-20240307` - Fastest, lowest cost

**Special Features**:
- **Thinking/Reasoning modes**: Available on Claude 4+ models
- **Extended context**: Up to 200K tokens
- **Prompt caching**: Significant cost savings for repeated prompts

**Example Configuration**:
```yaml
backend: "anthropic"
model: "claude-sonnet-4-20250514"
maxTokens: 8192
temperature: 0.05
promptCaching: true
thinkingMode: "medium"
showReasoning: true
```

**Environment Setup**:
```bash
export ANTHROPIC_API_KEY="sk-ant-your-key-here"
```

### OpenAI (GPT)

**Recommended for**: Creative writing, conversational AI, general tasks

**Available Models**:
- `gpt-4` - Most capable GPT-4 model
- `gpt-4-turbo` - Faster GPT-4 variant
- `gpt-3.5-turbo` - Fast and cost-effective
- `o1-preview` - Advanced reasoning model
- `o1-mini` - Reasoning model (smaller)

**Special Features**:
- **Reasoning tokens**: Available on o1+ models
- **Function calling**: Advanced API features
- **Fine-tuning**: Custom model training

**Example Configuration**:
```yaml
backend: "openai"
model: "gpt-4"
maxTokens: 4000
temperature: 0.7
showUsage: true
```

**Environment Setup**:
```bash
export OPENAI_API_KEY="sk-your-openai-key-here"
```

### Google AI (Gemini)

**Recommended for**: Fast responses, cost-effective processing, multimodal tasks

**Available Models**:
- `gemini-2.5-pro` - Latest and most capable
- `gemini-1.5-pro` - Extended context (2M tokens)
- `gemini-pro` - Balanced performance
- `gemini-flash` - Fastest responses

**Special Features**:
- **Extended context**: Up to 2M tokens on some models
- **Thinking budget**: Available on Gemini 2.5+
- **Multimodal**: Built-in image and document processing

**Example Configuration**:
```yaml
backend: "googleai"
model: "gemini-2.5-pro"
maxTokens: 8192
temperature: 0.1
thinkingBudget: 1000
showUsage: true
```

**Environment Setup**:
```bash
export GOOGLE_API_KEY="your-google-ai-key-here"
```

### OpenRouter

**Recommended for**: Access to multiple providers, model comparison, specialized models

**Model Format**: `provider/model` (e.g., `anthropic/claude-sonnet-4`, `openai/gpt-4`)

**Available Providers**:
- `anthropic/` - Claude models
- `openai/` - GPT models
- `google/` - Gemini models
- `meta-llama/` - Llama models
- `mistralai/` - Mistral models
- And many more...

**Example Configuration**:
```yaml
backend: "openrouter"
model: "anthropic/claude-sonnet-4"
maxTokens: 4000
temperature: 0.05
showUsage: true
```

**Environment Setup**:
```bash
export OPENROUTER_API_KEY="sk-or-your-openrouter-key-here"
```

### Ollama (Local Models)

**Recommended for**: Privacy, offline usage, custom models, development

**Available Models** (examples):
- `llama3.2` - Meta's latest Llama
- `mistral` - Mistral models
- `codellama` - Code-specialized Llama
- `deepseek-coder` - Coding specialist
- `qwen2.5` - Alibaba's model

**Requirements**:
- Ollama installed and running locally
- Models downloaded: `ollama pull llama3.2`

**Example Configuration**:
```yaml
backend: "ollama"
model: "llama3.2"
maxTokens: 4000
temperature: 0.3
# No API key required for local models
```

**Setup**:
```bash
# Install Ollama
curl -fsSL https://ollama.ai/install.sh | sh

# Pull a model
ollama pull llama3.2

# Start Ollama service (usually automatic)
ollama serve
```

## Example Configurations

### Task-Specific Configurations

#### Code Review Configuration
```yaml
# code-review-config.yaml
backend: "anthropic"
model: "claude-sonnet-4-20250514"
maxTokens: 8000
temperature: 0.1
systemPrompt: |
  You are a senior software engineer and code reviewer. When reviewing code:
  1. Focus on bugs, security issues, and performance problems
  2. Suggest best practices and improvements
  3. Explain your reasoning clearly
  4. Provide specific, actionable feedback

  Format your review as:
  - Summary: Overall assessment
  - Issues: List of problems found
  - Suggestions: Specific improvements
  - Code Examples: Show better alternatives when relevant
promptCaching: true
showUsage: true
thinkingMode: "medium"
```

#### Research Assistant Configuration
```yaml
# research-config.yaml
backend: "anthropic"
model: "claude-opus-4-1-20250805"        # Most capable for complex analysis
maxTokens: 16000                          # Long responses for detailed analysis
temperature: 0.05                         # Focused and factual
systemPrompt: |
  You are a research assistant with expertise across multiple domains.
  Provide comprehensive, well-sourced analysis with:
  1. Clear methodology explanation
  2. Evidence-based conclusions
  3. Identification of limitations and uncertainties
  4. Suggestions for further investigation
promptCaching: true
showUsage: true
thinkingMode: "high"
showReasoning: true
```

#### Creative Writing Configuration
```yaml
# creative-config.yaml
backend: "openai"
model: "gpt-4"
maxTokens: 8000
temperature: 0.8                          # Higher creativity
systemPrompt: |
  You are a creative writing assistant and storytelling expert.
  Help with:
  - Character development and dialogue
  - Plot structure and pacing
  - Descriptive writing and world-building
  - Style and voice refinement

  Always maintain the writer's voice while offering constructive suggestions.
showUsage: true
```

#### Quick Q&A Configuration
```yaml
# quick-config.yaml
backend: "googleai"
model: "gemini-flash"                     # Fastest responses
maxTokens: 2000                           # Shorter responses
temperature: 0.1                          # Focused answers
systemPrompt: "Provide concise, accurate answers. Be direct and helpful."
promptCaching: false                      # Not needed for quick queries
showUsage: false                          # Minimize output
```

### Environment-Specific Configurations

#### Development Environment
```yaml
# dev-config.yaml
backend: "anthropic"
model: "claude-haiku-3-20240307"          # Fast for development
maxTokens: 4000
temperature: 0.1
debug: true                               # Show debug info
verbose: true                             # Detailed output
showUsage: true                           # Monitor costs
promptCaching: true                       # Save money during development
maxRetries: 1                             # Fail fast during development
retryDelay: "500ms"
```

#### Production Environment
```yaml
# prod-config.yaml
backend: "anthropic"
model: "claude-sonnet-4-20250514"
maxTokens: 8000
temperature: 0.05
debug: false                              # Clean output
verbose: false
showUsage: false                          # Minimal output
promptCaching: true                       # Cost optimization
maxRetries: 5                             # Robust error handling
retryDelay: "2s"                          # Conservative retry timing
completionTimeout: "5m"                   # Longer timeout for reliability
```

#### Cost-Optimized Configuration
```yaml
# cost-optimized-config.yaml
backend: "googleai"                       # Generally lower cost
model: "gemini-flash"                     # Fastest, cheapest
maxTokens: 2000                           # Limit response length
temperature: 0.05                         # Focused responses
promptCaching: true                       # Maximum cost savings
showUsage: true                           # Monitor spending
```

## Advanced Configuration

### Logit Bias (Token Probability Adjustment)

Fine-tune the AI's token selection probability:

```yaml
# Encourage certain tokens
logitBias:
  "Python": 0.5                          # Strongly favor Python mentions
  "JavaScript": -0.5                     # Discourage JavaScript mentions
  "secure": 0.2                          # Slightly favor security-related terms
  "deprecated": -0.3                     # Discourage deprecated mentions
  "yes": 0.1                             # Slightly favor positive responses
  "no": -0.1                             # Slightly discourage negative responses
```

**Note**: Logit bias is an advanced feature. Values typically range from -1.0 to 1.0.

### Custom System Prompts

Create sophisticated system prompts for specialized tasks:

```yaml
systemPrompt: |
  You are a specialized AI assistant for DevOps and Site Reliability Engineering.

  EXPERTISE AREAS:
  - Infrastructure as Code (Terraform, CloudFormation)
  - Container orchestration (Kubernetes, Docker)
  - CI/CD pipelines (GitHub Actions, Jenkins)
  - Monitoring and observability (Prometheus, Grafana)
  - Cloud platforms (AWS, GCP, Azure)

  RESPONSE GUIDELINES:
  1. Always consider security implications
  2. Prefer cloud-native and scalable solutions
  3. Include relevant best practices
  4. Provide working code examples when possible
  5. Explain trade-offs and alternatives

  RESPONSE FORMAT:
  - Start with a brief summary
  - Provide step-by-step instructions
  - Include code snippets with explanations
  - End with testing/validation steps

  Current context: Production environment, high availability requirements
```

### Multi-Environment Configuration Management

Use different configs for different environments:

```bash
# Directory structure
~/.cgpt/
├── config.yaml              # Default configuration
├── dev-config.yaml          # Development settings
├── prod-config.yaml         # Production settings
├── creative-config.yaml     # Creative tasks
└── research-config.yaml     # Research tasks

# Usage
cgpt --config ~/.cgpt/dev-config.yaml -i "Debug this code"
cgpt --config ~/.cgpt/creative-config.yaml -c  # Creative session
```

### Configuration Inheritance

You can create base configurations and extend them:

```yaml
# base-config.yaml
backend: "anthropic"
maxTokens: 8000
temperature: 0.05
promptCaching: true
showUsage: true
maxRetries: 3
retryDelay: "1s"
```

```yaml
# dev-config.yaml (extends base)
<<: *base-config               # YAML merge key (if supported)
model: "claude-haiku-3-20240307"
debug: true
verbose: true
```

## Configuration Troubleshooting

### Common Configuration Issues

#### 1. Configuration File Not Found
```bash
# Check configuration search paths
cgpt --verbose -i "test" 2>&1 | grep -i config

# Verify file exists and is readable
ls -la ~/.cgpt/config.yaml
cat ~/.cgpt/config.yaml | head -5
```

#### 2. Invalid YAML Syntax
```bash
# Validate YAML syntax
python -c "import yaml; yaml.safe_load(open('config.yaml'))"

# Or use yq if available
yq eval . config.yaml
```

#### 3. Environment Variable Conflicts
```bash
# Check all CGPT-related environment variables
env | grep -i cgpt

# Test configuration loading
cgpt --verbose --debug -i "test config" 2>&1 | head -20
```

#### 4. API Key Issues
```bash
# Verify API keys are set
echo "Anthropic: ${ANTHROPIC_API_KEY:0:10}..."
echo "OpenAI: ${OPENAI_API_KEY:0:10}..."
echo "Google: ${GOOGLE_API_KEY:0:10}..."

# Test API connectivity
cgpt --debug -i "test" -m claude-haiku-3-20240307
```

#### 5. Model/Backend Mismatches
```bash
# Check available models for backend
cgpt --verbose -b anthropic -m gpt-4 -i "test" 2>&1 | grep -i model
# Should show warning about model/backend mismatch
```

### Debug Configuration Loading

Enable verbose mode to see how cgpt loads configuration:

```bash
cgpt --verbose --debug -i "configuration test" 2>&1 | grep -E "(config|backend|model|key)"
```

This will show:
- Configuration file paths searched
- Environment variables found
- Final configuration values used
- API key validation (without exposing keys)

### Validation Commands

```bash
# Test basic setup
cgpt --version
cgpt --help | head -10

# Test configuration loading
cgpt --verbose -i "test" --dry-run 2>&1 | head -20

# Test specific backend
cgpt --verbose -b anthropic -m claude-haiku-3-20240307 -i "test"

# Test model auto-detection
cgpt --verbose -m gpt-4 -i "test" 2>&1 | grep -i backend
```

### Configuration Best Practices

1. **Start Simple**: Begin with basic configuration and add complexity gradually
2. **Use Environment Variables**: For API keys and environment-specific settings
3. **Version Control**: Keep configuration files in version control (excluding API keys)
4. **Document Changes**: Comment your configuration files
5. **Test Configurations**: Verify settings work before deploying
6. **Monitor Usage**: Enable `showUsage` to track costs and performance
7. **Security**: Never commit API keys; use environment variables or secure vaults

This configuration reference should help you set up cgpt for your specific needs. For additional help, use `cgpt --help` or check the [GitHub repository](https://github.com/tmc/cgpt) for the latest documentation.