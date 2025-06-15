# Migration from Dependabot to Renovate

This document describes the migration from GitHub Dependabot to Renovate for dependency management in the Inventario project.

## Motivation

The main reasons for migrating from Dependabot to Renovate are:

1. **Volta Support**: Renovate can update Volta Node.js version configurations, which Dependabot cannot handle
2. **Better Grouping**: More flexible grouping options for related dependencies
3. **Enhanced Configuration**: More granular control over update schedules and strategies
4. **Advanced Features**: Better handling of lock files, semantic commits, and post-update operations

## Configuration Comparison

### Dependabot (Previous)

The previous `.github/dependabot.yml` configuration handled:
- Go modules in `/go` directory with weekly updates
- npm packages in `/frontend` directory with weekly updates  
- npm packages in `/e2e` directory with weekly updates
- GitHub Actions with weekly updates
- Custom grouping for cloud SDKs, Golang core packages, and OpenAPI packages

### Renovate (New)

The new `renovate.json` configuration provides:
- All the same functionality as Dependabot
- **Enhanced Volta support**: Automatically detects and updates Volta Node.js/npm versions in package.json
- **Better scheduling**: Consolidated weekly schedule before 6am on Mondays
- **Semantic commits**: Properly formatted commit messages with `chore(deps):` prefix
- **Post-update operations**: Automatic `go mod tidy` and `npm dedupe` after updates
- **Lock file maintenance**: Automated lock file updates on the same schedule

## Key Differences

### Volta Version Management

**Before (Dependabot)**: Could not update Volta configurations in package.json files
```json
{
  "volta": {
    "node": "22.16.0",  // ❌ Never updated by Dependabot
    "npm": "11.4.1"     // ❌ Never updated by Dependabot  
  }
}
```

**After (Renovate)**: Automatically detects and updates Volta configurations
```json
{
  "volta": {
    "node": "22.16.0",  // ✅ Will be updated by Renovate
    "npm": "11.4.1"     // ✅ Will be updated by Renovate
  }
}
```

### Grouping Strategy

**Before**: Basic pattern matching in Dependabot
```yaml
groups:
  cloud-sdks:
    patterns:
      - "cloud.google.com/*"
      - "github.com/Azure/azure-sdk-for-go/*"
```

**After**: Regex-based matching with more flexibility in Renovate
```json
{
  "groupName": "cloud-sdks",
  "matchPackageNames": [
    "/^cloud\\.google\\.com//",
    "/^github\\.com/Azure/azure-sdk-for-go//"
  ]
}
```

### Scheduling

**Before**: Multiple separate weekly schedules
**After**: Unified schedule "before 6am on monday" across all package types

## Migration Steps

1. ✅ **Analysis**: Analyzed current Dependabot configuration and Volta usage
2. ✅ **Verification**: Confirmed Renovate supports all required package ecosystems  
3. ✅ **Configuration**: Created comprehensive `renovate.json` file
4. ✅ **Validation**: Validated configuration using renovate-config-validator
5. 🔄 **Testing**: Enable Renovate in GitHub repository settings
6. ⏳ **Cleanup**: Remove `.github/dependabot.yml` after confirming Renovate is working
7. ⏳ **Monitoring**: Monitor first few Renovate PRs to ensure proper functionality

## Expected Benefits

1. **Volta Updates**: Node.js and npm versions in Volta configurations will now be automatically updated
2. **Consistent Scheduling**: All dependency updates consolidated to Monday mornings
3. **Better Commit Messages**: Semantic commit format with proper prefixes
4. **Automated Cleanup**: Post-update operations like `go mod tidy` and `npm dedupe` 
5. **Lock File Maintenance**: Regular automated updates to lock files

## Configuration Files

- **New**: `renovate.json` - Main Renovate configuration
- **To Remove**: `.github/dependabot.yml` - After confirming Renovate is working

## Rollback Plan

If issues are encountered with Renovate:
1. Disable Renovate in GitHub repository settings
2. Restore `.github/dependabot.yml` 
3. Re-enable Dependabot in repository settings
4. Debug and fix Renovate configuration before trying again