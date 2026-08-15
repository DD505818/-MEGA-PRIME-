import json
import os
import threading
from http.server import BaseHTTPRequestHandler, ThreadingHTTPServer


class HealthState:
    def __init__(self) -> None:
        self._ready = False
        self._dependency = "startup"
        self._lock = threading.Lock()

    def ready(self) -> None:
        with self._lock:
            self._ready = True
            self._dependency = ""

    def not_ready(self, dependency: str) -> None:
        with self._lock:
            self._ready = False
            self._dependency = dependency

    def snapshot(self) -> tuple[bool, str]:
        with self._lock:
            return self._ready, self._dependency


def start_health_server(state: HealthState) -> ThreadingHTTPServer:
    class Handler(BaseHTTPRequestHandler):
        def do_GET(self) -> None:
            if self.path in {"/health", "/health/live"}:
                self._json(200, {"status": "live"})
                return
            if self.path == "/health/ready":
                is_ready, dependency = state.snapshot()
                if is_ready:
                    self._json(200, {"status": "ready"})
                else:
                    self._json(
                        503,
                        {"status": "not_ready", "dependency": dependency},
                    )
                return
            self._json(404, {"status": "not_found"})

        def _json(self, status: int, payload: dict) -> None:
            body = json.dumps(payload, separators=(",", ":")).encode()
            self.send_response(status)
            self.send_header("Content-Type", "application/json")
            self.send_header("Content-Length", str(len(body)))
            self.end_headers()
            self.wfile.write(body)

        def log_message(self, format: str, *args) -> None:
            return

    port = int(os.getenv("HEALTH_PORT", "8090"))
    server = ThreadingHTTPServer(("0.0.0.0", port), Handler)
    threading.Thread(target=server.serve_forever, daemon=True).start()
    return server
