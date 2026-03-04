# Mkdir Rules

Create directories with optional permission settings:

```
mkdir <path> [permissions: <octal>] [id: <rule-id>] [after: <dependency>] on: [platform1, platform2, ...]
```

**What is this used for?**
Create directory structures that your applications need. Useful for setting up project directories, cache directories, data directories, and other folder hierarchies with specific permission requirements.

**Options:**
- `permissions: <octal>` - Set directory permissions in octal (0-777). Examples: 700 (rwx------), 755 (rwxr-xr-x), 750 (rwxr-x---). If not specified, uses system default umask (optional)
- `id: <rule-id>` - Give this rule a unique identifier (optional)
- `after: <dependency>` - Execute after another rule (optional)
- `on: [platforms]` - Target specific platforms (optional)

**How it works:**
1. Creates parent directories automatically using `mkdir -p` (like Unix mkdir with -p flag)
2. Applies permissions with `chmod` if specified
3. Idempotent - safe to run multiple times, won't fail if directory exists
4. Supports `~` for home directory expansion

**Common Permission Values:**
- `700` - Owner only (drwx------)
- `755` - Owner full, others read+execute (drwxr-xr-x)
- `750` - Owner full, group read+execute (drwxr-x---)
- `777` - Everyone full access (drwxrwxrwx)

**Examples:**

```blueprint
# Simple directory creation
mkdir ~/.config on: [mac, linux]

# With specific permissions
mkdir ~/.config permissions: 700 on: [mac, linux]

# Nested directories (parent directories created automatically)
mkdir ~/.config/myapp/data permissions: 750 on: [mac, linux]

# With ID for dependencies
mkdir ~/projects id: create-projects on: [mac, linux]

# Multiple directories with dependencies
mkdir ~/projects id: create-projects on: [mac, linux]
mkdir ~/projects/myapp after: create-projects on: [mac, linux]
mkdir ~/projects/myapp/data permissions: 700 after: create-projects on: [mac, linux]

# Project structure setup
mkdir ~/workspace/projects permissions: 755 id: setup-workspace on: [mac, linux]
mkdir ~/workspace/projects/active permissions: 750 after: setup-workspace on: [mac, linux]
mkdir ~/workspace/archive permissions: 700 after: setup-workspace on: [mac, linux]
```

**Security Note:**
When creating directories that will hold sensitive data, use restricted permissions:
- `700` for private directories (owner only)
- `750` for team access (owner read/write/execute, group read/execute, others nothing)
- Avoid `777` unless absolutely necessary
