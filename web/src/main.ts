import "./style.css";

const WS_URL = "wss://api.globalvolu.me/ws";
const BAR_WIDTH = 20;

document.querySelector<HTMLDivElement>("#app")!.innerHTML = `
  <div id="main-wrapper">
    <main>
      <header>
        <h1>globalvolu.me</h1>
        <p class="tagline">one knob. everyone turns it.</p>
      </header>
      <div id="volume-value">0%</div>
      <div id="volume-bar">[--------------------]</div>
      <div id="status">connecting...</div>
    </main>
  </div>
  <footer>
    join the chaos · <a href="https://github.com/vmorsell/global-volume/tree/main/clients/python" target="_blank">client.py</a>
  </footer>
`;

const valueDisplay = document.getElementById("volume-value") as HTMLElement;
const volumeBar = document.getElementById("volume-bar") as HTMLElement;
const status = document.getElementById("status") as HTMLElement;

function renderBar(vol: number): string {
  const filled = Math.round((vol / 100) * BAR_WIDTH);
  const empty = BAR_WIDTH - filled;

  const filledStr = "#".repeat(Math.max(0, filled - 1));
  const peakStr = filled > 0 ? "#" : "";
  const emptyStr = "-".repeat(empty);

  return `[<span class="filled">${filledStr}</span><span class="peak">${peakStr}</span>${emptyStr}]`;
}

function setVolume(vol: number) {
  valueDisplay.textContent = `${vol}%`;
  volumeBar.innerHTML = renderBar(vol);
}

function setStatus(msg: string) {
  status.textContent = msg;
}

setStatus("connecting...");

const ws = new WebSocket(WS_URL);

ws.onopen = () => {
  setStatus("connected");

  ws.send(JSON.stringify({ action: "getState" }));
};

ws.onmessage = (event) => {
  try {
    const data = JSON.parse(event.data);
    if (data.volume !== undefined) {
      setVolume(data.volume);
    }
    setStatus(`${data.users || 0} users connected`);
  } catch (e) {
    setStatus("invalid message");
  }
};

ws.onclose = () => {
  setStatus("disconnected");
};

ws.onerror = () => {
  setStatus("connection error");
};
