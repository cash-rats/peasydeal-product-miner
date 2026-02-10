import gzip
import tempfile
import unittest
from pathlib import Path

from scripts.cdp_snapshot_html import maybe_truncate, pick_target, write_html


class PickTargetTests(unittest.TestCase):
    def setUp(self):
        self.targets = [
            {
                "id": "t0",
                "type": "page",
                "url": "about:blank",
                "webSocketDebuggerUrl": "ws://127.0.0.1:9222/devtools/page/t0",
            },
            {
                "id": "t1",
                "type": "page",
                "url": "https://shopee.tw/product/1/2",
                "webSocketDebuggerUrl": "ws://127.0.0.1:9222/devtools/page/t1",
            },
        ]

    def test_pick_target_by_id(self):
        got = pick_target(self.targets, target_id="t1", url_contains="")
        self.assertEqual(got["id"], "t1")

    def test_pick_target_by_url_contains(self):
        got = pick_target(self.targets, target_id="", url_contains="shopee.tw")
        self.assertEqual(got["id"], "t1")

    def test_pick_target_prefers_non_blank(self):
        got = pick_target(self.targets, target_id="", url_contains="")
        self.assertEqual(got["id"], "t1")


class TruncateTests(unittest.TestCase):
    def test_no_truncate(self):
        out, truncated, original = maybe_truncate("abc", 10)
        self.assertEqual(out, "abc")
        self.assertFalse(truncated)
        self.assertEqual(original, 3)

    def test_truncate(self):
        out, truncated, original = maybe_truncate("abcdef", 3)
        self.assertEqual(out, "abc")
        self.assertTrue(truncated)
        self.assertEqual(original, 6)


class WriteHTMLTests(unittest.TestCase):
    def test_write_plain(self):
        with tempfile.TemporaryDirectory() as d:
            path = Path(d) / "a.html"
            n, digest = write_html(str(path), "<html>x</html>", force_gzip=False)
            self.assertTrue(n > 0)
            self.assertEqual(len(digest), 64)
            self.assertEqual(path.read_text(encoding="utf-8"), "<html>x</html>")

    def test_write_gzip(self):
        with tempfile.TemporaryDirectory() as d:
            path = Path(d) / "a.html.gz"
            n, digest = write_html(str(path), "<html>y</html>", force_gzip=False)
            self.assertTrue(n > 0)
            self.assertEqual(len(digest), 64)
            with gzip.open(path, "rb") as f:
                self.assertEqual(f.read().decode("utf-8"), "<html>y</html>")


if __name__ == "__main__":
    unittest.main()
