#!/usr/bin/env python3
"""PTY smoke test for keeping an update after project setup fails."""

import fcntl
import json
import os
import pathlib
import pty
import select
import struct
import subprocess
import tempfile
import termios
import time
import zipfile


def run_pty(binary: pathlib.Path, args: list[str], input_data: bytes = b"") -> bytes:
    master, slave = pty.openpty()
    fcntl.ioctl(slave, termios.TIOCSWINSZ, struct.pack("HHHH", 28, 110, 0, 0))
    env = os.environ.copy()
    env["UPDATE_CLI_TUI"] = "fullscreen"
    env.pop("CI", None)
    env.pop("GITHUB_ACTIONS", None)
    process = subprocess.Popen(
        [str(binary), *args],
        stdin=slave,
        stdout=slave,
        stderr=slave,
        env=env,
        close_fds=True,
    )
    os.close(slave)
    if input_data:
        os.write(master, input_data)

    output = bytearray()
    deadline = time.time() + 30
    while time.time() < deadline:
        ready, _, _ = select.select([master], [], [], 0.2)
        if ready:
            try:
                chunk = os.read(master, 65536)
            except OSError:
                break
            if not chunk:
                break
            output.extend(chunk)
        if process.poll() is not None and not ready:
            break
    process.wait(timeout=5)
    os.close(master)
    if process.returncode != 0:
        print(output.decode("utf-8", errors="replace"))
        raise SystemExit(f"expected exit 0, got {process.returncode}")
    return bytes(output)


def write_config(root: pathlib.Path, downloads: pathlib.Path) -> None:
    config_dir = root / ".updater-cli"
    config_dir.mkdir(parents=True, exist_ok=True)
    (config_dir / "config.json").write_text(
        json.dumps(
            {
                "schemaVersion": 6,
                "projectName": "keep-demo",
                "source": {"type": "download", "folder": str(downloads)},
                "releaseDir": "release",
                "currentDir": "current",
            }
        ),
        encoding="utf-8",
    )


def release_zip(downloads: pathlib.Path, version: str, files: dict[str, str]) -> pathlib.Path:
    archive = downloads / f"keep-demo-v{version}.zip"
    with zipfile.ZipFile(archive, "w", compression=zipfile.ZIP_DEFLATED) as zf:
        zf.writestr("VERSION", version + "\n")
        for name, content in files.items():
            zf.writestr(name, content)
    return archive


def main() -> int:
    repo = pathlib.Path(__file__).resolve().parents[1]
    binary = repo / "dist" / "update-cli"
    if not binary.exists():
        raise SystemExit(f"binary missing: {binary}")

    with tempfile.TemporaryDirectory(prefix="update-cli-keep-setup-") as tmp:
        base = pathlib.Path(tmp)
        root = base / "project"
        downloads = base / "downloads"
        downloads.mkdir(parents=True)
        write_config(root, downloads)

        v1 = release_zip(downloads, "1.0.0", {"app.txt": "old\n"})
        subprocess.run(
            [str(binary), "--update", str(v1), "--root", str(root), "--no-setup", "--no-wait", "--no-ui"],
            check=True,
            stdout=subprocess.DEVNULL,
            stderr=subprocess.DEVNULL,
        )

        v2 = release_zip(
            downloads,
            "2.0.0",
            {
                "app.txt": "new\n",
                "setup.yaml": (
                    "schemaVersion: 1\n"
                    "project:\n"
                    "  name: Keep Demo\n"
                    "steps:\n"
                    "  - id: fail\n"
                    "    name: Setup schlägt fehl\n"
                    "    run: printf partial > partial-setup.txt; exit 17\n"
                ),
            },
        )

        output = run_pty(
            binary,
            ["--update", str(v2), "--root", str(root), "--setup", "--no-wait"],
            input_data=b"j\n",
        )
        required = {
            "keep confirmation": "Update trotz fehlgeschlagenem Setup behalten?".encode("utf-8"),
            "modal yes": b"YES",
            "modal no": b"NO",
            "retry info": "Update wird ohne Projekt-Setup erneut installiert".encode("utf-8"),
            "kept result": "Update wurde trotz fehlgeschlagenem Setup beibehalten".encode("utf-8"),
        }
        missing = [name for name, marker in required.items() if marker not in output]
        if missing:
            print(output.decode("utf-8", errors="replace"))
            raise SystemExit("missing PTY markers: " + ", ".join(missing))

        current = root / "current"
        if (current / "app.txt").read_text(encoding="utf-8") != "new\n":
            raise SystemExit("kept update did not install the new current state")
        if (current / "partial-setup.txt").exists():
            raise SystemExit("partial failed-setup state survived the rollback/retry flow")
        if not (root / "release" / "2.0.0").is_dir():
            raise SystemExit("kept update did not activate release 2.0.0")

    print("setup failure keep-update PTY smoke passed")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
