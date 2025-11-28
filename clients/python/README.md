# Python Client

Syncs your system volume with everyone else connected to globalvolu.me.

## Requirements

- Python 3.8+
- [uv](https://docs.astral.sh/uv/) (recommended) or pip
- macOS or Linux

## Run

```bash
uv run client.py
```

Or with pip:

```bash
pip install websockets
python client.py
```

## What it does

1. Connects to the globalvolu.me WebSocket server
2. Watches your local volume for changes
3. When you change volume → broadcasts to all connected users
4. When someone else changes volume → updates your system volume

## Platforms

- **macOS**: Uses `osascript` to get/set volume
- **Linux**: Uses `amixer` (ALSA)
