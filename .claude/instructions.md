# Claude Code Instructions for Blueprint

## Git Commit Guidelines

### Commit Message Format
- Keep commit messages clean and professional
- **DO NOT** include "🤖 Generated with [Claude Code]..." footer
- **DO NOT** include "Co-Authored-By: Claude <noreply@anthropic.com>" line
- Focus on describing the actual work done

### Example Format
```
Update GitHub Actions integration workflow for trunk-based development

## Changes

### Branch Strategy
- Update to trunk-based development: only main and feat/** branches
```

Not:
```
Update GitHub Actions integration workflow for trunk-based development

[detailed content]

🤖 Generated with [Claude Code](https://claude.com/claude-code)

Co-Authored-By: Claude <noreply@anthropic.com>
```

## Development Practices

### Trunk-Based Development
- Primary branches: `main` and `feat/**`
- No `master` or `develop` branches
- Pull requests target `main` branch

### Test Coverage
- Maintain and improve test coverage
- Coverage comparison reports for PRs (increase/decrease tracking)
- All tests must pass before merging

### Code Quality
- Run linter and security checks via GitHub Actions
- Fix any code quality issues before committing
- Keep code clean and maintainable

## Testing
- Run `go test ./...` before committing
- Verify all handler tests pass
- Check integration tests for complex features
