import "./style.css";

const WS_URL = "wss://h1uwqodz7a.execute-api.eu-west-1.amazonaws.com/dev/";

document.querySelector<HTMLDivElement>("#app")!.innerHTML = `
  <div id="main-wrapper">
    <main>
      <h1>globalvolu.me</h1>
      <input id="volume-slider" type="range" min="0" max="100" value="0" disabled />
      <div id="volume-value">0 / 100</div>
      <div id="status">Connecting...</div>
    </main>
  </div>
  <footer>
    <a href="https://github.com/vmorsell/global-volume/tree/main/client" target="_blank">Python Client</a>
  </footer>
`;

const slider = document.getElementById("volume-slider") as HTMLInputElement;
const valueDisplay = document.getElementById("volume-value") as HTMLElement;
const status = document.getElementById("status") as HTMLElement;

function setVolume(vol: number) {
  slider.value = vol.toString();
  valueDisplay.textContent = `${vol} / 100`;
}

function setStatus(msg: string) {
  status.textContent = msg;
}

setStatus("Connecting...");

const ws = new WebSocket(WS_URL);

ws.onopen = () => {
  setStatus("Connected");

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
    setStatus("Received invalid message");
  }
};

ws.onclose = () => {
  setStatus("Disconnected from server");
};

ws.onerror = () => {
  setStatus("WebSocket error");
};
