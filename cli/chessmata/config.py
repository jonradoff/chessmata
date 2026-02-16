"""Configuration management for Chessmata CLI.

Stores configuration in ~/.config/chessmata/ following XDG Base Directory spec.
Credentials are stored separately in ~/.config/chessmata/credentials.json
"""

import json
import os
from pathlib import Path
from typing import Optional
from dataclasses import dataclass, asdict


def get_config_dir() -> Path:
    """Get the configuration directory, following XDG spec on Linux/Mac."""
    if os.name == 'nt':  # Windows
        config_dir = Path(os.environ.get('APPDATA', '~')).expanduser() / 'chessmata'
    else:  # Linux/Mac - use XDG_CONFIG_HOME or default to ~/.config
        xdg_config = os.environ.get('XDG_CONFIG_HOME', '')
        if xdg_config:
            config_dir = Path(xdg_config) / 'chessmata'
        else:
            config_dir = Path.home() / '.config' / 'chessmata'

    return config_dir


def ensure_config_dir() -> Path:
    """Ensure the configuration directory exists and return its path."""
    config_dir = get_config_dir()
    config_dir.mkdir(parents=True, exist_ok=True)

    # Set restrictive permissions on Unix systems
    if os.name != 'nt':
        os.chmod(config_dir, 0o700)

    return config_dir


@dataclass
class Config:
    """Application configuration."""
    server_url: str = "https://chessmata.metavert.io"
    email: str = ""

    @classmethod
    def load(cls) -> 'Config':
        """Load configuration from file."""
        config_file = get_config_dir() / 'config.json'

        if not config_file.exists():
            return cls()

        try:
            with open(config_file, 'r') as f:
                data = json.load(f)
                return cls(
                    server_url=data.get('server_url', cls.server_url),
                    email=data.get('email', cls.email),
                )
        except (json.JSONDecodeError, IOError):
            return cls()

    def save(self) -> None:
        """Save configuration to file."""
        config_dir = ensure_config_dir()
        config_file = config_dir / 'config.json'

        with open(config_file, 'w') as f:
            json.dump(asdict(self), f, indent=2)

        # Set restrictive permissions on Unix systems
        if os.name != 'nt':
            os.chmod(config_file, 0o600)

    def is_configured(self) -> bool:
        """Check if the client has been configured."""
        return bool(self.server_url)


@dataclass
class Credentials:
    """User credentials and session information."""
    access_token: Optional[str] = None
    user_id: Optional[str] = None
    email: Optional[str] = None
    display_name: Optional[str] = None
    elo_rating: Optional[int] = None

    @classmethod
    def load(cls) -> 'Credentials':
        """Load credentials from file."""
        creds_file = get_config_dir() / 'credentials.json'

        if not creds_file.exists():
            return cls()

        try:
            with open(creds_file, 'r') as f:
                data = json.load(f)
                return cls(
                    access_token=data.get('access_token'),
                    user_id=data.get('user_id'),
                    email=data.get('email'),
                    display_name=data.get('display_name'),
                    elo_rating=data.get('elo_rating'),
                )
        except (json.JSONDecodeError, IOError):
            return cls()

    def save(self) -> None:
        """Save credentials to file."""
        config_dir = ensure_config_dir()
        creds_file = config_dir / 'credentials.json'

        with open(creds_file, 'w') as f:
            json.dump(asdict(self), f, indent=2)

        # Set restrictive permissions on Unix systems (readable only by owner)
        if os.name != 'nt':
            os.chmod(creds_file, 0o600)

    def clear(self) -> None:
        """Clear credentials and delete the credentials file."""
        creds_file = get_config_dir() / 'credentials.json'

        if creds_file.exists():
            creds_file.unlink()

        self.access_token = None
        self.user_id = None
        self.email = None
        self.display_name = None
        self.elo_rating = None

    def is_logged_in(self) -> bool:
        """Check if user is logged in."""
        return bool(self.access_token)


def setup_interactive() -> Config:
    """Run interactive setup to configure the client."""
    print("\n=== Chessmata CLI Setup ===\n")

    config = Config.load()

    # Server URL
    default_url = config.server_url or "https://chessmata.metavert.io"
    server_url = input(f"Server URL [{default_url}]: ").strip()
    if not server_url:
        server_url = default_url

    # Email (for authentication)
    default_email = config.email or ""
    email_prompt = f"Email [{default_email}]: " if default_email else "Email (for login): "
    email = input(email_prompt).strip()
    if not email and default_email:
        email = default_email

    config = Config(
        server_url=server_url,
        email=email,
    )
    config.save()

    print(f"\nConfiguration saved to: {get_config_dir() / 'config.json'}")
    print("You can now use 'chessmata login' to authenticate.\n")

    return config
