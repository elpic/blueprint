# Sudoers Rules

Grant the current user (or a specified user) passwordless sudo by writing a drop-in file to `/etc/sudoers.d/`:

```
sudoers [user: <username>] [id: <rule-id>] [after: <dependency>] on: [platform1, platform2, ...]
```

**What is this used for?**
Configure passwordless sudo so that subsequent blueprint runs (and cron-scheduled runs) can execute privileged commands without prompting for a password. This is typically the first rule in a blueprint that uses the `schedule` action.

**Options:**
- `user: <username>` - User to grant passwordless sudo (optional, defaults to the current `$USER`)
- `id: <rule-id>` - Give this rule a unique identifier (optional, defaults to `"sudoers"`)
- `after: <dependency>` - Execute after another rule (optional)
- `on: [platforms]` - Target specific platforms (optional)

**How it works:**
1. Resolves the target user (`user:` value or `$USER`)
2. Writes `<user> ALL=(ALL) NOPASSWD: ALL` to a temp file
3. Validates it with `visudo -c -f`
4. Installs it to `/etc/sudoers.d/<user>` with `0440` permissions using `sudo install`
5. Auto-removes the drop-in file when the rule is removed from the blueprint

**Examples:**

```blueprint
# Grant passwordless sudo to the current user
sudoers on: [mac, linux]

# Grant passwordless sudo to a specific user
sudoers user: deploy id: sudoers-deploy on: [linux]

# With dependency
install sudo id: sudo-pkg on: [linux]
sudoers after: sudo-pkg on: [linux]
```

**Security notes:**
- Writes `NOPASSWD: ALL` — only use on personal/trusted machines
- File is validated by `visudo` before installation; invalid entries are rejected
- Requires sudo to install (you will be prompted once)
- When removed from blueprint, the drop-in file is deleted automatically
