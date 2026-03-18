# Troubleshooting: Oh-My-Zsh and Antigen Setup Issues

## Problem Description
User is experiencing errors where zsh configuration is trying to load antigen components that don't exist:

```
/home/elpic/.zshrc:source:23: no such file or directory: /home/elpic/.oh-my-zsh/antigen.zsh
/home/elpic/.zshrc:27: command not found: antigen
```

## Root Cause Analysis

### 1. Check if Blueprint File Exists
```bash
# Look for the user's blueprint file
ls -la /home/elpic/setup
ls -la /home/elpic/*.bp
ls -la /home/elpic/*/setup*
```

### 2. Check Current .zshrc Configuration
```bash
# Examine what's currently in .zshrc
cat /home/elpic/.zshrc | head -30
```

### 3. Check Blueprint Execution History
```bash
# Check if blueprint was ever executed
cat /home/elpic/.blueprint/history.json
```

### 4. Check Current Installation Status
```bash
# Check what's currently installed
ls -la /home/elpic/.oh-my-zsh/
ls -la /home/elpic/.oh-my-zsh/antigen.zsh
```

## Solution 1: Create Proper Blueprint File

Create `/home/elpic/setup.bp` with the following content:

```blueprint
# Oh-My-Zsh and Antigen Setup
# Fixes the "antigen.zsh not found" error

# 1. Install zsh shell (if not already installed)
install zsh on: [linux]

# 2. Clone oh-my-zsh to the standard location
clone https://github.com/ohmyzsh/ohmyzsh.git to: ~/.oh-my-zsh id: ohmyzsh on: [linux]

# 3. Download antigen.zsh to the oh-my-zsh directory  
download https://git.io/antigen to: ~/.oh-my-zsh/antigen.zsh id: antigen after: ohmyzsh on: [linux]

# 4. Create a proper .zshrc configuration
run 'cat > ~/.zshrc << EOF
# Oh My Zsh Configuration
export ZSH="\\$HOME/.oh-my-zsh"
ZSH_THEME="robbyrussell"

# Load Antigen
source \\$ZSH/antigen.zsh

# Load oh-my-zsh library
antigen use oh-my-zsh

# Load plugins
antigen bundle git
antigen bundle command-not-found
antigen bundle zsh-users/zsh-syntax-highlighting

# Apply the configuration
antigen apply
EOF' id: setup-zshrc after: antigen on: [linux]
```

## Solution 2: Execute Blueprint

```bash
# Preview what will be executed (dry run)
./blueprint plan /home/elpic/setup.bp

# Execute the blueprint
./blueprint apply /home/elpic/setup.bp
```

## Solution 3: Manual Fix (Quick Workaround)

If blueprint isn't available, manually fix the issue:

```bash
# 1. Install oh-my-zsh
sh -c "$(curl -fsSL https://raw.githubusercontent.com/ohmyzsh/ohmyzsh/master/tools/install.sh)"

# 2. Download antigen
curl -L git.io/antigen > ~/.oh-my-zsh/antigen.zsh

# 3. Fix .zshrc to use correct antigen path
sed -i 's|source.*antigen.zsh|source $HOME/.oh-my-zsh/antigen.zsh|g' ~/.zshrc
```

## Solution 4: Alternative Antigen Setup

Instead of placing antigen.zsh in oh-my-zsh directory, use a dedicated antigen directory:

```blueprint
# Alternative setup with dedicated antigen directory
install zsh on: [linux]
clone https://github.com/ohmyzsh/ohmyzsh.git to: ~/.oh-my-zsh id: ohmyzsh on: [linux]
clone https://github.com/zsh-users/antigen.git to: ~/.antigen id: antigen after: ohmyzsh on: [linux]
```

Then update `.zshrc` to source from the correct location:
```bash
source ~/.antigen/antigen.zsh
```

## Prevention: Best Practices

### 1. Use Blueprint Dependencies
Always specify dependencies to ensure proper installation order:

```blueprint
# Good: Specify dependencies
clone https://github.com/ohmyzsh/ohmyzsh.git to: ~/.oh-my-zsh id: ohmyzsh on: [linux]
download https://git.io/antigen to: ~/.oh-my-zsh/antigen.zsh after: ohmyzsh on: [linux]

# Bad: No dependencies, might fail
clone https://github.com/ohmyzsh/ohmyzsh.git to: ~/.oh-my-zsh on: [linux]
download https://git.io/antigen to: ~/.oh-my-zsh/antigen.zsh on: [linux]
```

### 2. Test Blueprint Before Applying
Always run plan mode first:

```bash
# Test first
./blueprint plan setup.bp

# Then apply
./blueprint apply setup.bp
```

### 3. Use Idempotent Configurations
Blueprint rules should be safe to run multiple times:

```blueprint
# This is idempotent - safe to run multiple times
clone https://github.com/ohmyzsh/ohmyzsh.git to: ~/.oh-my-zsh on: [linux]
```

## Debugging Commands

### Check Blueprint Status
```bash
./blueprint status
```

### Check Blueprint History
```bash
cat ~/.blueprint/history.json | jq '.'
```

### Verify Installation
```bash
# Check if oh-my-zsh exists
ls -la ~/.oh-my-zsh/

# Check if antigen exists
ls -la ~/.oh-my-zsh/antigen.zsh

# Test zsh configuration
zsh -c "source ~/.zshrc"
```

## Common Issues and Fixes

### Issue: Permission Denied
```bash
# Fix: Ensure proper permissions
chmod +x ~/.oh-my-zsh/antigen.zsh
```

### Issue: Network Timeouts
```bash
# Fix: Retry download with timeout
download https://git.io/antigen to: ~/.oh-my-zsh/antigen.zsh timeout: 60s on: [linux]
```

### Issue: Circular Dependencies
```bash
# Fix: Review dependency chain
./blueprint plan setup.bp
# Look for dependency cycles in output
```

### Issue: Wrong File Paths
```bash
# Fix: Use absolute paths with ~ expansion
# Good
download https://git.io/antigen to: ~/.oh-my-zsh/antigen.zsh on: [linux]

# Bad
download https://git.io/antigen to: /home/elpic/.oh-my-zsh/antigen.zsh on: [linux]
```

## Testing Your Fix

1. **Backup current configuration:**
   ```bash
   cp ~/.zshrc ~/.zshrc.backup
   ```

2. **Apply the blueprint:**
   ```bash
   ./blueprint apply setup.bp
   ```

3. **Test new shell:**
   ```bash
   zsh
   # Should load without errors
   ```

4. **Verify antigen commands work:**
   ```bash
   antigen list
   antigen help
   ```

## Further Resources

- [Blueprint Documentation](https://github.com/elpic/blueprint)
- [Oh My Zsh Documentation](https://github.com/ohmyzsh/ohmyzsh)
- [Antigen Documentation](https://github.com/zsh-users/antigen)