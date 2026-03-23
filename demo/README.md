# Demo GIFs

Terminal demo GIF generated with [VHS](https://github.com/charmbracelet/vhs) and optimized with [gifsicle](https://www.lcdf.org/gifsicle/).

## Prerequisites

```bash
brew install vhs gifsicle
```

## Generate

```bash
cd demo
vhs hero.tape
gifsicle -O3 --lossy=80 hero.gif -o hero.opt.gif && mv hero.opt.gif hero.gif
```

## Files

| File | Description |
|------|-------------|
| `simulate-claude.sh` | ANSI escape codes로 Claude Code UI 재현 |
| `hero.tape` | VHS 스크립트 — Hide/Show로 셋업 숨기고 시뮬레이션 녹화 |
| `hero.gif` | README 상단 데모 — Claude Code에서 plan → go → sync |
