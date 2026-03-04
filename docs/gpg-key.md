# GPG Key Rules

Add GPG keys and configure Debian repositories with signature verification:

```
gpg-key <url> keyring: <name> deb-url: <url> [id: <rule-id>] [after: <dependency>] on: [platform1, platform2, ...]
```

**What is this used for?**
Add GPG keys for Debian package repositories to enable secure package verification. This is commonly used when adding third-party package repositories.

**Options:**
- `keyring: <name>` - Name for the keyring file (stored as `/usr/share/keyrings/<name>.gpg`)
- `deb-url: <url>` - Debian repository URL for the sources.list entry
- `id: <rule-id>` - Give this rule a unique identifier (optional)
- `after: <dependency>` - Execute after another rule (optional)
- `on: [platforms]` - Target specific platforms (Linux only) (optional)

**How it works:**
1. Downloads GPG key from URL using `curl`
2. Converts key from ASCII-armored format to binary using `gpg --dearmor`
3. Stores key in `/usr/share/keyrings/<name>.gpg`
4. Adds repository source to `/etc/apt/sources.list.d/<name>.list` with signature verification
5. Sets keyring file permissions to 0644
6. Runs `sudo apt update` to refresh package cache
7. Auto-uninstalls when rule is removed from blueprint

**Examples:**

```blueprint
# Simple GPG key and repository
gpg-key https://apt.fury.io/wez/gpg.key keyring: wezterm-fury deb-url: https://apt.fury.io/wez/ on: [linux]

# With ID for dependencies
gpg-key https://example.com/repo.key keyring: example-repo deb-url: https://example.com/apt id: example-setup on: [linux]

# Multiple repositories
gpg-key https://apt.fury.io/wez/gpg.key keyring: wezterm-fury deb-url: https://apt.fury.io/wez/ on: [linux]
gpg-key https://keyserver.ubuntu.com/export/key.asc keyring: ubuntu-ppa deb-url: https://ppa.launchpad.net/example/ppa/ubuntu on: [linux]

# With dependencies
install curl id: curl-setup on: [linux]
gpg-key https://apt.fury.io/wez/gpg.key keyring: wezterm-fury deb-url: https://apt.fury.io/wez/ after: curl-setup on: [linux]
install wezterm after: wezterm-fury on: [linux]
```

**Command Executed:**
The following commands are executed for a gpg-key rule:
```bash
curl -fsSL <url> | sudo gpg --yes --dearmor -o /usr/share/keyrings/<name>.gpg
echo 'deb [signed-by=/usr/share/keyrings/<name>.gpg] <deb-url> * *' | sudo tee /etc/apt/sources.list.d/<name>.list
sudo chmod 644 /usr/share/keyrings/<name>.gpg
sudo apt update
```

**Security notes:**
- Keys are verified using GPG's standard verification
- Repository entries use signed-by flag for secure verification
- Keyring files are world-readable but only root can modify
- When removed from blueprint, both the key and repository source are deleted
- Works only on Linux systems with apt package manager
