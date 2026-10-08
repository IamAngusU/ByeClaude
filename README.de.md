<p align="center">
  <picture>
    <source media="(prefers-color-scheme: dark)" srcset="assets/brand/byeclaude-logo-dark.webp">
    <img src="assets/brand/byeclaude-logo.webp" width="220" alt="ByeClaude Logo">
  </picture>
</p>

<p align="center">
  <a href="README.md"><img src="docs/assets/readme-language-en.svg" height="40" alt="Read this README in English"></a>
</p>

<h1 align="center">Unerwünschte AI-Co-Author-Credits in der Git-Historie prüfen und sicher bereinigen.</h1>
<p align="center">Erst scannen. Jeden Rewrite vorher prüfen. Dateiinhalte unverändert lassen.</p>

<p align="center">
  <a href="#schnellstart"><img src="docs/assets/readme/badge-default.svg" height="40" alt="Zuerst nur lesen"></a>
  <a href="#sicherheit-zuerst"><img src="docs/assets/readme/badge-safety.svg" height="40" alt="Backups und geschützte Pushes"></a>
  <a href="#schnellstart"><img src="docs/assets/readme/badge-platforms.svg" height="40" alt="Windows, Linux und macOS"></a>
  <a href="LICENSE"><img src="docs/assets/readme/badge-license.svg" height="40" alt="MIT-Lizenz"></a>
</p>

<p align="center">
  <a href="https://github.com/angusu-de/ByeClaude/actions/workflows/ci.yml"><img src="https://raw.githubusercontent.com/angusu-de/ByeClaude/ci-proof/proof/public-proof.svg" height="54" alt="Live CI-Status von ByeClaude"></a>
</p>

<p align="center"><sub>Öffentliche CI über einen separaten Account desselben Maintainers. <a href="https://github.com/angusu-de/ByeClaude/blob/ci-proof/proof/README.md">Getesteter Commit und einzelne Schritte</a>. Badge-Design: IamAngusU/Badges.</sub></p>

Einige AI-Tools hängen Zeilen wie `Co-authored-by: Claude <noreply@anthropic.com>` an Git-Commit-Messages. ByeClaude gibt dir die Kontrolle über diese deklarierte Git-Metadaten: bestehende Historie prüfen, ausgewählte Credits entfernen und verhindern, dass sie erneut auftauchen. Claude/Anthropic ist standardmäßig ausgewählt; eine gespeicherte Blacklist kann mehrere Tool-Identitäten abdecken.

Trotz des Namens ist ByeClaude kein Anti-AI-Projekt und kein AI-Detector. Es versucht nicht zu erraten, wer Code geschrieben hat. Es arbeitet ausschließlich mit deklarierter Git-Metadaten.

Ich nutze Claude Code selbst nicht in meinem täglichen Workflow. Die Claude-Attribution wird deshalb in Wegwerf-Test-Repositories nachgestellt statt aus meiner eigenen Historie übernommen. Rewrite- und Sicherheitsverhalten werden automatisiert und über öffentliche CI getestet; Feedback aus echten Claude-Code-Repositories ist deshalb besonders willkommen.

## Schnellstart

**Installieren → geführtes Setup → Git normal weiterbenutzen.** Du brauchst [Git](https://git-scm.com/downloads), aber keinen Go-Compiler, keine Administratorrechte und für lokale Arbeit keinen GitHub-Login.

<details open>
<summary><strong>Windows · in PowerShell einfügen</strong></summary>

```powershell
irm https://raw.githubusercontent.com/IamAngusU/ByeClaude/v0.1.0-alpha.6/install.ps1 | iex
```

</details>

<details>
<summary><strong>macOS / Linux · in Bash, Zsh oder sh einfügen</strong></summary>

```sh
(f="$(mktemp)" && trap 'rm -f -- "$f"' EXIT && curl -fsSL https://raw.githubusercontent.com/IamAngusU/ByeClaude/v0.1.0-alpha.6/install.sh -o "$f" && sh "$f")
```

</details>

Das Installationsskript erkennt die Plattform, lädt das passende Release, prüft SHA-256, installiert in deinem Benutzerordner, richtet den Benutzer-PATH ein und öffnet das geführte Setup in einem interaktiven Terminal. Git selbst bleibt Voraussetzung. [Windows-Skript prüfen](install.ps1) · [Unix-Skript prüfen](install.sh) · [Manueller Download und Supply-Chain-Details](docs/supply-chain.md).

### Repository einrichten

1. **Identitäten auswählen.** Claude behalten oder exakte E-Mail-Adressen weiterer Tools hinzufügen.
2. **Historie prüfen.** Gefundene Credits, Autoren und Committer ansehen. Das ist read-only.
3. **Cleanup vorschauen.** Auswirkungen und Backup-Plan prüfen. Erst die Eingabe von `CLEAN` wendet den Rewrite lokal an.
4. **Künftige Commits schützen.** Optional beide lokalen Git-Hooks installieren.

Der Installer bereinigt keine Historie und installiert keine Hooks ohne separate Bestätigung.

### Benutzung

Nach dem Setup laufen normale `git commit`- und `git push`-Vorgänge über die installierten Hooks. Der Commit-Hook entfernt passende Co-Author-Credits; der Push-Hook prüft die Historie und kann einen Push blockieren, bis alte Treffer geprüft wurden. Die Hooks gelten nur für diesen Clone.

```text
byeclaude
```

| Ziel | Befehl |
| --- | --- |
| Geführtes Setup erneut öffnen | `byeclaude guide` |
| Nur prüfen, nichts ändern | `byeclaude scan` |
| Lokale Aktivität live ansehen | `byeclaude metrics --watch` |

![Aufgezeichneter geführter Terminal-Ablauf](docs/assets/readme/terminal-flow.gif)

## Was ändert sich?

Standardmäßig entfernt ByeClaude nur passende `Co-authored-by`-Zeilen. Menschliche Credits, die nicht zur Blacklist passen, bleiben erhalten. **Die Inhalte der committed Dateien bleiben identisch.** Eine Änderung von Author/Committer ist eine separate, erweiterte Option.

<p align="center"><img src="docs/assets/readme/attribution-before-after.svg" width="1040" alt="Ein passender Claude-Co-Author-Eintrag wird entfernt; Dateiinhalte bleiben gleich, Commit-IDs ändern sich."></p>

Git-Commit-IDs hängen vom vollständigen Commit-Objekt ab. Wird ein Commit geändert, ändern sich deshalb auch seine ID und die IDs betroffener Nachfolger. ByeClaude zeigt das vor dem Anwenden an und legt lokale Backup-Refs an. [Technische Details](docs/how-it-works.md).

## Veröffentlichen, wenn du bereit bist

**Das Menü pusht nichts automatisch zu GitHub.** Nach einem geprüften Cleanup:

```sh
byeclaude push
byeclaude verify
```

`push` verwendet einen atomaren Force-with-Lease-Workflow und verweigert die Veröffentlichung, wenn sich lokale oder entfernte Refs unerwartet geändert haben. `verify` prüft anschließend frisch geladene GitHub-Historie und veröffentlichte PR-Refs in den tatsächlich erreichbaren Bereichen.

Ein lokaler Rewrite lässt sich über `byeclaude restore --backup ID --apply` zurücksetzen. Restore überschreibt keine späteren lokalen Änderungen. [Sicherheit und Recovery](docs/safety.md).

## Lokale Metriken

`byeclaude metrics` zeigt unter anderem entfernte Credits, rewritete Commits, automatische Commit-Message-Edits und blockierte Push-Versuche. Die Zähler bleiben lokal; Repository-Namen, E-Mail-Adressen und Commit-Inhalte werden nicht hochgeladen.

```sh
byeclaude metrics
byeclaude metrics --watch
byeclaude metrics off
```

![Lokale Live-Metriken](docs/assets/readme/terminal-metrics.gif)

## Sicherheit zuerst

- Audits und Previews sind read-only. Cleanup und Hook-Installation brauchen eigene Bestätigungen.
- Ändert sich Repository oder Policy während der Prüfung, wird der anstehende Vorgang abgebrochen.
- Backup-Refs halten den vorherigen lokalen Zustand erreichbar.
- Der eingebaute Push schützt neuere Remote-Arbeit mit atomarem Force-with-Lease.
- Hooks gelten nur für diesen Clone; Web/API-Commits, andere Clones oder absichtlich umgangene Hooks brauchen eigene Absicherung.
- GitHub-Caches, historische PR-Objekte, Forks und andere Clones können alte Commits weiterhin enthalten. Kein Tool kann aus diesem Workflow vollständige Löschung versprechen.

ByeClaude prüft **deklarierte Git-Metadaten**. Es erkennt keinen AI-generierten Code und beweist nicht, wer eine Datei geschrieben hat. Es ist weiterhin Prerelease-Software.

## Weitere Optionen

Die TUI ist optional. Für Scripts und CI stehen unter anderem `scan`, `check`, `plan`, `blacklist`, `setup` und `verify` zur Verfügung. Batch-Audits bleiben read-only.

Die ausführliche technische Dokumentation ist aktuell Englisch:

[CLI-Referenz](docs/cli.md) · [Regeln und Blacklist](docs/rules.md) · [Hooks und CI](docs/automation.md) · [Batch-Scanning](docs/batch.md) · [GitHub-Verifikation](docs/github-verification.md) · [Identity Correction](docs/identity-correction.md) · [Gesamte Doku](docs/README.md)

[Contributing](CONTRIBUTING.md) · [Security Policy](SECURITY.md) · [MIT-Lizenz](LICENSE)

<p align="center"><sub>Powered by <a href="https://angusu.de">angusu.de</a> · Angus Uelsmann. Unabhängiges Open-Source-Projekt. Nicht mit Anthropic verbunden. <a href="TRADEMARKS.md">Trademark-Hinweise</a>.</sub></p>
