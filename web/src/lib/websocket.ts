/**
 * WebSocket client for real-time notifications.
 *
 * Connects to the notification WebSocket endpoint with JWT auth
 * and automatically reconnects with exponential backoff on disconnect.
 */

export type NotificationSocketHandle = {
  close: () => void;
};

/**
 * Create a WebSocket connection for real-time notifications.
 *
 * @param token - JWT access token for authentication
 * @param onMessage - callback invoked with parsed notification data on each message
 * @returns handle with close() to teardown the connection and stop reconnection
 */
export function createNotificationSocket(
  token: string,
  onMessage: (data: any) => void
): NotificationSocketHandle {
  let ws: WebSocket | null = null;
  let backoff = 1000; // start at 1s
  let timer: ReturnType<typeof setTimeout> | null = null;
  let closed = false;

  const MAX_BACKOFF = 30000; // max 30s

  function connect() {
    if (closed) return;

    const protocol = location.protocol === 'https:' ? 'wss:' : 'ws:';
    const url = `${protocol}//${location.host}/api/v1/ws/notifications?token=${encodeURIComponent(token)}`;

    ws = new WebSocket(url);

    ws.onopen = () => {
      // Reset backoff on successful connection
      backoff = 1000;
    };

    ws.onmessage = (event) => {
      try {
        const data = JSON.parse(event.data);
        onMessage(data);
      } catch {
        // Ignore non-JSON messages
      }
    };

    ws.onclose = () => {
      if (closed) return;
      scheduleReconnect();
    };

    ws.onerror = () => {
      // onerror is always followed by onclose, so reconnect happens there
    };
  }

  function scheduleReconnect() {
    if (closed) return;

    // Add random jitter (0-25% of backoff) to prevent thundering herd
    const jitter = Math.random() * backoff * 0.25;
    const delay = backoff + jitter;

    timer = setTimeout(() => {
      connect();
    }, delay);

    // Double backoff for next attempt, capped at max
    backoff = Math.min(backoff * 2, MAX_BACKOFF);
  }

  // Start the initial connection
  connect();

  return {
    close() {
      closed = true;
      if (timer !== null) {
        clearTimeout(timer);
        timer = null;
      }
      if (ws) {
        ws.onclose = null; // prevent reconnect on intentional close
        ws.close();
        ws = null;
      }
    },
  };
}
