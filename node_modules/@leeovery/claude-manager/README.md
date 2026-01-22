<h1 align="center">Claude Manager</h1>

<p align="center">
  <strong>npm Package for Managing Claude Code Skills & Commands</strong>
</p>

<p align="center">
  <a href="#about">About</a> •
  <a href="#installation">Installation</a> •
  <a href="#how-it-works">How It Works</a> •
  <a href="#cli-commands">CLI Commands</a> •
  <a href="#creating-plugins">Creating Plugins</a> •
  <a href="#available-plugins">Available Plugins</a> •
  <a href="#troubleshooting">Troubleshooting</a>
</p>

---

## Versions

| Version | Package Manager | Status     | Branch                                                     |
|---------|-----------------|------------|------------------------------------------------------------|
| 2.x     | npm             | **Active** | `main`                                                     |
| 1.x     | Composer        | Deprecated | [`v1`](https://github.com/leeovery/claude-manager/tree/v1) |

> **Note:** This package is installed automatically as a dependency of plugins.
> To migrate from v1, update your plugins to their v2 versions (npm-based).

---

## About

Claude Manager is an npm package that automatically manages [Claude Code](https://claude.ai/code) skills, commands, agents, hooks, and scripts across your projects.

**What it does:**
- Automatically installs skills, commands, agents, hooks, and scripts from plugin packages into your project's `.claude/` directory
- Copies assets so they're committed to your repository and available immediately
- Works with any project that has a `package.json` (Node.js, Laravel, Nuxt, etc.)
- Provides CLI tools for listing and managing installed plugins

**Why use it?**

Instead of manually copying skill files between projects, you can install them as npm packages and let the manager handle the rest. Update a skill package once, run `npm update`, and all your projects get the improvements.

## Installation

The manager is installed automatically as a dependency of plugin packages. When you install a Claude plugin:

```bash
npm install -D @your-org/claude-your-plugin
# or
pnpm add -D @your-org/claude-your-plugin
# or
yarn add -D @your-org/claude-your-plugin
```

The plugin's postinstall script copies assets to `.claude/`. That's it.

### Removing Plugins

Due to bugs in npm 7+ ([issue #3042](https://github.com/npm/cli/issues/3042)) and pnpm ([issue #3276](https://github.com/pnpm/pnpm/issues/3276)), preuninstall hooks don't run reliably. Remove files manually first:

```bash
npx claude-manager remove @leeovery/claude-laravel && npm rm @leeovery/claude-laravel
```

### pnpm Users

pnpm doesn't expose binaries from transitive dependencies, so install the manager directly alongside plugins:

```bash
pnpm add -D @leeovery/claude-manager @leeovery/claude-laravel
pnpm approve-builds  # approve when prompted
pnpm install         # triggers postinstall
```

## How It Works

1. Plugin packages have `@leeovery/claude-manager` as a dependency
2. Plugin's `postinstall` script copies assets to `.claude/`
3. A manifest (`.claude/.plugins-manifest.json`) tracks what's installed
4. Claude Code discovers the assets automatically

> **Note:** Plugins include `preuninstall` scripts but npm 7+ doesn't run them reliably ([issue #3042](https://github.com/npm/cli/issues/3042)). See [Removing Plugins](#removing-plugins) for the manual removal command.

**After installation, your project structure looks like:**

```
your-project/
├── .claude/
│   ├── .plugins-manifest.json
│   ├── skills/
│   │   └── laravel-actions/
│   │       └── skill.md
│   ├── commands/
│   │   └── artisan-make.md
│   ├── agents/
│   │   └── code-reviewer.md
│   ├── hooks/
│   │   └── pre-commit.sh
│   └── scripts/
│       └── build-check.sh
├── node_modules/
│   └── @your-org/
│       └── claude-your-plugin/
└── package.json
```

## CLI Commands

The manager provides a CLI tool for managing plugins:

| Command | Description |
|---------|-------------|
| `npx claude-manager list` | Show all installed plugins and their assets |
| `npx claude-manager install` | Sync all plugins from manifest (runs automatically) |
| `npx claude-manager add <package>` | Manually add a plugin |
| `npx claude-manager remove <package>` | Remove a plugin and its assets |

## Creating Plugins

Want to create your own skill or command packages?

### Plugin Requirements

1. Have `@leeovery/claude-manager` as a dependency
2. Add `postinstall` and `preuninstall` scripts (see example below)
3. Include asset directories (`skills/`, `commands/`, `agents/`, `hooks/`, `scripts/`)

### Example package.json

```json
{
    "name": "@your-org/claude-your-skills",
    "version": "1.0.0",
    "description": "Your custom skills for Claude Code",
    "license": "MIT",
    "dependencies": {
        "@leeovery/claude-manager": "^2.0.0"
    },
    "scripts": {
        "postinstall": "claude-manager add",
        "preuninstall": "claude-manager remove"
    }
}
```

The `postinstall` script copies assets when the plugin is installed. The `preuninstall` script cleans up when the plugin is removed.

### Plugin Structure

```
your-plugin/
├── skills/
│   ├── skill-one/
│   │   └── skill.md
│   └── skill-two/
│       └── skill.md
├── commands/
│   ├── command-one.md
│   └── command-two.md
├── agents/
│   └── agent-one.md
├── hooks/
│   └── pre-commit.sh
├── scripts/
│   └── build-check.sh
└── package.json
```

The manager auto-discovers `skills/`, `commands/`, `agents/`, `hooks/`, and `scripts/` directories—no additional configuration needed. All asset directories support nested subdirectories.

## Available Plugins

| Package | Description |
|---------|-------------|
| [@leeovery/claude-technical-workflows](https://github.com/leeovery/claude-technical-workflows) | Structured discussion & planning skills—research, discuss, specify, plan, implement, review |
| [@leeovery/claude-laravel](https://github.com/leeovery/claude-laravel) | Opinionated Laravel development patterns and practices |
| [@leeovery/claude-nuxt](https://github.com/leeovery/claude-nuxt) | Opinionated Nuxt 4 + Vue 3 development patterns |

## Manifest

The manager tracks installed plugins in `.claude/.plugins-manifest.json`:

```json
{
  "plugins": {
    "@your-org/claude-laravel": {
      "version": "1.0.0",
      "files": [
        "skills/laravel-actions",
        "commands/artisan-make.md"
      ]
    }
  }
}
```

This file should be committed to your repository. It ensures:
- Old files are cleaned up when plugins are updated or removed
- You can see what's installed at a glance

## Troubleshooting

### Assets not copied

Run the install command manually:

```bash
npx claude-manager install
```

### Skills not showing in Claude Code

Check that `.claude/` directories exist and contain files:

```bash
ls -la .claude/skills/
ls -la .claude/commands/
ls -la .claude/agents/
ls -la .claude/hooks/
ls -la .claude/scripts/
```

### Plugin not detected

Verify the plugin's package.json has:
- `@leeovery/claude-manager` as a dependency
- `postinstall` and `preuninstall` scripts
- A `skills/`, `commands/`, `agents/`, `hooks/`, or `scripts/` directory with content

## Requirements

- Node.js >= 18.0.0
- npm, pnpm, yarn, or bun

## Contributing

Contributions are welcome! Whether it's:

- Bug fixes
- Documentation improvements
- New features
- Discussion about approaches

Please open an issue first to discuss significant changes.

## License

MIT License. See [LICENSE](LICENSE) for details.

---

<p align="center">
  <sub>Built by <a href="https://github.com/leeovery">Lee Overy</a></sub>
</p>
