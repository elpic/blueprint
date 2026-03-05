# Schedule Rules

Install a crontab entry that runs `blueprint apply <source> --skip-decrypt` on a recurring schedule:

```
schedule <preset> source: <path|dir|repo> [id: <rule-id>] [after: <dependency>] on: [platform1, platform2, ...]
schedule cron: "<expression>" source: <path|dir|repo> [id: <rule-id>] [after: <dependency>] on: [platform1, platform2, ...]
```

**What is this used for?**
Automate blueprint execution so your machine stays up to date without manual intervention. After the initial `blueprint apply`, the schedule rule installs a crontab entry that re-applies the blueprint on every run.

**Presets:**
| Preset   | Cron expression |
|----------|-----------------|
| `daily`  | `@daily`        |
| `weekly` | `@weekly`       |
| `hourly` | `@hourly`       |

**Options:**
- `source: <value>` - File path, directory, or repo path passed directly to `blueprint apply` (required)
- `cron: "<expression>"` - Raw cron expression, e.g. `"0 9 * * *"` (alternative to preset)
- `id: <rule-id>` - Give this rule a unique identifier (optional, auto-generated as `schedule-<preset>` or `schedule-custom`)
- `after: <dependency>` - Execute after another rule (optional)
- `on: [platforms]` - Target specific platforms (optional)

**How it works:**
1. Verifies the current user has passwordless sudo (`sudo -n true`) — hard fails if not
2. Reads the current crontab (`crontab -l`)
3. Skips if the exact crontab line already exists (idempotent)
4. Appends the new line and reinstalls via `crontab -`
5. Auto-removes the crontab line when the rule is removed from the blueprint

**Examples:**

```blueprint
# Run blueprint daily (prerequisite: sudoers rule must be set up first)
sudoers on: [mac, linux]
schedule daily source: setup.bp on: [mac, linux]

# Weekly sync of a dotfiles repo
schedule weekly source: ~/dotfiles on: [mac]

# Custom cron — every day at 9am
schedule cron: "0 9 * * *" source: ~/dotfiles/setup.bp id: morning-sync on: [mac]

# With explicit ID and dependency
sudoers id: passwordless-sudo on: [mac, linux]
schedule daily source: /path/to/setup.bp id: daily-apply after: passwordless-sudo on: [mac, linux]
```

**Crontab line installed:**
```
@daily /path/to/blueprint apply setup.bp --skip-decrypt >> ~/.blueprint/schedule.log 2>&1
```

**Notes:**
- Output (stdout + stderr) is appended to `~/.blueprint/schedule.log` — tail it with `tail -f ~/.blueprint/schedule.log`
- `--skip-decrypt` is always appended so cron jobs never block on password prompts
- The absolute path to the `blueprint` binary is used (via `os.Executable()`) to avoid `PATH` issues in cron's minimal environment
- A `sudoers` rule must be applied before scheduling, otherwise the pre-flight check fails with a clear error message
- Removing the schedule rule from the blueprint and re-running `apply` removes the crontab entry automatically
