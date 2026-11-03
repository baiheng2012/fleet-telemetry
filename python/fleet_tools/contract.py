from pathlib import Path

import yaml

REPO_ROOT = Path(__file__).resolve().parents[2]
DEFAULT_CONTRACT = REPO_ROOT / "contracts" / "vehicle_semantics.yaml"


def load_contract(path=DEFAULT_CONTRACT) -> dict:
    with open(path, encoding="utf-8") as f:
        c = yaml.safe_load(f)
    for name, s in c["signals"].items():
        lo, hi = s["range"]
        if lo >= hi:
            raise ValueError(f"signal {name}: range min must be < max")
    return c
