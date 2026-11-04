"""Compare two contract versions and classify changes as breaking or compatible."""
import argparse
import sys

from .contract import load_contract


def diff(old: dict, new: dict) -> list:
    out = []

    def add(level, msg):
        out.append((level, msg))

    os_, ns = old["signals"], new["signals"]
    for n in sorted(os_.keys() - ns.keys()):
        add("breaking", f"signal removed: {n}")
    for n in sorted(ns.keys() - os_.keys()):
        add("compatible", f"signal added: {n}")
    for n in sorted(os_.keys() & ns.keys()):
        o, w = os_[n], ns[n]
        if o["type"] != w["type"]:
            add("breaking", f"{n}: type {o['type']} -> {w['type']}")
        if o["unit"] != w["unit"]:
            add("breaking", f"{n}: unit {o['unit']} -> {w['unit']}")
        (olo, ohi), (nlo, nhi) = o["range"], w["range"]
        if nlo > olo or nhi < ohi:
            add("breaking", f"{n}: range narrowed [{olo}, {ohi}] -> [{nlo}, {nhi}]")
        elif (nlo, nhi) != (olo, ohi):
            add("compatible", f"{n}: range widened [{olo}, {ohi}] -> [{nlo}, {nhi}]")
        for t in sorted(set(o["applies_to"]) - set(w["applies_to"])):
            add("breaking", f"{n}: no longer applies to {t}")
        for t in sorted(set(w["applies_to"]) - set(o["applies_to"])):
            add("compatible", f"{n}: now applies to {t}")

    of, nf = set(old.get("fault_codes", [])), set(new.get("fault_codes", []))
    for c in sorted(of - nf):
        add("breaking", f"fault code removed: {c}")
    for c in sorted(nf - of):
        add("compatible", f"fault code added: {c}")

    if out and new.get("version", 0) <= old.get("version", 0):
        add("breaking", f"contract changed but version not bumped ({old.get('version')} -> {new.get('version')})")
    return out


def main(argv=None) -> int:
    ap = argparse.ArgumentParser(description=__doc__)
    ap.add_argument("old")
    ap.add_argument("new")
    ap.add_argument("--allow-breaking", action="store_true")
    args = ap.parse_args(argv)

    changes = diff(load_contract(args.old), load_contract(args.new))
    for level, msg in changes:
        print(f"{level.upper():10} {msg}")
    if not changes:
        print("no changes")
    breaking = any(level == "breaking" for level, _ in changes)
    return 1 if breaking and not args.allow_breaking else 0


if __name__ == "__main__":
    sys.exit(main())
