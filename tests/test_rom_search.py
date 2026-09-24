"""Tests for ROM installation without contacting game sites."""
import importlib.util
import io
from pathlib import Path
import sys
import tempfile
import unittest
from unittest.mock import patch


SOURCE = Path(__file__).resolve().parents[1] / "Apps/ROMSearch/rom_sources.py"
spec = importlib.util.spec_from_file_location("rom_sources", SOURCE)
rom_sources = importlib.util.module_from_spec(spec)
sys.modules[spec.name] = rom_sources
spec.loader.exec_module(rom_sources)


class Response(io.BytesIO):
    def __init__(self, url, data, content_type="application/zip"):
        super().__init__(data)
        self.url = url
        self.headers = {"Content-Type": content_type,
                        "Content-Length": str(len(data))}


class DownloadTests(unittest.TestCase):
    def test_both_sources_save_in_matching_existing_folders(self):
        for source, system, folder, url, resolver, payload, filename in (
            ("CoolROM", "genesis", "MD",
             "https://dl.coolrom.com/roms/genesis/test.md/token/123/",
             "coolrom_download_url", b"rom", "test.md"),
            ("RomsFun", "mame", "MAME",
             "https://sto1.romsforever.co/archive/test.zip?e=123&s=x",
             "romsfun_download_url", b"PK\x03\x04test", "test.zip"),
        ):
            with self.subTest(source=source), tempfile.TemporaryDirectory() as root:
                (Path(root) / folder).mkdir()
                game = rom_sources.Game(source, system, "Test", "https://example.test/page")
                with patch.object(rom_sources, resolver, return_value=url), \
                     patch.object(rom_sources.urllib.request, "urlopen",
                                  return_value=Response(url, payload)):
                    saved = rom_sources.download(game, root)
                self.assertEqual(saved, Path(root) / folder / filename)
                self.assertEqual(saved.read_bytes(), payload)
                self.assertFalse((saved.parent / (saved.name + ".partial")).exists())

    def test_html_response_never_replaces_an_existing_rom(self):
        with tempfile.TemporaryDirectory() as root:
            target = Path(root) / "MD"
            target.mkdir()
            game = rom_sources.Game("CoolROM", "genesis", "Test", "https://example.test/page")
            url = "https://dl.coolrom.com/roms/genesis/test.zip/token/123/"
            with patch.object(rom_sources, "coolrom_download_url", return_value=url), \
                 patch.object(rom_sources.urllib.request, "urlopen",
                              return_value=Response(url, b"blocked", "text/html")):
                with self.assertRaises(rom_sources.SourceError):
                    rom_sources.download(game, root)
            self.assertEqual(list(target.iterdir()), [])


if __name__ == "__main__":
    unittest.main()
