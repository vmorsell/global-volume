import "./style.css";

const WS_URL = "wss://api.globalvolu.me/ws";
const BAR_WIDTH = 20;
const VOLUME_MIN = 0;
const VOLUME_MAX = 100;

let ws: WebSocket | null = null;
let connectionAttempts = 0;
let lastConnectionTime = 0;

document.querySelector<HTMLDivElement>("#app")!.innerHTML = `
  <div id="main-wrapper">
    <main>
      <header>
        <h1>globalvolu.me</h1>
        <p class="tagline">one volume knob to rule them all</p>
      </header>
      <div id="volume-value">0%</div>
      <div id="volume-bar">[--------------------]</div>
      <div id="status">connecting...</div>
    </main>
  </div>
  <footer>
    join the community · <a href="https://github.com/vmorsell/global-volume/tree/main/clients/python" target="_blank">client.py</a>
  </footer>
`;

const valueDisplay = document.getElementById("volume-value");
const volumeBar = document.getElementById("volume-bar");
const status = document.getElementById("status");

if (!valueDisplay || !volumeBar || !status) {
  throw new Error("Failed to initialize DOM elements");
}

function renderBar(vol: number): string {
  const clampedVol = Math.max(VOLUME_MIN, Math.min(VOLUME_MAX, vol));
  const filled = Math.round((clampedVol / 100) * BAR_WIDTH);
  const empty = BAR_WIDTH - filled;

  const filledStr = "#".repeat(Math.max(0, filled - 1));
  const peakStr = filled > 0 ? "#" : "";
  const emptyStr = "-".repeat(empty);

  return `[<span class="filled">${filledStr}</span><span class="peak">${peakStr}</span>${emptyStr}]`;
}

function setVolume(vol: number): void {
  if (!valueDisplay || !volumeBar) {
    return;
  }
  const clampedVol = Math.max(VOLUME_MIN, Math.min(VOLUME_MAX, vol));
  valueDisplay.textContent = `${clampedVol}%`;
  volumeBar.innerHTML = renderBar(clampedVol);
}

function setStatus(msg: string): void {
  if (!status) {
    console.error("Status element not found");
    return;
  }
  status.textContent = msg;
}

function handleVolumeMessage(data: { volume?: number }): void {
  const volume = data.volume;
  if (
    typeof volume === "number" &&
    VOLUME_MIN <= volume &&
    volume <= VOLUME_MAX
  ) {
    setVolume(volume);
  }
}

function handleClientsMessage(data: { clients?: number }): void {
  const clients = data.clients;
  if (typeof clients === "number" && clients >= 0) {
    setStatus(`${clients} client${clients !== 1 ? "s" : ""} connected`);
  }
}

function handleMessage(event: MessageEvent): void {
  try {
    const data = JSON.parse(event.data);

    if (typeof data !== "object" || data === null) {
      return;
    }

    const messageType = data.type;

    if (messageType === "volume") {
      handleVolumeMessage(data);
    } else if (messageType === "clients") {
      handleClientsMessage(data);
    }
  } catch (e) {
    console.error("Failed to parse message:", e);
    setStatus("invalid message");
  }
}

function connect(): void {
  const now = Date.now();
  connectionAttempts++;

  setStatus("connecting...");
  lastConnectionTime = now;

  ws = new WebSocket(WS_URL);

  ws.onopen = () => {
    connectionAttempts = 0; // Reset on successful connection
    setStatus("connected");
    if (ws) {
      ws.send(JSON.stringify({ action: "getVolume" }));
      ws.send(JSON.stringify({ action: "getConnectedClientsCount" }));
    }
  };

  ws.onmessage = handleMessage;

  ws.onclose = (event) => {
    const connectionDuration = Date.now() - lastConnectionTime;
    ws = null;

    if (event.code === 1006) {
      if (connectionDuration < 500 && connectionAttempts > 1) {
        setStatus(
          "connection rejected - server may be at capacity, retrying in 10s..."
        );
        setTimeout(connect, 10000);
        return;
      }
      setStatus("connection failed - retrying...");
      const delay = Math.min(3000 * Math.pow(2, connectionAttempts - 1), 30000);
      setTimeout(connect, delay);
    } else if (event.code === 1000) {
      setStatus("disconnected");
    } else {
      setStatus(`disconnected (code: ${event.code})`);
      setTimeout(connect, 3000);
    }
  };

  ws.onerror = () => {
    if (connectionAttempts > 1 && Date.now() - lastConnectionTime < 500) {
      setStatus("connection rejected - server may be at capacity");
    } else {
      setStatus("connection error");
    }
  };
}

connect();
