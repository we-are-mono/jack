# Jack Release Process

This document describes the process for creating a new release of Jack, including building binaries for multiple architectures, creating packages, and publishing to GitHub.

## Overview

Jack supports multiple architectures and package formats:

**Architectures:**
- linux/arm64 (ARM 64-bit)
- linux/amd64 (x86_64)

**Package Formats:**
- `.deb` - Debian/Ubuntu packages
- `.tar.gz` - Portable archives with install script

## Prerequisites

### Required Tools

1. **Go 1.24+** - For building binaries
   ```bash
   go version
   ```

2. **dpkg-buildpackage** - For building Debian packages
   ```bash
   sudo apt install dpkg-dev debhelper
   ```

3. **GitHub CLI (gh)** - For creating GitHub releases
   ```bash
   # Install
   sudo apt install gh
   # Or from: https://cli.github.com/

   # Authenticate
   gh auth login
   ```

4. **Git** - With repository access
   ```bash
   git remote -v
   ```

### Repository State

- Working directory must be clean (no uncommitted changes)
- All tests must pass
- Integration tests verified
- Version number decided

## Release Steps

### 1. Prepare the Release

```bash
# Ensure you're on the main branch
git checkout master

# Pull latest changes
git pull origin master

# Verify tests pass
sg docker -c "make test-integration"

# Verify linting
make lint
make deadcode
```

### 2. Choose Version Number

Follow [Semantic Versioning](https://semver.org/):

- **Major version** (v1.0.0 → v2.0.0): Breaking changes
- **Minor version** (v0.1.0 → v0.2.0): New features, backwards compatible
- **Patch version** (v0.1.0 → v0.1.1): Bug fixes

Example: `v0.1.0`

### 3. Build for All Architectures

```bash
# Build binaries for arm64 and amd64
./scripts/build-release.sh v0.1.0

# Verify builds
ls -lh release/
```

This creates:
```
release/
├── jack-v0.1.0-linux-arm64/
│   ├── jack
│   ├── jack-plugin-firewall
│   ├── jack-plugin-dnsmasq
│   ├── jack-plugin-wireguard
│   ├── jack-plugin-monitoring
│   ├── jack-plugin-leds
│   └── jack-plugin-sqlite3
└── jack-v0.1.0-linux-amd64/
    └── (same files)
```

### 4. Create Packages

```bash
# Create .deb and .tar.gz packages
./scripts/package-release.sh v0.1.0

# Verify packages
ls -lh release/packages/
```

This creates:
```
release/packages/
├── jack_0.1.0_arm64.deb
├── jack_0.1.0_arm64.deb.sha256
├── jack_0.1.0_amd64.deb
├── jack_0.1.0_amd64.deb.sha256
├── jack-v0.1.0-linux-arm64.tar.gz
├── jack-v0.1.0-linux-arm64.tar.gz.sha256
├── jack-v0.1.0-linux-amd64.tar.gz
└── jack-v0.1.0-linux-amd64.tar.gz.sha256
```

### 5. Test Packages (Optional but Recommended)

#### Test .deb Package

```bash
# On an amd64 test machine
sudo apt install ./release/packages/jack_0.1.0_amd64.deb
jack --version
systemctl status jack
sudo apt remove jack
```

#### Test .tar.gz Package

```bash
# Extract
tar xzf release/packages/jack-v0.1.0-linux-amd64.tar.gz
cd jack-v0.1.0-linux-amd64

# Install
sudo ./install.sh

# Test
jack --version
systemctl status jack
```

### 6. Create GitHub Release

```bash
# Create release and upload packages
./scripts/create-github-release.sh v0.1.0

# For pre-releases (beta, RC, etc.)
./scripts/create-github-release.sh v0.2.0-beta.1 true
```

This will:
1. Create git tag `v0.1.0` if it doesn't exist
2. Push tag to GitHub
3. Generate release notes
4. Create GitHub release
5. Upload all packages (.deb, .tar.gz, checksums)

### 7. Verify Release

1. Visit the releases page: https://github.com/we-are-mono/jack/releases
2. Verify all 8 files are uploaded:
   - 2 .deb packages (arm64, amd64)
   - 2 .deb checksums
   - 2 .tar.gz archives
   - 2 .tar.gz checksums
3. Test download links
4. Verify checksums match

### 8. Announce Release

- Update README.md if needed
- Post announcement (if applicable)
- Notify team

## Troubleshooting

### Build Fails for Specific Architecture

```bash
# Build only one architecture
JACK_RELEASE_ARCHS="arm64" ./scripts/build-release.sh v0.1.0
```

### Debian Package Build Fails

```bash
# Check debian/rules syntax
make -f debian/rules clean

# Build manually with verbose output
DEB_BUILD_OPTIONS=nocheck dpkg-buildpackage -aamd64 -us -uc -b
```

### GitHub Release Fails

```bash
# Check authentication
gh auth status

# Create release manually
gh release create v0.1.0 \
    --title "Jack v0.1.0" \
    --notes "Release notes here" \
    release/packages/*
```

### Tag Already Exists

```bash
# Delete local tag
git tag -d v0.1.0

# Delete remote tag
git push origin :refs/tags/v0.1.0

# Recreate tag
git tag -a v0.1.0 -m "Release v0.1.0"
git push origin v0.1.0
```

## Rolling Back a Release

If a release needs to be rolled back:

```bash
# Delete GitHub release
gh release delete v0.1.0

# Delete tag from remote
git push origin :refs/tags/v0.1.0

# Delete local tag
git tag -d v0.1.0

# Clean up artifacts
rm -rf release/
```

## Manual Release (Without Scripts)

If you need to create a release manually:

### 1. Build Binaries

```bash
# ARM64
GOARCH=arm64 ./build.sh

# AMD64
GOARCH=amd64 ./build.sh
```

### 2. Create Debian Package

```bash
# Clean
make clean

# Build
DEB_BUILD_OPTIONS=nocheck dpkg-buildpackage -aarm64 -us -uc -b
```

### 3. Create tar.gz

```bash
tar czf jack-v0.1.0-linux-arm64.tar.gz \
    -C bin \
    jack jack-plugin-*
```

### 4. Upload to GitHub

```bash
gh release create v0.1.0 \
    --title "Jack v0.1.0" \
    --notes-file release-notes.md \
    jack-v0.1.0-linux-arm64.tar.gz \
    jack_0.1.0_arm64.deb
```

## Best Practices

1. **Always run tests before releasing**
   ```bash
   sg docker -c "make test-integration"
   ```

2. **Always create release notes** documenting:
   - New features
   - Bug fixes
   - Breaking changes
   - Migration guides (if needed)

3. **Use semantic versioning** consistently

4. **Tag releases** with annotated tags:
   ```bash
   git tag -a v0.1.0 -m "Release v0.1.0"
   ```

5. **Never delete releases** unless absolutely necessary (security, critical bugs)

6. **Keep release artifacts** for at least 1 year

7. **Test packages** before publishing when possible

## Automated Releases (Future)

In the future, this process will be automated with GitHub Actions:

- On tag push → Build for all architectures
- Run tests in CI
- Create packages
- Create GitHub release
- Upload artifacts

Configuration will be in `.github/workflows/release.yml` (not yet implemented).

## Support

For questions about the release process:
- Open an issue: https://github.com/we-are-mono/jack/issues
- Contact: team@mono.com
