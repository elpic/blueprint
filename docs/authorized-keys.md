# Authorized Keys Rules

Add SSH public keys to `~/.ssh/authorized_keys` for passwordless SSH access:

```
authorized_keys file: <path> [id: <rule-id>] [after: <dependency>] [on: [platform1, platform2, ...]]
authorized_keys encrypted: <encrypted-file> [password-id: <id>] [id: <rule-id>] [after: <dependency>] [on: [platform1, platform2, ...]]
```

**What is this used for?**
Provision SSH public keys into `~/.ssh/authorized_keys` so that remote machines accept SSH connections from your keys. Supports plain `.pub` files and encrypted files (via the existing `decrypt` pipeline).

**Options:**
- `file: <path>` — Path to a plain text file containing one or more public key lines (e.g. `~/.ssh/id_ed25519.pub`). Required unless `encrypted:` is used.
- `encrypted: <path>` — Path to an AES-256-GCM encrypted file containing public key line(s). Required unless `file:` is used.
- `password-id: <id>` — Password ID to use when decrypting an `encrypted:` file. Defaults to `"default"` (optional).
- `id: <rule-id>` — Unique identifier for this rule (optional).
- `after: <dependency>` — Execute after another rule (optional).
- `on: [platforms]` — Target specific platforms (optional).

**How it works:**
1. Creates `~/.ssh/` with permissions `0700` (if not exists)
2. Creates `~/.ssh/authorized_keys` with permissions `0600` (if not exists)
3. Reads the public key(s) from the source file (plain or decrypted)
4. Appends only key lines not already present — idempotent on repeated runs
5. `IsInstalled()` verifies both status and physical file presence

**Examples:**

```blueprint
# Add a local public key
authorized_keys file: ~/.ssh/id_ed25519.pub on: [mac, linux]

# Add a key from an encrypted secrets file
authorized_keys encrypted: secrets/authorized_key.enc on: [linux]

# Encrypted with a specific password-id
authorized_keys encrypted: secrets/deploy_key.enc password-id: ssh-keys on: [linux]

# With ID for dependencies
authorized_keys file: ~/.ssh/id_ed25519.pub id: add-pubkey on: [mac, linux]
known_hosts github.com after: add-pubkey on: [mac, linux]
```
