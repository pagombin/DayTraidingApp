"""Prerequisites check step - verifies Docker and Docker Compose are installed."""

from rich.console import Console

from utils.docker_check import check_docker, check_docker_compose

console = Console()


def run(config: dict) -> dict:
    """Check for Docker and Docker Compose, return status dict."""
    docker_ok = check_docker()
    compose_ok = check_docker_compose()

    if docker_ok:
        console.print("[green]  Docker .............. installed[/green]")
    else:
        console.print("[red]  Docker .............. NOT FOUND[/red]")

    if compose_ok:
        console.print("[green]  Docker Compose ...... installed[/green]")
    else:
        console.print("[red]  Docker Compose ...... NOT FOUND[/red]")

    if not docker_ok or not compose_ok:
        console.print(
            "\n[bold yellow]Warning:[/bold yellow] Some prerequisites are missing. "
            "The platform requires Docker and Docker Compose to run."
        )

    return {
        "prerequisites": {
            "docker_installed": docker_ok,
            "docker_compose_installed": compose_ok,
        }
    }
