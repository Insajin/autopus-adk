# Demo GIFs

Terminal demo GIFs generated with [VHS](https://github.com/charmbracelet/vhs).

## Prerequisites

```bash
# macOS
brew install vhs

# Or via Go
go install github.com/charmbracelet/vhs@latest
```

VHS requires `ffmpeg` and `ttyd` (installed automatically by Homebrew).

## Generate GIFs

```bash
# Generate all demos
vhs demo/hero.tape
vhs demo/doctor.tape
vhs demo/status.tape
vhs demo/quickstart.tape

# Or generate all at once
for tape in demo/*.tape; do vhs "$tape"; done
```

## Files

| Tape | Output | Description |
|------|--------|-------------|
| `hero.tape` | `hero.gif` | Full hero demo for README top |
| `doctor.tape` | `doctor.gif` | Health check showcase |
| `status.tape` | `status.gif` | SPEC dashboard |
| `quickstart.tape` | `quickstart.gif` | Quick start flow |

## Regenerating

After changing CLI output or adding features, regenerate GIFs:

```bash
for tape in demo/*.tape; do vhs "$tape"; done
git add demo/*.gif
```
