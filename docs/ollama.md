# Ollama Rules

Pull and manage local LLM models via [Ollama](https://ollama.com):

```
ollama <model ...> [id: <rule-id>] [after: <dependency>] on: [platform1, platform2, ...]
```

**Model Syntax:**
- `model` - Pull a model by name (e.g., `llama3`, `codellama`, `mistral`)
- Multiple models can be specified in a single rule

**Options:**
- `id: <rule-id>` - Give this rule a unique identifier (optional)
- `after: <dependency>` - Execute after another rule (optional)
- `on: [platforms]` - Target specific platforms (optional, defaults to all)

**Behavior:**
- Installs Ollama itself via `curl -fsSL https://ollama.com/install.sh | sh` if not already present
- Runs `ollama pull <model>` for each model listed in the rule
- Auto-uninstalls models (`ollama rm`) if removed from the blueprint
- Does not require sudo
- Concurrent Ollama installation attempts are serialized with a mutex to avoid conflicts

**Examples:**
```
# Pull a single model
ollama llama3 on: [mac, linux]

# Pull multiple models in one rule
ollama llama3 codellama mistral on: [mac, linux]

# With a rule ID and dependency
ollama llama3 id: llm-setup after: ollama-install on: [linux]
```

**Auto-generated IDs:**

When no `id:` is specified, the handler generates one automatically:
- `ollama-<first-model>` when at least one model is listed (e.g., `ollama-llama3`)
- `ollama` when no models are listed

**Automatic cleanup:**
```
# Before
ollama llama3 codellama mistral on: [mac]

# After (codellama and mistral removed)
ollama llama3 on: [mac]

# blueprint apply setup.bp -> codellama and mistral are auto-uninstalled via `ollama rm`
```

**Status tracking:**

Blueprint records each pulled model in `~/.blueprint/status.json` with the model name, install timestamp, source blueprint, and OS. View installed models with:

```bash
blueprint status
```
