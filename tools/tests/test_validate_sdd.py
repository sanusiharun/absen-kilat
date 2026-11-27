import tempfile
import unittest
from pathlib import Path
from tools.validate_sdd import validate, digest, ROOT

class ValidatorTests(unittest.TestCase):
    def test_repository_structure(self):
        self.assertEqual(validate(ROOT), [])
    def test_missing_structure(self):
        with tempfile.TemporaryDirectory() as tmp:
            self.assertTrue(validate(Path(tmp)))
    def test_digest_invalidates_on_edit(self):
        with tempfile.TemporaryDirectory() as tmp:
            p = Path(tmp)
            (p / "proposal.md").write_text("one")
            first = digest(p, "G1")
            (p / "proposal.md").write_text("two")
            self.assertNotEqual(first, digest(p, "G1"))
    def test_missing_specs(self):
        with tempfile.TemporaryDirectory() as tmp:
            with self.assertRaises(ValueError):
                digest(Path(tmp), "G2")
    def test_empty_proposal(self):
        with tempfile.TemporaryDirectory() as tmp:
            p = Path(tmp)
            (p / "proposal.md").write_text(" ")
            with self.assertRaises(ValueError):
                digest(p, "G1")
