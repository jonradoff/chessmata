"""WebSocket client for real-time game updates."""

import json
import socket
import ssl
import threading
import time
from typing import Callable, Optional, Dict, Any
from urllib.parse import urlparse
import base64
import hashlib
import os


class WebSocketError(Exception):
    """WebSocket connection error."""
    pass


class SimpleWebSocket:
    """A minimal WebSocket client implementation.

    This is a simplified WebSocket client that doesn't require external dependencies.
    It implements the basic WebSocket protocol (RFC 6455) for text messages.
    """

    def __init__(self, url: str, on_message: Optional[Callable[[str], None]] = None):
        self.url = url
        self.on_message = on_message
        self._socket: Optional[socket.socket] = None
        self._connected = False
        self._receive_thread: Optional[threading.Thread] = None
        self._stop_event = threading.Event()

    def connect(self) -> None:
        """Establish WebSocket connection."""
        parsed = urlparse(self.url)

        # Determine if SSL
        use_ssl = parsed.scheme in ('wss', 'https')
        port = parsed.port or (443 if use_ssl else 80)
        host = parsed.hostname

        # Create socket
        sock = socket.socket(socket.AF_INET, socket.SOCK_STREAM)
        sock.settimeout(30)

        if use_ssl:
            context = ssl.create_default_context()
            sock = context.wrap_socket(sock, server_hostname=host)

        try:
            sock.connect((host, port))
        except Exception as e:
            raise WebSocketError(f"Failed to connect to {host}:{port}: {e}")

        self._socket = sock

        # Perform WebSocket handshake
        key = base64.b64encode(os.urandom(16)).decode('utf-8')
        path = parsed.path
        if parsed.query:
            path += f'?{parsed.query}'

        handshake = (
            f"GET {path} HTTP/1.1\r\n"
            f"Host: {host}\r\n"
            f"Upgrade: websocket\r\n"
            f"Connection: Upgrade\r\n"
            f"Sec-WebSocket-Key: {key}\r\n"
            f"Sec-WebSocket-Version: 13\r\n"
            f"\r\n"
        )

        sock.sendall(handshake.encode('utf-8'))

        # Read response
        response = b''
        while b'\r\n\r\n' not in response:
            chunk = sock.recv(1024)
            if not chunk:
                raise WebSocketError("Connection closed during handshake")
            response += chunk

        response_str = response.decode('utf-8')

        if '101' not in response_str:
            raise WebSocketError(f"WebSocket handshake failed: {response_str[:200]}")

        self._connected = True
        self._stop_event.clear()

        # Start receive thread
        self._receive_thread = threading.Thread(target=self._receive_loop, daemon=True)
        self._receive_thread.start()

    def close(self) -> None:
        """Close the WebSocket connection."""
        self._stop_event.set()
        self._connected = False

        if self._socket:
            try:
                # Send close frame
                self._send_frame(b'', opcode=0x8)
            except Exception:
                pass

            try:
                self._socket.close()
            except Exception:
                pass

            self._socket = None

    def send(self, message: str) -> None:
        """Send a text message."""
        if not self._connected:
            raise WebSocketError("Not connected")

        data = message.encode('utf-8')
        self._send_frame(data, opcode=0x1)

    def _send_frame(self, data: bytes, opcode: int = 0x1) -> None:
        """Send a WebSocket frame."""
        frame = bytearray()

        # First byte: FIN + opcode
        frame.append(0x80 | opcode)

        # Masking is required for client-to-server
        mask_bit = 0x80
        length = len(data)

        if length <= 125:
            frame.append(mask_bit | length)
        elif length <= 65535:
            frame.append(mask_bit | 126)
            frame.extend(length.to_bytes(2, 'big'))
        else:
            frame.append(mask_bit | 127)
            frame.extend(length.to_bytes(8, 'big'))

        # Masking key
        mask = os.urandom(4)
        frame.extend(mask)

        # Masked payload
        for i, byte in enumerate(data):
            frame.append(byte ^ mask[i % 4])

        self._socket.sendall(frame)

    def _receive_loop(self) -> None:
        """Background thread to receive messages."""
        buffer = b''

        while not self._stop_event.is_set() and self._connected:
            try:
                self._socket.settimeout(1.0)
                chunk = self._socket.recv(4096)

                if not chunk:
                    break

                buffer += chunk

                # Try to parse frames from buffer
                while len(buffer) >= 2:
                    # First byte
                    first_byte = buffer[0]
                    opcode = first_byte & 0x0F

                    # Second byte
                    second_byte = buffer[1]
                    masked = (second_byte & 0x80) != 0
                    payload_len = second_byte & 0x7F

                    header_len = 2

                    if payload_len == 126:
                        if len(buffer) < 4:
                            break
                        payload_len = int.from_bytes(buffer[2:4], 'big')
                        header_len = 4
                    elif payload_len == 127:
                        if len(buffer) < 10:
                            break
                        payload_len = int.from_bytes(buffer[2:10], 'big')
                        header_len = 10

                    if masked:
                        header_len += 4

                    total_len = header_len + payload_len

                    if len(buffer) < total_len:
                        break

                    # Extract payload
                    if masked:
                        mask = buffer[header_len - 4:header_len]
                        payload = bytearray()
                        for i, byte in enumerate(buffer[header_len:total_len]):
                            payload.append(byte ^ mask[i % 4])
                    else:
                        payload = buffer[header_len:total_len]

                    buffer = buffer[total_len:]

                    # Handle frame
                    if opcode == 0x1:  # Text frame
                        message = payload.decode('utf-8')
                        if self.on_message:
                            self.on_message(message)
                    elif opcode == 0x8:  # Close frame
                        self._connected = False
                        break
                    elif opcode == 0x9:  # Ping
                        self._send_frame(payload, opcode=0xA)  # Pong

            except socket.timeout:
                continue
            except Exception as e:
                if not self._stop_event.is_set():
                    print(f"WebSocket error: {e}")
                break

        self._connected = False

    @property
    def connected(self) -> bool:
        return self._connected


class GameWebSocket:
    """WebSocket client for game updates."""

    def __init__(
        self,
        server_url: str,
        session_id: str,
        player_id: str,
        on_game_update: Optional[Callable[[Dict[str, Any]], None]] = None,
        on_opponent_move: Optional[Callable[[Dict[str, Any]], None]] = None,
        on_player_joined: Optional[Callable[[Dict[str, Any]], None]] = None,
        on_resignation: Optional[Callable[[Dict[str, Any]], None]] = None,
        on_error: Optional[Callable[[str], None]] = None,
    ):
        self.server_url = server_url
        self.session_id = session_id
        self.player_id = player_id
        self.on_game_update = on_game_update
        self.on_opponent_move = on_opponent_move
        self.on_player_joined = on_player_joined
        self.on_resignation = on_resignation
        self.on_error = on_error
        self._ws: Optional[SimpleWebSocket] = None

    def connect(self) -> None:
        """Connect to the game WebSocket."""
        # Convert HTTP(S) URL to WS(S)
        ws_url = self.server_url.replace('https://', 'wss://').replace('http://', 'ws://')
        ws_url = f"{ws_url}/ws/games/{self.session_id}?playerId={self.player_id}"

        self._ws = SimpleWebSocket(ws_url, on_message=self._handle_message)
        self._ws.connect()

    def disconnect(self) -> None:
        """Disconnect from the WebSocket."""
        if self._ws:
            self._ws.close()
            self._ws = None

    def _handle_message(self, message: str) -> None:
        """Handle incoming WebSocket message."""
        try:
            data = json.loads(message)
            msg_type = data.get('type')

            if msg_type == 'game_update':
                if self.on_game_update:
                    self.on_game_update(data)
            elif msg_type == 'move':
                if self.on_opponent_move:
                    self.on_opponent_move(data)
            elif msg_type == 'player_joined':
                if self.on_player_joined:
                    self.on_player_joined(data)
            elif msg_type == 'resignation':
                if self.on_resignation:
                    self.on_resignation(data)
            elif msg_type == 'error':
                if self.on_error:
                    self.on_error(data.get('message', 'Unknown error'))
        except json.JSONDecodeError:
            if self.on_error:
                self.on_error(f"Invalid message: {message}")

    @property
    def connected(self) -> bool:
        return self._ws is not None and self._ws.connected
