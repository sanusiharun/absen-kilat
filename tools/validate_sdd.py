"""Structural checks only; no approval or runtime claims."""
from pathlib import Path
import sys
root = Path(__file__).resolve().parents[1]
required = ["AGENTS.md", "CLAUDE.md", "docs/reengineering/INDEX.md", "docs/reengineering/engineering/CONSTITUTION.md", "docs/reengineering/engineering/WORKFLOW.md", "docs/reengineering/features/INDEX.md", "openspec/config.yaml"]
for change in (root / "openspec/changes").iterdir():
    if change.is_dir() and change.name != "archive":
        required += [str(change.relative_to(root) / f) for f in ("proposal.md", "design.md", "tasks.md", "verification.md")]
        if not list(change.glob("specs/*/spec.md")):
            required.append(str(change.relative_to(root) / "specs/<capability>/spec.md"))
missing = [p for p in required if not (root / p).is_file() or not (root / p).stat().st_size]
print("STRUCTURE FAIL: " + ", ".join(missing) if missing else "STRUCTURE PASS (not approval or runtime verification)")
sys.exit(bool(missing))
