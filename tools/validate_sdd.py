"""Structural validation and deterministic review digest; NOT approval authentication."""
import argparse
import hashlib
from pathlib import Path

ROOT = Path(__file__).resolve().parents[1]
REQUIRED = ("AGENTS.md", "CLAUDE.md", "docs/reengineering/INDEX.md", "docs/reengineering/features/INDEX.md", "openspec/config.yaml")
ENGINEERING = ("CONSTITUTION", "WORKFLOW", "AGENT_PROTOCOL", "VERIFICATION", "HERMES_PROTOCOL", "AGENT_CAPABILITY_MATRIX", "AUTONOMY_POLICY", "TASK_DELEGATION", "RECOVERY_PROTOCOL", "COST_MANAGEMENT", "HERMES_ONBOARDING", "APPROVAL_PROTOCOL", "KNOWN_BASELINE_ISSUES")
SKILLS = ("spec-driven", "frontend-design", "backend-development", "mobile-development", "code-review", "verification", "orchestration", "feature-discovery", "documentation-audit")

def validate(root):
    required = list(REQUIRED) + [f"docs/reengineering/engineering/{n}.md" for n in ENGINEERING] + [f".agents/skills/{n}/SKILL.md" for n in SKILLS]
    changes = root / "openspec/changes"
    errors = []
    if not changes.is_dir():
        errors.append("missing openspec/changes")
    else:
        for change in sorted(changes.iterdir()):
            if not change.is_dir() or change.name == "archive":
                continue
            required.extend(str(change.relative_to(root) / n) for n in ("proposal.md", "design.md", "tasks.md", "verification.md"))
            specs = list(change.glob("specs/*/spec.md"))
            if not specs:
                errors.append(f"{change.name}: missing spec")
            required.extend(str(p.relative_to(root)) for p in specs)
    for name in required:
        p = root / name
        if not p.is_file() or not p.read_bytes().strip():
            errors.append(f"missing or empty: {name}")
    return errors

def digest(change, gate):
    if gate not in ("G1", "G2"):
        raise ValueError("invalid gate")
    paths = [change / "proposal.md"]
    if gate == "G2":
        specs = sorted(change.glob("specs/*/spec.md"))
        if not specs:
            raise ValueError("missing specs")
        paths += [change / "design.md", change / "tasks.md"] + specs
    h = hashlib.sha256()
    for p in paths:
        data = p.read_bytes()
        if not data.strip():
            raise ValueError("empty artifact")
        name = p.relative_to(change).as_posix().encode()
        for part in (name, data):
            h.update(len(part).to_bytes(8, "big"))
            h.update(part)
    return h.hexdigest()

def main():
    parser = argparse.ArgumentParser()
    parser.add_argument("--digest", nargs=2, metavar=("CHANGE", "GATE"))
    args = parser.parse_args()
    errors = validate(ROOT)
    if errors:
        print("\n".join(errors))
        return 1
    if args.digest:
        name, gate = args.digest
        if not name or name in (".", "..", "archive") or Path(name).name != name:
            parser.error("invalid change ID")
        print(digest(ROOT / "openspec/changes" / name, gate))
    else:
        print("STRUCTURE PASS (not approval or runtime verification)")
    return 0

if __name__ == "__main__":
    raise SystemExit(main())
