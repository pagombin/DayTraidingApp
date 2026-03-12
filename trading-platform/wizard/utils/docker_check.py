"""Utility to check if Docker and Docker Compose are installed."""

import shutil


def check_docker() -> bool:
    """Return True if Docker is found on PATH."""
    return shutil.which("docker") is not None


def check_docker_compose() -> bool:
    """Return True if Docker Compose is found on PATH.

    Checks for both the v2 plugin (docker compose) via the docker binary
    and the standalone v1 binary (docker-compose).
    """
    # Check for standalone docker-compose binary
    if shutil.which("docker-compose") is not None:
        return True

    # If docker exists, the compose subcommand is likely available (v2)
    if shutil.which("docker") is not None:
        return True

    return False
