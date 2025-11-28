# /// script
# dependencies = ["websockets>=15.0"]
# ///
"""
A simple, reviewable Python client for global volume sync.
- Connects to a WebSocket relay server
- Listens for local volume changes (macOS/Linux)
- Sends/receives volume updates
- Sets system volume on update

Run with: uv run client.py
"""
import asyncio
import json
import os
import subprocess
import sys
import websockets

SERVER_URL = "wss://api.globalvolu.me/ws"
POLL_INTERVAL = 0.5  # seconds

async def get_volume():
    if sys.platform == "darwin":
        out = subprocess.check_output([
            "osascript", "-e", "output volume of (get volume settings)"
        ])
        return int(out.strip())
    elif sys.platform.startswith("linux"):
        out = subprocess.check_output(["amixer", "get", "Master"]).decode()
        import re
        m = re.search(r"\[(\d+)%\]", out)
        return int(m.group(1)) if m else 0
    else:
        raise NotImplementedError("Platform not supported")

async def set_volume(vol):
    if sys.platform == "darwin":
        subprocess.call([
            "osascript", "-e", f"set volume output volume {vol}"
        ])
    elif sys.platform.startswith("linux"):
        subprocess.call(["amixer", "set", "Master", f"{vol}%"])
    else:
        raise NotImplementedError("Platform not supported")

async def volume_watcher(send_queue):
    last = await get_volume()

    while True:
        await asyncio.sleep(POLL_INTERVAL)
        try:
            v = await get_volume()
            if v != last:
                await send_queue.put(v)
                last = v
        except Exception:
            pass

async def main():
    print("\U0001F310 Global Volume Sync")
    print("\u2500" * 28)
    print("[INFO] Connecting to sync server...")
    user_count = None
    async with websockets.connect(SERVER_URL) as ws:
        print(f"[INFO] Connected to sync server at {SERVER_URL}")

        await ws.send(json.dumps({"action": "getState"}))

        send_queue = asyncio.Queue()
        asyncio.create_task(volume_watcher(send_queue))

        local_vol = await get_volume()
        print(f"[INFO] Local volume: {local_vol}%")

        async def sender():
            while True:
                v = await send_queue.get()
                print(f"[INFO] Local volume: {v}%")
                await ws.send(json.dumps({"action": "reqVolumeChange", "volume": v}))

        async def receiver():
            nonlocal user_count
            async for msg in ws:
                try:
                    data = json.loads(msg)
                    if isinstance(data, dict):
                        if "users" in data:
                            user_count = data["users"]
                            print(f"[INFO] {user_count} users connected.")
                        if "volume" in data:
                            volume = data["volume"]
                            print(f"[INFO] Volume set to {volume}%.")
                            await set_volume(volume)
                except Exception:
                    pass
        await asyncio.gather(sender(), receiver())

if __name__ == "__main__":
    try:
        asyncio.run(main())
    except KeyboardInterrupt:
        print("Exiting...")
