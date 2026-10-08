"""Exercise the action wrapper without network access or a Go toolchain."""
import hashlib
import os
from pathlib import Path
import shutil
import subprocess
import tempfile
import unittest


ROOT = Path(__file__).resolve().parents[1]
BASH = shutil.which("bash")


@unittest.skipUnless(BASH, "bash is required")
class ActionTests(unittest.TestCase):
    def setUp(self):
        self.tmp = tempfile.TemporaryDirectory(prefix="byeclaude-action-test-")
        self.addCleanup(self.tmp.cleanup)
        self.root = Path(self.tmp.name)
        self.action = self.root / "action with spaces"
        (self.action / "scripts").mkdir(parents=True)
        (self.action / "action-release").mkdir()
        shutil.copyfile(ROOT / "scripts/run-action.sh", self.action / "scripts/run-action.sh")
        (self.action / "action-release/version").write_text("v0.0.0-test\n", newline="\n")
        self.bin = self.root / "tools"
        self.bin.mkdir()
        self.fixture = self.root / "binary"
        self.write(self.fixture, '''#!/usr/bin/env bash
if [ "$1" = version ]; then printf 'byeclaude %s\n' "${FAKE_VERSION:-v0.0.0-test}"; exit 0; fi
printf '%s\n' "$@" > "$ARGUMENTS"
printf '%s\n' "$BYECLAUDE_METRICS" > "$METRICS_MODE"
exit "${CHECK_EXIT:-0}"
''')
        self.write(self.bin / "curl", '''#!/usr/bin/env bash
printf '%s\n' "$@" > "$DOWNLOAD_ARGS"
while [ "$#" -gt 0 ]; do
  if [ "$1" = -o ]; then cp "$FIXTURE" "$2"; exit "${DOWNLOAD_EXIT:-0}"; fi
  shift
done
exit 1
''')
        self.write(self.bin / "git", '''#!/usr/bin/env bash
if [ "${@: -1}" = --is-shallow-repository ]; then printf '%s\n' "${SHALLOW:-false}"; fi
''')
        self.write(self.bin / "go", "#!/usr/bin/env bash\necho 'GO MUST NOT RUN' >&2\nexit 99\n")
        self.write(self.bin / "cygpath", '#!/usr/bin/env bash\nprintf "%s\\n" "${@: -1}"\n')
        self.runner_tmp = self.root / "runner temp"
        self.runner_tmp.mkdir()
        inherited = {k: v for k, v in os.environ.items() if k.upper() != "PATH"}
        self.env = dict(inherited, PATH=str(self.bin) + os.pathsep + os.environ["PATH"],
                        RUNNER_OS="Linux", RUNNER_ARCH="X64", RUNNER_TEMP=self.runner_tmp.as_posix(),
                        GITHUB_WORKSPACE=(self.root / "workspace with spaces").as_posix(),
                        FIXTURE=self.fixture.as_posix(), ARGUMENTS=(self.root / "args").as_posix(),
                        METRICS_MODE=(self.root / "metrics").as_posix(), DOWNLOAD_ARGS=(self.root / "download").as_posix())
        # Git Bash prepends its own tools at startup. Re-prepend the test shims
        # inside each noninteractive shell so tests cannot hit real git/curl.
        tools_path = self.bin.as_posix()
        if os.name == "nt":
            tools_path = "/" + tools_path[0].lower() + tools_path[2:]
        startup = self.root / "bash-env"
        startup.write_text('export PATH="$TEST_TOOLS:$PATH"\n', newline="\n")
        self.env.update(BASH_ENV=startup.as_posix(), TEST_TOOLS=tools_path)
        for key in ("BYECLAUDE_INCLUDE_REMOTES", "BYECLAUDE_INCLUDE_IDENTITIES", "BYECLAUDE_RULES_FILE"):
            self.env.pop(key, None)
        self.pin()

    def write(self, path, text):
        path.write_text(text, newline="\n")
        path.chmod(0o755)

    def pin(self, asset="byeclaude_linux_amd64", digest=None):
        digest = digest or hashlib.sha256(self.fixture.read_bytes()).hexdigest()
        (self.action / "action-release/SHA256SUMS.txt").write_text(f"{digest}  {asset}\n", newline="\n")

    def run_action(self, **changes):
        result = subprocess.run([BASH, str(self.action / "scripts/run-action.sh")],
                                env=dict(self.env, **changes), capture_output=True, text=True, timeout=15)
        self.assertFalse(list(self.runner_tmp.iterdir()), "temporary binary survived")
        self.assertNotIn("GO MUST NOT RUN", result.stderr)
        return result

    def test_verified_binary_and_literal_arguments_without_go(self):
        rules = 'rules $(touch PWNED); space.json'
        result = self.run_action(BYECLAUDE_RULES_FILE=rules, BYECLAUDE_INCLUDE_IDENTITIES="true")
        self.assertEqual(result.returncode, 0, result.stderr)
        args = (self.root / "args").read_text().splitlines()
        self.assertEqual(args, ["check", "--include-remotes", "--include-identities", "--rules",
                               self.env["GITHUB_WORKSPACE"] + "/" + rules, "--repo", self.env["GITHUB_WORKSPACE"]])
        self.assertEqual((self.root / "metrics").read_text().strip(), "off")
        self.assertIn("Older attribution can block", result.stdout)
        self.assertIn("--proto-redir\n=https", (self.root / "download").read_text())
        self.assertFalse((ROOT / "PWNED").exists())

    def test_matching_history_propagates_failure(self):
        result = self.run_action(CHECK_EXIT="7", BYECLAUDE_INCLUDE_REMOTES="false")
        self.assertEqual(result.returncode, 7, result.stderr)
        self.assertNotIn("--include-remotes", (self.root / "args").read_text())

    def test_bad_digest_and_download_never_execute(self):
        self.pin(digest="0" * 64)
        result = self.run_action()
        self.assertNotEqual(result.returncode, 0)
        self.assertIn("checksum mismatch", result.stderr)
        self.assertFalse((self.root / "args").exists())
        self.pin()
        result = self.run_action(DOWNLOAD_EXIT="22")
        self.assertNotEqual(result.returncode, 0)
        self.assertFalse((self.root / "args").exists())

    def test_shallow_and_invalid_inputs_fail_before_download(self):
        for changes in ({"SHALLOW": "true"}, {"RUNNER_ARCH": "X86"}, {"RUNNER_OS": "unknown"},
                        {"BYECLAUDE_INCLUDE_REMOTES": "yes"}, {"BYECLAUDE_INCLUDE_IDENTITIES": "yes"},
                        {"BYECLAUDE_RULES_FILE": "../secret"}, {"BYECLAUDE_RULES_FILE": "..\\secret"},
                        {"BYECLAUDE_RULES_FILE": "/secret"}, {"BYECLAUDE_RULES_FILE": "C:\\secret"}):
            with self.subTest(changes=changes):
                result = self.run_action(**changes)
                self.assertNotEqual(result.returncode, 0)
                self.assertFalse((self.root / "download").exists())

    def test_all_six_platform_assets_are_selected_before_verification(self):
        for runner, target in (("Linux", "linux"), ("macOS", "darwin"), ("Windows", "windows")):
            for arch, binary_arch in (("X64", "amd64"), ("ARM64", "arm64")):
                asset = f"byeclaude_{target}_{binary_arch}" + (".exe" if runner == "Windows" else "")
                self.pin(asset, "0" * 64)
                result = self.run_action(RUNNER_OS=runner, RUNNER_ARCH=arch)
                self.assertIn("checksum mismatch", result.stderr)
                self.assertIn("/v0.0.0-test/" + asset, (self.root / "download").read_text())

    def test_duplicate_checksum_and_version_mismatch_fail_closed(self):
        path = self.action / "action-release/SHA256SUMS.txt"
        path.write_text(path.read_text() * 2, newline="\n")
        self.assertIn("duplicate", self.run_action().stderr)
        self.pin()
        result = self.run_action(FAKE_VERSION="v9.9.9")
        self.assertIn("version mismatch", result.stderr)
        self.assertFalse((self.root / "args").exists())


if __name__ == "__main__":
    unittest.main()
