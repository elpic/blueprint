# Known Hosts Rules

Manage SSH known_hosts file entries for host verification:

```
known_hosts <host> [key-type: <type>] [id: <rule-id>] [after: <dependency>] on: [platform1, platform2, ...]
```

**What is this used for?**
Automatically add SSH hosts to your `~/.ssh/known_hosts` file to prevent "Host key verification failed" prompts during SSH connections.

**Options:**
- `key-type: <type>` - SSH key type to scan for (ed25519, ecdsa, rsa). If not specified, auto-detects in order: ed25519 → ecdsa → rsa (optional)
- `id: <rule-id>` - Give this rule a unique identifier (optional)
- `after: <dependency>` - Execute after another rule (optional)
- `on: [platforms]` - Target specific platforms (optional)

**How it works:**
1. Creates `~/.ssh` directory with permissions 0700 (if not exists)
2. Creates `~/.ssh/known_hosts` file with permissions 0600 (if not exists)
3. Uses `ssh-keyscan` to retrieve host public key
4. Adds host entry to known_hosts file to prevent verification prompts
5. Automatically tries multiple key types with fallback if one fails

**Examples:**

```blueprint
# Simple known_hosts entry (auto-detects key type)
known_hosts github.com on: [mac, linux]

# Explicit key type
known_hosts github.com key-type: ed25519 on: [mac, linux]

# With ID for dependencies
known_hosts gitlab.com key-type: rsa id: gitlab-host on: [mac]

# Multiple hosts with different key types
known_hosts github.com key-type: ed25519 id: github on: [mac, linux]
known_hosts gitlab.com key-type: rsa id: gitlab on: [mac, linux]
known_hosts bitbucket.org key-type: ecdsa id: bitbucket on: [mac, linux]

# With dependencies
known_hosts github.com id: github on: [mac]
clone https://github.com/user/private-repo.git to: ~/projects/repo after: github on: [mac]
```
