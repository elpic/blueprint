# Decrypt Rules

Decrypt encrypted files to specified locations with optional password protection:

```
decrypt <encrypted-file> to: <destination> [group: <group>] [password-id: <id>] [id: <rule-id>] [after: <dependency>] on: [platform1, platform2, ...]
```

**What is this used for?**
Safely store sensitive files (SSH keys, certificates, config files) in encrypted form in your repository, then decrypt them during blueprint execution.

**Options:**
- `to: <destination>` - Where to decrypt the file (supports `~/` for home directory)
- `group: <group>` - Group name for grouping related decrypt rules (optional)
- `password-id: <id>` - Unique identifier for password grouping (optional, defaults to "default")
- `id: <rule-id>` - Give this rule a unique identifier (optional)
- `after: <dependency>` - Execute after another rule (optional)
- `on: [platforms]` - Target specific platforms (optional)

**How it works:**
1. Blueprint prompts for all unique `password-id` values at the start of execution
2. Encrypted files are decrypted using the provided password
3. Decrypted files are written with restricted permissions (0600)
4. Multiple decrypt rules can share the same `password-id` (prompted only once)
5. Works with both local files and git repository blueprints

**Examples:**

```blueprint
# Simple decrypt
decrypt id_rsa.enc to: ~/.ssh/id_rsa password-id: main on: [mac, linux]

# Multiple decrypts with same password
decrypt id_rsa.enc to: ~/.ssh/id_rsa password-id: main on: [mac, linux]
decrypt config.enc to: ~/.config/app.conf password-id: main on: [mac, linux]

# With group for organization
decrypt ssh-key.enc to: ~/.ssh/id_rsa group: security password-id: main on: [mac]
decrypt ssl-cert.enc to: ~/.certs/cert.pem group: security password-id: main on: [mac]

# With dependencies
asdf nodejs@18.19.0 id: setup-node on: [mac]
decrypt npm-token.enc to: ~/.npmrc password-id: npm-creds after: setup-node on: [mac]
```

**Creating encrypted files:**

Use the `encrypt` command to create encrypted files for your blueprint:

```bash
# Encrypt a file with default password-id
./blueprint encrypt ~/.ssh/id_rsa

# Encrypt with specific password-id
./blueprint encrypt ~/.ssh/id_rsa --password-id main

# Creates: ~/.ssh/id_rsa.enc
```

This creates `~/.ssh/id_rsa.enc` which can be added to version control safely.

**Security notes:**
- Encrypted files use AES-256-GCM encryption
- Each encryption uses a random nonce
- Passwords are derived using SHA-256
- Decrypted files are written with 0600 permissions (user-only)
- Passwords are cached during execution but never saved
- Works with repository-based blueprints (clones to temp directory)
