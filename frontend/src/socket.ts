type EventCallback = (data: any) => void;

const WS_URL = "ws://localhost:8080/ws";
const RECONNECT_BASE_DELAY = 1000;
const RECONNECT_MAX_DELAY = 30000;

let ws: WebSocket | null = null;
let reconnectDelay = RECONNECT_BASE_DELAY;
let reconnectTimer: ReturnType<typeof setTimeout> | null = null;
let socketId: string | null = null;

const listeners: Map<string, Set<EventCallback>> = new Map();

function connect() {
  ws = new WebSocket(WS_URL);

  ws.onopen = () => {
    console.log("WebSocket connected");
    reconnectDelay = RECONNECT_BASE_DELAY;
  };

  ws.onmessage = (event) => {
    try {
      const msg = JSON.parse(event.data) as { event: string; data: any };

      // Capture our socket ID from the server
      if (msg.event === "connected" && msg.data?.socketId) {
        socketId = msg.data.socketId;
      }

      const callbacks = listeners.get(msg.event);
      if (callbacks) {
        callbacks.forEach((cb) => cb(msg.data));
      }
    } catch (err) {
      console.error("Failed to parse WebSocket message:", err);
    }
  };

  ws.onclose = () => {
    console.log("WebSocket disconnected, reconnecting...");
    scheduleReconnect();
  };

  ws.onerror = (err) => {
    console.error("WebSocket error:", err);
    ws?.close();
  };
}

function scheduleReconnect() {
  if (reconnectTimer) return;
  reconnectTimer = setTimeout(() => {
    reconnectTimer = null;
    reconnectDelay = Math.min(reconnectDelay * 2, RECONNECT_MAX_DELAY);
    connect();
  }, reconnectDelay);
}

export function sendEvent(event: string, data: Record<string, any> = {}) {
  if (ws && ws.readyState === WebSocket.OPEN) {
    ws.send(JSON.stringify({ event, data }));
  } else {
    console.warn("WebSocket not connected, message dropped:", event);
  }
}

export function onEvent(event: string, callback: EventCallback) {
  if (!listeners.has(event)) {
    listeners.set(event, new Set());
  }
  listeners.get(event)!.add(callback);
}

export function offEvent(event: string, callback?: EventCallback) {
  if (!callback) {
    listeners.delete(event);
  } else {
    listeners.get(event)?.delete(callback);
  }
}

export function getSocketId(): string | null {
  return socketId;
}

// Initialize connection immediately
connect();
