#!/usr/bin/env python3
import asyncio
import json
import logging
import re
import subprocess
import sys
from typing import Optional

import websockets

try:
    from websockets.client import ClientProtocol
except ImportError:
    ClientProtocol = websockets.WebSocketClientProtocol  # type: ignore[misc,assignment]

SERVER_URL = "wss://api.globalvolu.me/ws"
POLL_INTERVAL = 0.5
VOLUME_MIN = 0
VOLUME_MAX = 100

logging.basicConfig(
    level=logging.INFO,
    format="[%(levelname)s] %(message)s"
)
logger = logging.getLogger(__name__)


class VolumeError(Exception):
    pass


class PlatformNotSupportedError(VolumeError):
    pass


async def get_volume() -> int:
    try:
        if sys.platform == "darwin":
            out = subprocess.check_output(
                ["osascript", "-e", "output volume of (get volume settings)"],
                stderr=subprocess.DEVNULL
            )
            return int(out.strip())
        elif sys.platform.startswith("linux"):
            out = subprocess.check_output(
                ["amixer", "get", "Master"],
                stderr=subprocess.DEVNULL
            ).decode()
            import re
            match = re.search(r"\[(\d+)%\]", out)
            if match:
                return int(match.group(1))
            raise VolumeError("Could not parse volume from amixer output")
        else:
            raise PlatformNotSupportedError(f"Platform {sys.platform} is not supported")
    except subprocess.CalledProcessError as e:
        raise VolumeError(f"Failed to get volume: {e}") from e
    except (ValueError, AttributeError) as e:
        raise VolumeError(f"Failed to parse volume: {e}") from e


async def set_volume(volume: int) -> None:
    if not (VOLUME_MIN <= volume <= VOLUME_MAX):
        raise ValueError(f"Volume must be between {VOLUME_MIN} and {VOLUME_MAX}")

    try:
        if sys.platform == "darwin":
            subprocess.check_call(
                ["osascript", "-e", f"set volume output volume {volume}"],
                stderr=subprocess.DEVNULL
            )
        elif sys.platform.startswith("linux"):
            subprocess.check_call(
                ["amixer", "set", "Master", f"{volume}%"],
                stderr=subprocess.DEVNULL
            )
        else:
            raise PlatformNotSupportedError(f"Platform {sys.platform} is not supported")
    except subprocess.CalledProcessError as e:
        raise VolumeError(f"Failed to set volume: {e}") from e


async def volume_watcher(send_queue: asyncio.Queue) -> None:
    try:
        last_volume = await get_volume()
    except VolumeError as e:
        logger.error(f"Failed to get initial volume: {e}")
        return

    while True:
        await asyncio.sleep(POLL_INTERVAL)
        try:
            current_volume = await get_volume()
            if current_volume != last_volume:
                await send_queue.put(current_volume)
                last_volume = current_volume
        except VolumeError as e:
            logger.warning(f"Failed to get volume: {e}")
        except Exception as e:
            logger.error(f"Unexpected error in volume watcher: {e}", exc_info=True)


async def sender(ws: ClientProtocol, send_queue: asyncio.Queue) -> None:
    while True:
        try:
            volume = await send_queue.get()
            logger.info(f"Publishing volume: {volume}%")
            message = json.dumps({"action": "reqVolumeChange", "volume": volume})
            await ws.send(message)
        except websockets.exceptions.ConnectionClosed:
            logger.info("WebSocket connection closed")
            break
        except Exception as e:
            logger.error(f"Error in sender: {e}", exc_info=True)


async def receiver(ws: ClientProtocol) -> None:
    client_count: Optional[int] = None
    
    async for message in ws:
        try:
            data = json.loads(message)
            if not isinstance(data, dict):
                continue
                
            message_type = data.get("type")
            
            if message_type == "clients":
                client_count = data.get("clients", 0)
                logger.info(f"{client_count} client(s) connected")
            elif message_type == "volume":
                volume = data.get("volume", 0)
                if not (VOLUME_MIN <= volume <= VOLUME_MAX):
                    logger.warning(f"Received invalid volume: {volume}")
                    continue
                logger.info(f"Got new volume: {volume}%")
                try:
                    await set_volume(volume)
                except VolumeError as e:
                    logger.error(f"Failed to set volume: {e}")
        except json.JSONDecodeError as e:
            logger.warning(f"Failed to parse message: {e}")
        except Exception as e:
            logger.error(f"Error in receiver: {e}", exc_info=True)


async def main() -> None:
    print("\U0001F310 Global Volume Sync")
    print("\u2500" * 28)
    
    connection_attempts = 0
    max_retries = 5
    
    while connection_attempts < max_retries:
        connection_attempts += 1
        
        if connection_attempts > 1:
            wait_time = min(10 * (connection_attempts - 1), 30)
            logger.info(f"Retrying connection (attempt {connection_attempts}/{max_retries}) in {wait_time}s...")
            await asyncio.sleep(wait_time)
        
        logger.info(f"Connecting to {SERVER_URL}...")
        
        try:
            async with websockets.connect(SERVER_URL) as ws:
                connection_attempts = 0  # Reset on successful connection
                logger.info(f"Connected to {SERVER_URL}")
                
                # Request initial state
                await ws.send(json.dumps({"action": "getVolume"}))
                await ws.send(json.dumps({"action": "getConnectedClientsCount"}))
                logger.info("Requested current volume and client count")
                
                # Get initial local volume
                try:
                    local_volume = await get_volume()
                    logger.info(f"Current local volume: {local_volume}%")
                except VolumeError as e:
                    logger.warning(f"Could not get local volume: {e}")
                
                # Start volume watcher and message handlers
                send_queue: asyncio.Queue = asyncio.Queue()
                asyncio.create_task(volume_watcher(send_queue))
                
                await asyncio.gather(
                    sender(ws, send_queue),
                    receiver(ws)
                )
        except websockets.exceptions.InvalidURI:
            logger.error(f"Invalid server URL: {SERVER_URL}")
            sys.exit(1)
        except websockets.exceptions.InvalidStatus as e:
            match = re.search(r'HTTP (\d+)', str(e))
            status_code = int(match.group(1)) if match else None
            
            if status_code == 503:
                logger.warning("Connection rejected - server may be at capacity")
                if connection_attempts >= max_retries:
                    logger.error("Max retry attempts reached. Please try again later.")
                    sys.exit(1)
                continue
            
            logger.error(f"Server rejected connection: HTTP {status_code}" if status_code else "Server rejected connection")
            if connection_attempts >= max_retries:
                sys.exit(1)
            continue
        except websockets.exceptions.ConnectionClosed:
            logger.info("Connection closed by server")
            if connection_attempts >= max_retries:
                logger.error("Max retry attempts reached")
                sys.exit(1)
            continue
        except KeyboardInterrupt:
            logger.info("Interrupted by user")
            sys.exit(0)
        except Exception as e:
            logger.error(f"Connection error: {e}")
            if connection_attempts >= max_retries:
                logger.error("Max retry attempts reached")
                sys.exit(1)
            continue
    
    logger.error("Failed to establish connection after multiple attempts")
    sys.exit(1)


if __name__ == "__main__":
    try:
        asyncio.run(main())
    except KeyboardInterrupt:
        print("\nExiting...")
