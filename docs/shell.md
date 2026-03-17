# Shell Handler

The shell handler allows you to declaratively set your default login shell using the `shell: <shell-name>` syntax.

## Syntax

```
shell <shell-name> [id:<id>] [on:<os-list>] [after:<dependency-list>]
```

### Parameters

- `<shell-name>`: The name or path of the shell to set as default
- `id:<id>`: Optional unique identifier for dependency resolution
- `on:<os-list>`: Optional OS filter (e.g., `on:mac,linux`)
- `after:<dependency-list>`: Optional dependencies that must be installed first

## Examples

### Basic Shell Change

```bash
# Set zsh as the default shell
shell zsh

# Set fish as the default shell
shell fish

# Set bash as the default shell
shell bash
```

### Using Absolute Paths

```bash
# Use absolute path for custom shell locations
shell /usr/local/bin/fish
shell /opt/homebrew/bin/zsh
```

### With OS Filters

```bash
# Only apply on macOS
shell zsh on:mac

# Only apply on Linux systems
shell bash on:linux

# Apply on both macOS and Linux
shell zsh on:mac,linux
```

### With Dependencies

```bash
# Install fish first, then set it as shell
install fish
shell fish after:fish

# With explicit IDs
install fish id:my-fish
shell fish id:set-fish after:my-fish
```

### Complex Example

```bash
# Install modern shells on different platforms
install fish on:mac,linux
homebrew fish on:mac
install zsh on:linux

# Set fish as default shell after installation
shell fish after:fish on:mac,linux
```

## How It Works

1. **Shell Resolution**: The handler first resolves the shell name to its full path by checking common locations:
   - `/bin/<shell>`
   - `/usr/bin/<shell>`
   - `/usr/local/bin/<shell>`
   - `/opt/homebrew/bin/<shell>` (Homebrew on Apple Silicon)
   - `/opt/local/bin/<shell>` (MacPorts)
   - Falls back to `which <shell>`

2. **Validation**: Ensures the shell:
   - Exists and is a file (not a directory)
   - Is executable
   - Is listed in `/etc/shells` (when available)

3. **Idempotency**: Checks if the shell is already set before making changes

4. **Shell Change**: Uses `chsh -s <shell-path>` to change the default shell

## Platform Support

### macOS
- Uses `dscl` to read current shell information
- Supports shells installed via Homebrew or MacPorts
- Works with system shells and custom installations

### Linux
- Uses `getent passwd` or `/etc/passwd` to read shell information
- Validates against `/etc/shells` when available
- Supports distribution package managers

## Common Shells

### Zsh
```bash
shell zsh
```
Usually located at `/bin/zsh` or `/usr/bin/zsh`

### Fish
```bash
# Install fish first
install fish        # Linux
homebrew fish       # macOS

# Then set as default
shell fish after:fish
```

### Bash
```bash
shell bash
```
Default on most systems, usually at `/bin/bash`

### Dash
```bash
shell dash
```
Lightweight POSIX-compliant shell

## Error Handling

The shell handler provides detailed error messages for common issues:

### Shell Not Found
```
Error: shell 'myshell' not found in common locations
```
**Solution**: Install the shell first or use the absolute path

### Shell Not Executable
```
Error: shell is not executable: /path/to/shell
```
**Solution**: Check file permissions

### Shell Not in /etc/shells
```
Error: shell '/usr/local/bin/fish' is not listed in /etc/shells
```
**Solution**: Add the shell to `/etc/shells` or install it properly

### Permission Denied
```
Error: failed to change shell: permission denied
```
**Solution**: Ensure you have permission to change your shell

## Security Considerations

- The shell handler only changes the shell for the current user
- Validates shells against `/etc/shells` when available
- Does not require sudo for normal operation
- Cannot automatically revert shell changes (for safety)

## Status and Tracking

The shell handler tracks:
- Which shell was set
- For which user
- When the change was made
- From which blueprint file

Use `blueprint ps` to see current shell status.

## Limitations

1. **No Automatic Uninstall**: Shell changes cannot be automatically reverted for safety reasons
2. **User-Specific**: Only changes the shell for the current user
3. **Session Restart Required**: New shell takes effect on next login/session

## Integration with Other Handlers

### Install Shell First
```bash
# Install a modern shell, then set it as default
install fish
shell fish after:fish
```

### Platform-Specific Shells
```bash
# Different shells for different platforms
homebrew fish on:mac
install fish on:linux

shell fish after:fish
```

### Development Environment
```bash
# Complete development setup
install git vim curl
homebrew fish on:mac
install fish on:linux

# Configure shell after tools are available
shell fish after:fish,git

# Clone dotfiles that might configure the shell
clone https://github.com/user/dotfiles.git to:~/dotfiles after:fish
```

## Troubleshooting

### Shell Change Not Taking Effect
- Log out and log back in
- Open a new terminal window
- Check if the change was applied: `echo $SHELL`

### Shell Not Available
- Verify installation: `which fish`
- Check `/etc/shells`: `cat /etc/shells`
- Install using package manager first

### Permission Issues
- Ensure `/etc/shells` contains your shell
- Contact system administrator if needed
- Use `chsh -l` to list available shells