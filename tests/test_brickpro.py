import importlib.util
import os
from pathlib import Path
import shutil
import subprocess
import tempfile
import unittest

ROOT = Path(__file__).resolve().parents[1]
spec = importlib.util.spec_from_file_location("adapter", ROOT / "System/bin/ex_portmaster.py")
adapter = importlib.util.module_from_spec(spec)
spec.loader.exec_module(adapter)
FRAGMENTS = {p.name: p.read_text() for p in (ROOT / "System/lib/trimui-ex").glob("*.inc")}


class AdapterTests(unittest.TestCase):
    def test_model_overrides_shared_resolution_without_affecting_other_devices(self):
        source = '''DEVICES = {}
HW_INFO = {}
def new_device_info():
    return {"name": "TrimUI", "device": "trimui-brick"}
'''
        patched = adapter.transform("pylibs/harbourmaster/hardware.py", source, FRAGMENTS)
        self.assertEqual(patched, adapter.transform("pylibs/harbourmaster/hardware.py", patched, FRAGMENTS))
        for model, expected in [("TG4040", "trimui-brick-pro"), ("TG3040", "trimui-brick"), ("", "trimui-brick")]:
            scope = {"safe_cat": lambda path: model}
            exec(patched, scope)
            self.assertEqual(scope["new_device_info"]()["device"], expected)
            self.assertEqual(scope["HW_INFO"]["trimui-brick-pro"]["analogsticks"], 2)

    def test_unknown_interface_is_rejected(self):
        with self.assertRaises(ValueError):
            adapter.transform("pylibs/harbourmaster/hardware.py", "def changed_api(): pass\n", FRAGMENTS)
        with self.assertRaises(ValueError):
            adapter.transform("device_info.txt", "#!/bin/bash\n", FRAGMENTS)

    def test_shell_patches_are_idempotent_and_parse_as_bash(self):
        for name, source in [("device_info.txt", "#!/bin/bash\n# GLIBC\ntrue\n"),
                             ("control.txt", "get_controls() { :; }\n# device_info.txt\n"),
                             ("../launch.sh", "#!/bin/sh\nsource config\na=(one two)\n")]:
            result = adapter.transform(name, source, FRAGMENTS)
            self.assertEqual(result, adapter.transform(name, result, FRAGMENTS))
            subprocess.run(["bash", "-n"], input=result, text=True, check=True)


class BashEntryTests(unittest.TestCase):
    def test_missing_shell_and_script_shebang_preserve_arguments_and_status(self):
        with tempfile.TemporaryDirectory() as directory:
            system = Path(directory)
            (system / "bin").mkdir()
            wrapper = system / "bin/bash"
            shutil.copy2(ROOT / "System/bin/bash", wrapper)
            (system / "bin/bash.real").symlink_to(shutil.which("bash"))
            game = system / "A game.sh"
            game.write_text(f'#!{wrapper}\na=(one two)\nprintf "%s|%s|%s" "$SHELL" "${{a[1]}}" "$1"\nexit 23\n')
            game.chmod(0o755)
            # No inherited login/SSH variables; reproduces the menu condition.
            result = subprocess.run([str(game), "argument with spaces"],
                                    env={"EX_SYSTEM_PATH": str(system)}, capture_output=True, text=True)
            self.assertEqual(result.returncode, 23, result.stderr)
            self.assertEqual(result.stdout, "/bin/sh|two|argument with spaces")

    def test_existing_shell_is_preserved(self):
        with tempfile.TemporaryDirectory() as directory:
            system = Path(directory)
            (system / "bin").mkdir()
            (system / "bin/bash.real").symlink_to(shutil.which("bash"))
            result = subprocess.run([str(ROOT / "System/bin/bash"), "-c", 'printf "%s" "$SHELL"'],
                                    env={"EX_SYSTEM_PATH": str(system), "SHELL": "/bin/custom-shell"},
                                    capture_output=True, text=True, check=True)
            self.assertEqual(result.stdout, "/bin/custom-shell")


class LauncherTests(unittest.TestCase):
    def setUp(self):
        self.tmp = tempfile.TemporaryDirectory()
        self.addCleanup(self.tmp.cleanup)
        self.base = Path(self.tmp.name)
        self.system = self.base / "System"
        (self.system / "etc").mkdir(parents=True)
        (self.system / "bin").mkdir()
        shutil.copy(ROOT / "System/etc/ex_config", self.system / "etc/ex_config")
        (self.system / "bin/bash").symlink_to(shutil.which("bash"))
        helper = self.system / "bin/ex_portmaster.sh"
        helper.write_text("#!/bin/sh\nexit 0\n")
        helper.chmod(0o755)
        self.cpu = self.base / "cpufreq"
        self.cpu.mkdir()
        for name, content in {
            "scaling_min_freq": "408000", "scaling_max_freq": "2000000",
            "scaling_governor": "schedutil",
            "scaling_available_frequencies": "408000 600000 1200000 1608000 1800000 2000000",
            "scaling_available_governors": "schedutil ondemand performance conservative",
        }.items():
            (self.cpu / name).write_text(content + "\n")
        (self.base / "model").write_text("TG4040\n")
        (self.base / "release").write_text("DISTRIB_DESCRIPTION='tina.user.20260828.020940 4.0.0'\n")
        self.env = dict(os.environ, EX_SYSTEM_PATH=str(self.system), EX_MODEL_FILE=str(self.base / "model"),
                        EX_RELEASE_FILE=str(self.base / "release"), EX_CPUFREQ_PATH=str(self.cpu), EX_PORTS_PATH=str(self.base))

    def state(self):
        return [(self.cpu / key).read_text() for key in ("scaling_min_freq", "scaling_max_freq", "scaling_governor")]

    def test_bash_arrays_arguments_exit_status_and_cpu_restore(self):
        game = self.base / "A game with spaces.sh"
        content = '#!/bin/bash\na=(one two)\nprintf "%s|%s|%s" "${a[1]}" "$1" "$2"\nexit 7\n'
        game.write_text(content)
        before = self.state()
        result = subprocess.run([str(ROOT / "Emus/PORTS/launch.sh"), str(game), "arg one", "$(literal)"],
                                env=self.env, text=True, capture_output=True)
        self.assertEqual(result.returncode, 7, result.stderr)
        self.assertEqual(result.stdout, "two|arg one|$(literal)")
        self.assertEqual(game.read_text(), content)
        self.assertEqual(self.state(), before)

    def test_invalid_profile_makes_no_writes(self):
        before = self.state()
        result = subprocess.run([str(ROOT / "Emus/PORTS/cpufreq.sh"), "not a profile"], env=self.env, capture_output=True)
        self.assertNotEqual(result.returncode, 0)
        self.assertEqual(self.state(), before)

    def test_unsupported_frequency_clamped_and_governor_preserved(self):
        (self.cpu / "scaling_available_frequencies").write_text("408000 600000 1200000\n")
        (self.cpu / "scaling_available_governors").write_text("schedutil\n")
        subprocess.run([str(ROOT / "Emus/PORTS/cpufreq.sh"), "High Performance"], env=self.env, check=True)
        self.assertEqual(self.state(), ["600000\n", "1200000\n", "schedutil\n"])

    def test_unknown_firmware_does_not_change_mac(self):
        (self.base / "release").write_text("unrecognized release format\n")
        result = subprocess.run(["sh", "-c", '. "$EX_SYSTEM_PATH/etc/ex_config"; . "$EX_SYSTEM_PATH/etc/ex_config"; printf "%s|%s" "$NETWORK_FIX" "$LD_LIBRARY_PATH"'],
                                env=dict(self.env, LD_LIBRARY_PATH=""), text=True, capture_output=True, check=True)
        self.assertEqual(result.stdout, f"N|/usr/trimui/lib:{self.system}/lib")

    def test_framebuffer_terminal_is_available_to_helpers(self):
        result = subprocess.run(["sh", "-c", '. "$EX_SYSTEM_PATH/etc/ex_config"; printenv TERM'],
                                env=dict(self.env, TERM="dumb"), text=True, capture_output=True, check=True)
        self.assertEqual(result.stdout, "linux\n")


if __name__ == "__main__":
    unittest.main()
