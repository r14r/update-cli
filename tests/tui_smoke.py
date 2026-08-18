#!/usr/bin/env python3
"""PTY smoke tests for fullscreen setup/update terminal contracts."""

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


def run_pty(
    binary: pathlib.Path,
    args: list[str],
    expected_exit: int,
    input_data: bytes = b"",
    cwd: pathlib.Path | None = None,
    env_overrides: dict[str, str] | None = None,
) -> bytes:
    master, slave = pty.openpty()
    fcntl.ioctl(slave, termios.TIOCSWINSZ, struct.pack("HHHH", 28, 110, 0, 0))
    env = os.environ.copy()
    env["UPDATE_CLI_TUI"] = "fullscreen"
    env.pop("NO_COLOR", None)
    if env_overrides:
        env.update(env_overrides)
    process = subprocess.Popen(
        [str(binary), *args],
        stdin=slave,
        stdout=slave,
        stderr=slave,
        env=env,
        cwd=str(cwd) if cwd is not None else None,
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
    if process.returncode != expected_exit:
        print(output.decode("utf-8", errors="replace"))
        raise SystemExit(
            f"TUI smoke failed: exit={process.returncode}, expected={expected_exit}, args={args}"
        )
    return bytes(output)


def require(output: bytes, markers: dict[str, bytes]) -> None:
    missing = [name for name, marker in markers.items() if marker not in output]
    if missing:
        print(output.decode("utf-8", errors="replace"))
        raise SystemExit(f"TUI smoke missing: {', '.join(missing)}")


def write_config(
    root: pathlib.Path,
    downloads: pathlib.Path,
    project: str,
    no_parameter: list[str] | None = None,
) -> None:
    config_dir = root / ".updater-cli"
    config_dir.mkdir(parents=True, exist_ok=True)
    (config_dir / "config.json").write_text(
        json.dumps(
            {
                "schemaVersion": 6,
                "projectName": project,
                "source": {"type": "download", "folder": str(downloads)},
                "releaseDir": "release",
                "currentDir": "current",
                "no parameter": no_parameter or ["check"],
            }
        ),
        encoding="utf-8",
    )


def release_zip(
    downloads: pathlib.Path, project: str, version: str, files: dict[str, str]
) -> pathlib.Path:
    archive = downloads / f"{project}-v{version}.zip"
    with zipfile.ZipFile(archive, "w", compression=zipfile.ZIP_DEFLATED) as zf:
        zf.writestr("VERSION", version + "\n")
        for name, content in files.items():
            zf.writestr(name, content)
    return archive


def main() -> int:
    root = pathlib.Path(__file__).resolve().parents[1]
    binary = root / "dist" / "update-cli"
    version = (root / "VERSION").read_text(encoding="utf-8").strip()
    header = f"Update CLI Version {version}".encode("utf-8")
    if not binary.exists():
        raise SystemExit(f"binary missing: {binary}")

    with tempfile.TemporaryDirectory(prefix="update-cli-tui-") as tmp:
        tmpdir = pathlib.Path(tmp)

        success_manifest = tmpdir / "success.yaml"
        (tmpdir / "VERSION").write_text("7.8.9\n", encoding="utf-8")
        success_manifest.write_text(
            "schemaVersion: 1\n"
            "project:\n"
            "  name: TUI Test\n"
            "  type: go\n"
            "  description: Prüft Umlaute äöü\n"
            "steps:\n"
            "  - id: noop\n"
            "    name: Prüfschritt\n"
            "    run: printf 'sichtbare-schrittausgabe\\n'\n",
            encoding="utf-8",
        )
        success = run_pty(
            binary, ["--setup-manifest", str(success_manifest), "--no-wait"], 0
        )
        require(
            success,
            {
                "alternate-screen enter": b"\x1b[?1049h",
                "line-wrap disable": b"\x1b[?7l",
                "framed content": "┌".encode("utf-8"),
                "three-part header with project version": b"\x1b[44m\x1b[97m\x1b[1m " + header + b"   |   TUI Test v7.8.9   |   Setup",
                "project type": b"go",
                "umlaut text": "Prüft Umlaute äöü".encode("utf-8"),
                "single-line setup step": b"[01/01]",
                "step output in content": b"sichtbare-schrittausgabe",
                "aligned setup output gutter": "│         │ sichtbare-schrittausgabe".encode("utf-8"),
                "green setup completion": b"\x1b[32m\x1b[1m" + "✓".encode("utf-8"),
                "alternate-screen exit": b"\x1b[?1049l",
            },
        )
        if "Prüfschritt abgeschlossen".encode("utf-8") in success:
            raise SystemExit("setup step still emits a second completion line")
        if b"INFO  [01/01]" in success:
            raise SystemExit("setup step still contains INFO prefix")
        if b"[1/1]" in success:
            raise SystemExit("setup counter is not zero padded")

        # --no-ui must win even when the environment explicitly requests the
        # fullscreen renderer. Output from the setup command must be streamed
        # directly and the alternate screen must never be entered.
        no_ui = run_pty(
            binary,
            ["--setup-manifest", str(success_manifest), "--no-ui", "--no-wait"],
            0,
        )
        require(
            no_ui,
            {
                "direct step": b"[01/01] Pr",
                "direct command output": b"sichtbare-schrittausgabe",
                "direct completion": b"[01/01]",
            },
        )
        if b"\x1b[?1049h" in no_ui or b"\x1b[?1049l" in no_ui:
            raise SystemExit("--no-ui unexpectedly entered the fullscreen alternate screen")
        if b"Task:" in no_ui:
            raise SystemExit("--no-ui still renders Task headings")

        # --noui is a supported alias for --no-ui and must have identical
        # alternate-screen suppression semantics.
        noui = run_pty(
            binary,
            ["--setup-manifest", str(success_manifest), "--noui", "--no-wait"],
            0,
        )
        require(
            noui,
            {
                "noui direct step": b"[01/01] Pr",
                "noui direct command output": b"sichtbare-schrittausgabe",
            },
        )
        if b"\x1b[?1049h" in noui or b"\x1b[?1049l" in noui:
            raise SystemExit("--noui unexpectedly entered the fullscreen alternate screen")
        if b"Task:" in noui:
            raise SystemExit("--noui still renders Task headings")

        # --setup must also work when invoked directly inside a deployed
        # current/ directory that contains setup.yaml but no project config.
        standalone_current = tmpdir / "standalone-current"
        standalone_current.mkdir()
        (standalone_current / "setup.yaml").write_text(
            "schemaVersion: 1\n"
            "project:\n"
            "  name: Current Folder Setup\n"
            "  type: go\n"
            "steps:\n"
            "  - id: marker\n"
            "    name: Current setup ausführen\n"
            "    run: printf standalone-ok > setup-result.txt\n",
            encoding="utf-8",
        )
        standalone = run_pty(
            binary, ["--setup", "--no-wait"], 0, cwd=standalone_current
        )
        require(
            standalone,
            {
                "standalone setup title": header,
                "standalone setup mode": b"Setup",
                "standalone manifest": b"setup.yaml",
                "standalone step": "Current setup ausführen".encode("utf-8"),
            },
        )
        if (standalone_current / "setup-result.txt").read_text(encoding="utf-8") != "standalone-ok":
            raise SystemExit("--setup in current/ did not execute the local setup.yaml")

        # The globally installed setup-template.sh must provide the same TUI by
        # delegating the current-directory manifest to the native CLI runner.
        template_current = tmpdir / "template-current"
        template_current.mkdir()
        (template_current / "setup.yaml").write_text(
            "schemaVersion: 1\n"
            "project:\n"
            "  name: Global Template TUI\n"
            "steps:\n"
            "  - id: marker\n"
            "    name: Template-Schritt\n"
            "    run: printf template-ok > template-result.txt\n",
            encoding="utf-8",
        )
        template_output = run_pty(
            root / "setup-template.sh",
            ["--no-wait"],
            0,
            cwd=template_current,
            env_overrides={"UPDATE_CLI_BIN": str(binary)},
        )
        require(
            template_output,
            {
                "template fullscreen": b"\x1b[?1049h",
                "template setup title": header,
                "template setup mode": b"Setup",
                "template step": b"Template-Schritt",
                "template fullscreen exit": b"\x1b[?1049l",
            },
        )
        if (template_current / "template-result.txt").read_text(encoding="utf-8") != "template-ok":
            raise SystemExit("global setup-template.sh did not execute current/setup.yaml")

        # Legacy setup.sh must never own a nested wait/fullscreen session.
        # The parent Update CLI TUI must remain visible and show the legacy
        # script as one explicit step.
        legacy_root = tmpdir / "legacy-project"
        legacy_downloads = tmpdir / "legacy-downloads"
        legacy_downloads.mkdir()
        write_config(legacy_root, legacy_downloads, "legacy-demo")
        legacy_current = legacy_root / "current"
        legacy_current.mkdir(parents=True)
        legacy_script = legacy_current / "setup.sh"
        legacy_script.write_text(
            "#!/usr/bin/env bash\n"
            "set -eu\n"
            "[ \"${SETUP_WAIT:-}\" = \"0\" ] || exit 41\n"
            "[ \"${SETUP_TUI_MODE:-}\" = \"plain\" ] || exit 42\n"
            "printf legacy-ok > legacy-result.txt\n",
            encoding="utf-8",
        )
        legacy_script.chmod(0o755)
        legacy_output = run_pty(
            binary, ["--setup", "--root", str(legacy_root), "--no-wait"], 0
        )
        require(
            legacy_output,
            {
                "legacy info region": "Legacy-Projekt-Setup".encode("utf-8"),
                "visible legacy step": b"[01/01] Legacy setup.sh ausf",
                "legacy success icon": b"\x1b[32m\x1b[1m" + "✓".encode("utf-8"),
            },
        )
        if b"INFO  [01/01]" in legacy_output:
            raise SystemExit("legacy setup step still contains INFO prefix")
        if (legacy_current / "legacy-result.txt").read_text(encoding="utf-8") != "legacy-ok":
            raise SystemExit("legacy setup.sh did not execute in parent-owned no-wait mode")

        failure_manifest = tmpdir / "failure.yaml"
        failure_manifest.write_text(
            "schemaVersion: 1\n"
            "project:\n"
            "  name: Failure Test\n"
            "steps:\n"
            "  - id: fail\n"
            "    name: Absichtlicher Fehler\n"
            "    run: echo visible-stdout; echo visible-stderr >&2; exit 7\n",
            encoding="utf-8",
        )
        failure = run_pty(
            binary, ["--setup-manifest", str(failure_manifest), "--no-wait"], 1
        )
        require(
            failure,
            {
                "failed command": "Fehlgeschlagener Befehl:".encode("utf-8"),
                "captured stdout": b"visible-stdout",
                "captured stderr": b"visible-stderr",
                "failure footer": b"FAIL Vorgang fehlgeschlagen",
                "alternate-screen exit after failure": b"\x1b[?1049l",
            },
        )

        # Setup-after-update defaults to YES. Pressing Enter without moving the
        # selection must execute setup and still commit the update.
        prompt_root = tmpdir / "prompt-project"
        prompt_downloads = tmpdir / "prompt-downloads"
        prompt_downloads.mkdir()
        write_config(prompt_root, prompt_downloads, "prompt-demo")
        prompt_archive = release_zip(
            prompt_downloads,
            "prompt-demo",
            "1.0.0",
            {
                "app.txt": "ok\n",
                "setup.yaml": (
                    "schemaVersion: 1\n"
                    "project:\n"
                    "  name: Prompt Demo\n"
                    "  type: go\n"
                    "steps:\n"
                    "  - id: setup\n"
                    "    run: printf ran > setup-ran.txt\n"
                ),
            },
        )
        prompt_output = run_pty(
            binary,
            ["--update", str(prompt_archive), "--root", str(prompt_root), "--no-wait"],
            0,
            input_data=b"\r",
        )
        require(
            prompt_output,
            {
                "setup confirmation modal question": "Projekt-Setup ist verfügbar. Jetzt ausführen?".encode(
                    "utf-8"
                ),
                "setup confirmation YES button": b"YES",
                "setup confirmation NO button": b"NO",
                "setup modal cursor hint": "←/→ = auswählen".encode("utf-8"),
                "aligned progress row": "[01/13] [".encode("utf-8"),
                "transaction phase": b"Transaktions-Snapshot von current erstellen",
                "green completed-step icon": b"\x1b[32m\x1b[1m" + "✓".encode("utf-8"),
                "final installed version": (
                    f"Update CLI Version {version} | prompt-demo | Aktualisiert auf Version: v1.0.0"
                ).encode("utf-8"),
            },
        )
        if b"INFO  Transaktions-Snapshot von current erstellen" in prompt_output:
            raise SystemExit("transaction snapshot emitted a duplicate INFO row")
        if b"OK    Transaktions-Snapshot erstellt" in prompt_output:
            raise SystemExit("transaction snapshot emitted a duplicate OK row")
        if b"INFO  [01/13]" in prompt_output:
            raise SystemExit("update phase still contains INFO prefix")
        if b"INFO  [" in prompt_output:
            raise SystemExit("an update/setup step still contains INFO prefix")
        if (
            "Projekt-Setup ist verfügbar. Jetzt ausführen? [j/N]".encode("utf-8") in prompt_output
            or "Projekt-Setup ist verfügbar. Jetzt ausführen? [J/n]".encode("utf-8") in prompt_output
        ):
            raise SystemExit("fullscreen setup confirmation still uses an inline footer prompt")
        if not (prompt_root / "current" / "setup-ran.txt").exists():
            raise SystemExit("default-YES setup modal did not run project setup on Enter")

        # Re-selecting the currently installed release is a successful no-op.
        # It must not paint the version-policy step or footer as FAIL. Instead
        # the content shows a green success banner and the normal blue close
        # footer remains available until Enter.
        same_root = tmpdir / "same-version-project"
        same_downloads = tmpdir / "same-version-downloads"
        same_downloads.mkdir()
        write_config(same_root, same_downloads, "same-demo")
        same_archive = release_zip(
            same_downloads, "same-demo", "1.0.3", {"app.txt": "same\n"}
        )
        run_pty(
            binary,
            [
                "--update",
                str(same_archive),
                "--root",
                str(same_root),
                "--no-ui",
                "--no-setup",
                "--no-wait",
            ],
            0,
        )
        same_output = run_pty(
            binary,
            ["--update", str(same_archive), "--root", str(same_root), "--no-setup"],
            0,
            input_data=b"\r",
        )
        require(
            same_output,
            {
                "same version notice": "Version 1.0.3 ist bereits installiert".encode("utf-8"),
                "green same-version content": (
                    b"\x1b[42m\x1b[97m\x1b[1m Version 1.0.3 ist bereits installiert"
                ),
                "normal close footer": "Update beenden | Enter zum Schließen".encode("utf-8"),
                "same-version final status": (
                    f"Update CLI Version {version} | same-demo | Installierte Version: v1.0.3"
                ).encode("utf-8"),
            },
        )
        if b"FAIL" in same_output or "Zur erneuten Installation".encode("utf-8") in same_output:
            raise SystemExit("same-version update still renders as a failure")

        # When the setup question is accepted, the update phase history must be
        # cleared from the scrollable content region before setup starts. Header,
        # project information and footer remain available. Inspect the final
        # fullscreen repaint: pre-setup phases 1-8 must no longer be present.
        clear_root = tmpdir / "clear-before-setup-project"
        clear_downloads = tmpdir / "clear-before-setup-downloads"
        clear_downloads.mkdir()
        write_config(clear_root, clear_downloads, "clear-demo")
        clear_archive = release_zip(
            clear_downloads,
            "clear-demo",
            "1.0.0",
            {
                "app.txt": "ok\n",
                "setup.yaml": (
                    "schemaVersion: 1\n"
                    "project:\n"
                    "  name: Clear Demo\n"
                    "steps:\n"
                    "  - id: setup\n"
                    "    name: Setup nach Clear\n"
                    "    run: printf 'setup-after-clear\\n'\n"
                ),
            },
        )
        clear_output = run_pty(
            binary,
            ["--update", str(clear_archive), "--root", str(clear_root), "--no-wait"],
            0,
            input_data=b"\x1b[D\r",
        )
        final_frame = clear_output.rsplit(b"\x1b[H", 1)[-1].split(b"\x1b[?1049l", 1)[0]
        require(
            final_frame,
            {
                "setup row after clear": "Setup nach Clear".encode("utf-8"),
                "setup stdout after clear": b"setup-after-clear",
                "post-setup transaction phase": b"Status schreiben und Transaktion abschlie",
            },
        )
        for stale in (
            b"Release-Quelle aufl",
            b"Zielversion und Update-Regeln pr",
            b"Transaktions-Snapshot von current erstellen",
            b"Release nach current synchronisieren",
        ):
            if stale in final_frame:
                raise SystemExit(
                    f"pre-setup update content was not cleared before setup: {stale!r}"
                )

        # check -> update -> setup must keep one consistent screen model:
        # setup output belongs to the scrollable content area while the footer
        # remains the high-level update state and is never replaced by a step.
        chained_root = tmpdir / "chained-project"
        chained_downloads = tmpdir / "chained-downloads"
        chained_downloads.mkdir()
        write_config(
            chained_root,
            chained_downloads,
            "chained-demo",
            ["check", "setup"],
        )
        release_zip(
            chained_downloads,
            "chained-demo",
            "1.0.0",
            {
                "app.txt": "ok\n",
                "setup.yaml": (
                    "schemaVersion: 1\n"
                    "project:\n"
                    "  name: Chained Demo\n"
                    "steps:\n"
                    "  - id: output\n"
                    "    name: Setup-Ausgabe prüfen\n"
                    "    run: printf 'nested-setup-output\\n'\n"
                ),
            },
        )
        chained_output = run_pty(
            binary,
            ["--check", "--root", str(chained_root), "--no-wait"],
            0,
            input_data=b"\r",
        )
        require(
            chained_output,
            {
                "check screen": "Versionsprüfung".encode("utf-8"),
                "update confirmation modal": "Update jetzt installieren?".encode("utf-8"),
                "update modal YES button": b"YES",
                "update modal NO button": b"NO",
                "update screen": header + b"   |   chained-demo   |   Update",
                "nested setup heading": b"Projekt-Setup",
                "nested setup step": "Setup-Ausgabe prüfen".encode("utf-8"),
                "nested setup stdout": b"nested-setup-output",
                "stable update footer": b"RUN  Update l\xc3\xa4uft",
                "chained final installed version": (
                    f"Update CLI Version {version} | chained-demo | Aktualisiert auf Version: v1.0.0"
                ).encode("utf-8"),
            },
        )
        for forbidden in (
            b"RUN  01/01 Setup-Ausgabe",
            b"OK   Setup-Ausgabe",
            b"OK   Projekt-Setup abgeschlossen",
            b"SKIP Projekt-Setup",
        ):
            if forbidden in chained_output:
                raise SystemExit(
                    f"nested setup overwrote the update footer: {forbidden!r}"
                )
        if (chained_root / "current" / "app.txt").read_text(encoding="utf-8") != "ok\n":
            raise SystemExit("check -> update -> setup did not install the release")

        # A setup failure during an update must show the exact transaction
        # phase and cause after restoring the previous current tree.
        error_root = tmpdir / "error-project"
        error_downloads = tmpdir / "error-downloads"
        error_downloads.mkdir()
        write_config(error_root, error_downloads, "error-demo")
        v1 = release_zip(error_downloads, "error-demo", "1.0.0", {"app.txt": "old\n"})
        run_pty(
            binary,
            ["--update", str(v1), "--root", str(error_root), "--no-setup", "--no-wait"],
            0,
        )
        v2 = release_zip(
            error_downloads,
            "error-demo",
            "1.1.0",
            {
                "app.txt": "new\n",
                "setup.yaml": (
                    "schemaVersion: 1\n"
                    "project:\n"
                    "  name: Error Demo\n"
                    "  type: go\n"
                    "steps:\n"
                    "  - id: fail\n"
                    "    name: Setup muss fehlschlagen\n"
                    "    run: echo setup-broke >&2; exit 17\n"
                ),
            },
        )
        update_failure = run_pty(
            binary,
            ["--update", str(v2), "--root", str(error_root), "--setup", "--no-wait"],
            1,
        )
        require(
            update_failure,
            {
                "setup error output": b"setup-broke",
                "detailed phase": "Phase: Projekt-Setup ausführen (setup)".encode("utf-8"),
                "target version": b"Zielversion: 1.1.0",
                "concrete cause": "Ursache: setup step".encode("utf-8"),
                "recovery": "Vorheriger current-Zustand wiederhergestellt".encode("utf-8"),
            },
        )
        if (error_root / "current" / "app.txt").read_text(encoding="utf-8") != "old\n":
            raise SystemExit("failed update did not restore previous current tree")

    return 0


if __name__ == "__main__":
    raise SystemExit(main())
