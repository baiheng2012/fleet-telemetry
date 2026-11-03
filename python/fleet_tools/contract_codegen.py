"""Contract -> dbt macros / dbt tests / data dictionary."""
import argparse
import sys

from .contract import DEFAULT_CONTRACT, REPO_ROOT, load_contract

PG_TYPES = {"float": "double precision", "int": "bigint", "string": "text"}
REF = "{{ ref('curated_telemetry') }}"


def gen_macros(c: dict) -> str:
    sigs = c["signals"]
    extract, clean, flags = [], [], []
    for n, s in sigs.items():
        lo, hi = s["range"]
        extract.append(f"(payload->>'{n}')::{PG_TYPES[s['type']]} as {n}")
        clean.append(f"case when {n} between {lo} and {hi} then {n} end as {n}")
        flags.append(
            f"case when {n} is not null and ({n} < {lo} or {n} > {hi}) "
            f"then '{n}_out_of_range' end::text"
        )
    codes = ", ".join(f"'{x}'" for x in c.get("fault_codes", []))
    if codes:
        flags.append(
            f"case when fault_code is not null and fault_code not in ({codes}) "
            f"then 'unknown_fault_code' end::text"
        )
    sep = ",\n    "
    return (
        "{# GENERATED from contracts/vehicle_semantics.yaml by fleet_tools.contract_codegen. DO NOT EDIT. #}\n\n"
        "{% macro contract_extract_signals() %}\n    " + sep.join(extract) + "\n{% endmacro %}\n\n"
        "{% macro contract_clean_signals() %}\n    " + sep.join(clean) + "\n{% endmacro %}\n\n"
        "{% macro contract_dq_flags() %}\n    array_remove(array[\n        "
        + ",\n        ".join(flags)
        + "\n    ], null)\n{% endmacro %}\n"
    )


def gen_range_test(c: dict) -> str:
    parts = []
    for n, s in c["signals"].items():
        lo, hi = s["range"]
        parts.append(
            f"select vehicle_id, ts, '{n}' as signal, {n}::double precision as value\n"
            f"from {REF}\n"
            f"where {n} is not null and ({n} < {lo} or {n} > {hi})"
        )
    return (
        "-- GENERATED from contracts/vehicle_semantics.yaml. DO NOT EDIT.\n"
        "-- dbt singular test: returns violating rows; passes when empty.\n"
        + "\nunion all\n".join(parts)
        + "\n"
    )


def gen_dictionary(c: dict) -> str:
    lines = [
        "<!-- GENERATED from contracts/vehicle_semantics.yaml. DO NOT EDIT. -->",
        "# Vehicle data dictionary",
        "",
        "| Signal | Type | Unit | Valid range | Applies to |",
        "|---|---|---|---|---|",
    ]
    for n, s in c["signals"].items():
        lo, hi = s["range"]
        lines.append(f"| `{n}` | {s['type']} | {s['unit']} | [{lo}, {hi}] | {', '.join(s['applies_to'])} |")
    lines += ["", "Valid fault codes: " + ", ".join(f"`{x}`" for x in c.get("fault_codes", [])), ""]
    return "\n".join(lines)


def outputs(c: dict) -> dict:
    return {
        REPO_ROOT / "warehouse/dbt/macros/contract_generated.sql": gen_macros(c),
        REPO_ROOT / "warehouse/dbt/tests/generated/assert_signals_in_range.sql": gen_range_test(c),
        REPO_ROOT / "docs/data_dictionary.md": gen_dictionary(c),
    }


def main(argv=None) -> int:
    ap = argparse.ArgumentParser(description=__doc__)
    ap.add_argument("--contract", default=str(DEFAULT_CONTRACT))
    ap.add_argument("--check", action="store_true", help="fail if generated files are out of date")
    args = ap.parse_args(argv)

    c = load_contract(args.contract)
    stale = []
    for path, text in outputs(c).items():
        rel = path.relative_to(REPO_ROOT)
        if args.check:
            if not path.exists() or path.read_text(encoding="utf-8") != text:
                stale.append(rel)
        else:
            path.parent.mkdir(parents=True, exist_ok=True)
            path.write_text(text, encoding="utf-8")
            print("wrote", rel)
    if stale:
        print("out of date, run: python -m fleet_tools.contract_codegen\n  " + "\n  ".join(map(str, stale)))
        return 1
    return 0


if __name__ == "__main__":
    sys.exit(main())
