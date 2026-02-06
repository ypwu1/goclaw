# ClawHub Subcommand Design Document

## Overview

ClawHub is the public skill registry for OpenClaw. It is a free service where all skills are public, open, and visible to everyone for sharing and reuse. A skill is a folder with a `SKILL.md` file (plus supporting text files). Users can browse skills in the web app or use the CLI to search, install, update, and publish skills.

- Website: https://clawhub.ai
- Documentation: https://docs.openclaw.ai/tools/clawhub

## Purpose

The `clawhub` subcommand in goclaw provides a Go-based CLI implementation for interacting with the ClawHub registry, allowing users to:

- Search for skills by name, tags, or natural language queries
- Download and install skill bundles
- Update installed skills
- Publish new skills and new versions of existing skills
- List installed skills
- Sync local skills with the registry

## Architecture

### Command Structure

```
goclaw clawhub [command] [options]
```

### Subcommands

1. **Auth Commands**
   - `login` - Authenticate with ClawHub (browser flow or token)
   - `logout` - Log out from current session
   - `whoami` - Display current authenticated user

2. **Search & Discovery**
   - `search [query]` - Search for skills using embeddings/vector search

3. **Install & Manage**
   - `install <slug>` - Install a skill from the registry
   - `update <slug>` - Update an installed skill
   - `update --all` - Update all installed skills
   - `list` - List all installed skills

4. **Publish & Sync**
   - `publish <path>` - Publish a skill to the registry
   - `sync` - Scan and publish/sync local skills

5. **Admin** (Owner/Moderator only)
   - `delete <slug>` - Delete a skill
   - `undelete <slug>` - Undelete a skill

## Configuration

### Global Options

- `--workdir <dir>` - Working directory (default: current dir; falls back to OpenClaw workspace)
- `--dir <dir>` - Skills directory, relative to workdir (default: `skills`)
- `--site <url>` - Site base URL (browser login)
- `--registry <url>` - Registry API base URL
- `--no-input` - Disable prompts (non-interactive mode)
- `-V, --version` - Print CLI version

### Environment Variables

- `CLAWHUB_SITE` - Override the site URL
- `CLAWHUB_REGISTRY` - Override the registry API URL
- `CLAWHUB_CONFIG_PATH` - Override where the CLI stores the token/config
- `CLAWHUB_WORKDIR` - Override the default workdir
- `CLAWHUB_DISABLE_TELEMETRY=1` - Disable telemetry on `sync`

### Storage

- **Lockfile**: `.clawhub/lock.json` under workdir - records installed skills
- **Config**: Stores auth tokens and CLI configuration
- **Skills Directory**: `<workdir>/skills` or `<workspace>/skills`

## Detailed Command Specifications

### 1. Authentication Commands

#### `login`

Authenticate with ClawHub using browser flow or API token.

```bash
goclaw clawhub login
goclaw clawhub login --token <token>
goclaw clawhub login --label <label>
goclaw clawhub login --no-browser --token <token>
```

Options:
- `--token <token>` - Paste an API token directly
- `--label <label>` - Label for stored token (default: "CLI token")
- `--no-browser` - Do not open browser (requires --token)

Flow:
1. If no token provided, open browser for OAuth flow
2. If --no-browser, require --token
3. Store token in config file
4. Verify authentication

#### `logout`

Remove stored authentication token.

```bash
goclaw clawhub logout
```

#### `whoami`

Display current authenticated user information.

```bash
goclaw clawhub whoami
```

### 2. Search Command

#### `search`

Search for skills using vector search (not just keyword matching).

```bash
goclaw clawhub search "[query]"
goclaw clawhub search "[query]" --limit 10
```

Options:
- `--limit <n>` - Maximum number of results to display

Output format:
```
[Slug] Display Name
⭐ Stars | ⤓ Downloads | ⤒ Updates
Tags: tag1, tag2, tag3
Description text...
```

### 3. Install Command

#### `install`

Install a skill from the registry to the local skills directory.

```bash
goclaw clawhub install <slug>
goclaw clawhub install <slug> --version 1.0.0
goclaw clawhub install <slug> --force
```

Options:
- `--version <version>` - Install a specific version (default: latest)
- `--force` - Overwrite if folder already exists

Process:
1. Query registry for skill metadata
2. Download version-specific zip file
3. Extract to skills directory
4. Update lockfile with installed version
5. Verify installation

### 4. Update Command

#### `update`

Update one or all installed skills to their latest versions.

```bash
goclaw clawhub update <slug>
goclaw clawhub update --all
goclaw clawhub update <slug> --version 1.2.0
goclaw clawhub update --all --force
```

Options:
- `--version <version>` - Update to specific version (single slug only)
- `--force` - Overwrite when local files don't match any published version
- `--all` - Update all installed skills

Process:
1. Read lockfile for current versions
2. Query registry for latest versions
3. Compare content hashes to detect local changes
4. Prompt if local changes detected (unless --force)
5. Download and install new versions
6. Update lockfile

### 5. List Command

#### `list`

Display all installed skills from the lockfile.

```bash
goclaw clawhub list
```

Output format:
```
Installed Skills:
================
[slug1] v1.0.0 - Display Name
[slug2] v2.1.3 - Another Skill
```

### 6. Publish Command

#### `publish`

Publish a skill folder to the ClawHub registry.

```bash
goclaw clawhub publish <path>
goclaw clawhub publish ./my-skill --slug my-skill --name "My Skill" --version 1.0.0
goclaw clawhub publish ./my-skill --changelog "Added new features" --tags latest,productivity
```

Required options:
- `--slug <slug>` - Skill slug (URL-friendly identifier)
- `--name <name>` - Display name
- `--version <version>` - Semver version (e.g., 1.0.0)

Optional:
- `--changelog <text>` - Changelog text (can be empty)
- `--tags <tags>` - Comma-separated tags (default: `latest`)

Requirements:
- Must be authenticated
- GitHub account must be at least one week old
- Skill folder must contain SKILL.md file

Process:
1. Validate skill folder structure
2. Read SKILL.md and metadata
3. Create bundle zip file
4. Upload to registry
5. Verify publication

### 7. Sync Command

#### `sync`

Scan local skills and publish new/updated ones to the registry.

```bash
goclaw clawhub sync
goclaw clawhub sync --all --dry-run
goclaw clawhub sync --root ~/other-skills --bump minor
goclaw clawhub sync --concurrency 8
```

Options:
- `--root <dir...>` - Extra scan roots (can specify multiple)
- `--all` - Upload everything without prompts
- `--dry-run` - Show what would be uploaded without doing it
- `--bump <type>` - Auto-bump version: patch|minor|major (default: patch)
- `--changelog <text>` - Changelog for updates
- `--tags <tags>` - Comma-separated tags (default: latest)
- `--concurrency <n>` - Concurrent registry checks (default: 4)

Scan order:
1. Current workdir skills folder
2. Fallback roots if none found (~/openclaw/skills, ~/.openclaw/skills)

Process:
1. Scan directories for skill folders (containing SKILL.md)
2. For each skill, check registry for existing versions
3. Compare content hashes to detect changes
4. Prompt for new/updated skills (unless --all)
5. Publish with auto-bumped versions
6. Send minimal telemetry snapshot (unless disabled)

### 8. Delete/Undelete Commands (Admin)

#### `delete`

Delete a skill from the registry (owner or admin only).

```bash
goclaw clawhub delete <slug> --yes
```

Options:
- `--yes` - Skip confirmation prompt

#### `undelete`

Undelete a previously deleted skill (owner or admin only).

```bash
goclaw clawhub undelete <slug> --yes
```

Options:
- `--yes` - Skip confirmation prompt

## Data Structures

### Lockfile Format (.clawhub/lock.json)

```json
{
  "version": "1.0.0",
  "skills": {
    "skill-slug": {
      "name": "Display Name",
      "version": "1.0.0",
      "installedAt": "2024-01-15T10:30:00Z",
      "hash": "sha256:abc123...",
      "tags": ["latest", "productivity"]
    }
  }
}
```

### Skill Metadata (SKILL.md)

```markdown
# Skill Name

Short description of what this skill does.

## Usage

How to use this skill...

## Requirements

Any dependencies or requirements...
```

### Registry API Response Format

```json
{
  "slug": "skill-slug",
  "name": "Display Name",
  "description": "Skill description",
  "versions": [
    {
      "version": "1.0.0",
      "changelog": "Initial release",
      "createdAt": "2024-01-15T10:30:00Z",
      "hash": "sha256:abc123..."
    }
  ],
  "tags": ["latest"],
  "stats": {
    "stars": 42,
    "downloads": 3652,
    "updates": 19
  }
}
```

## Error Handling

### Common Error Scenarios

1. **Not Authenticated**
   - Error: "Not logged in. Run 'goclaw clawhub login' first."
   - Resolution: Prompt user to authenticate

2. **Skill Not Found**
   - Error: "Skill 'xyz' not found in registry."
   - Resolution: Suggest search command

3. **Version Not Found**
   - Error: "Version 2.0.0 not found for skill 'xyz'. Available: 1.0.0, 1.1.0"
   - Resolution: List available versions

4. **Local Changes Detected**
   - Error: "Local files have modifications that don't match any published version."
   - Resolution: Prompt for --force or suggest committing changes

5. **Network Errors**
   - Error: "Failed to connect to registry: <details>"
   - Resolution: Retry with exponential backoff

6. **Invalid Skill Structure**
   - Error: "SKILL.md not found in skill directory."
   - Resolution: Validate and guide user

## Security Considerations

1. **Token Storage**: Store auth tokens securely in config file
2. **HTTPS Only**: All API communication over HTTPS
3. **Content Verification**: Verify downloaded files using checksums
4. **Path Sanitization**: Prevent directory traversal when extracting skills
5. **Rate Limiting**: Respect registry rate limits
6. **Telemetry Opt-out**: Allow disabling telemetry

## Testing Strategy

### Unit Tests

- Config file parsing and writing
- Lockfile management
- Version comparison and semver handling
- Content hash calculation
- URL path sanitization

### Integration Tests

- Registry API interactions (mocked)
- Download and extraction workflows
- Auth flow (with mock server)

### E2E Tests

- Full install workflow
- Full publish workflow
- Update workflow
- Sync workflow

## Implementation Phases

### Phase 1: Core Infrastructure
- [ ] Config and lockfile management
- [ ] HTTP client with registry API
- [ ] Auth token storage and retrieval
- [ ] Error handling framework

### Phase 2: Authentication
- [ ] Login command (browser flow)
- [ ] Login command (token flow)
- [ ] Logout command
- [ ] Whoami command

### Phase 3: Search and Discovery
- [ ] Search command
- [ ] List command
- [ ] Result formatting and display

### Phase 4: Install and Update
- [ ] Install command
- [ ] Update command (single)
- [ ] Update command (all)
- [ ] Version comparison logic
- [ ] Local change detection

### Phase 5: Publish and Sync
- [ ] Publish command
- [ ] Sync command
- [ ] Skill validation
- [ ] Bundle creation

### Phase 6: Admin Features
- [ ] Delete command
- [ ] Undelete command
- [ ] Moderation features

### Phase 7: Polish
- [ ] Progress indicators
- [ ] Color output
- [ ] Help text and documentation
- [ ] Shell completion

## Dependencies

### Go Packages

- `cobra` - CLI framework
- `spf13/viper` - Configuration management
- `hashicorp/go-version` - Semver handling
- `go-resty` - HTTP client
- `profmorcus/sha256` - Content hashing

### External Services

- ClawHub Registry API (https://clawhub.ai)
- ClawHub Web (for OAuth flow)

## Compatibility

- Go 1.21+
- macOS, Linux, Windows
- OpenClaw workspace format
