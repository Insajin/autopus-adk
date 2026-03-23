# Demo GIFs

Terminal demo GIFs generated with [VHS](https://github.com/charmbracelet/vhs) and optimized with [gifsicle](https://www.lcdf.org/gifsicle/).

## Prerequisites

```bash
brew install vhs gifsicle
```

## Generate GIFs

```bash
cd demo
for tape in *.tape; do vhs "$tape"; done
for f in *.gif; do gifsicle -O3 --lossy=80 "$f" -o "$f.opt" && mv "$f.opt" "$f"; done
```

## Files

| Tape | Output | README Section |
|------|--------|----------------|
| `hero.tape` + `simulate-claude.sh` | `hero.gif` | Top — Claude Code session: plan → go → sync |
| `workflow.tape` | `workflow.gif` | "Three Commands to Ship" — CLI commands showcase |
