import "./style.css";

const WS_URL = "wss://h1uwqodz7a.execute-api.eu-west-1.amazonaws.com/dev/";

document.querySelector<HTMLDivElement>("#app")!.innerHTML = `
  <h1>Global Volume</h1>
  <div style="margin:2em 0;">
    <input id="volume-slider" type="range" min="0" max="100" value="0" disabled style="width:300px;" />
    <div style="margin-top:1em;font-size:2em;">
      <span id="volume-value">0</span> / 100
    </div>
    <div id="status" style="margin-top:1em;color:gray;font-size:0.9em;"></div>
  </div>
`;

const slider = document.getElementById("volume-slider") as HTMLInputElement;
const valueDisplay = document.getElementById("volume-value") as HTMLElement;
const status = document.getElementById("status") as HTMLElement;

function setVolume(vol: number) {
  slider.value = vol.toString();
  valueDisplay.textContent = vol.toString();
}

function setStatus(msg: string) {
  status.textContent = msg;
}

setStatus("Connecting...");

const ws = new WebSocket(WS_URL);

ws.onopen = () => {
  setStatus("Connected");
};

ws.onmessage = (event) => {
  try {
    const data = JSON.parse(event.data);
    if (data.volume !== undefined) {
      setVolume(data.volume);
    }
    if (data.users !== undefined) {
      setStatus(`${data.users} users connected`);
    }
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
