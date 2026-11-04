def test_dashboard_panels_have_sql_and_unique_ids():
    from fleet_tools import dashboard_gen
    panels = dashboard_gen.build()["panels"]
    assert len({p["id"] for p in panels}) == len(panels)
    assert all(p["targets"][0]["rawSql"].strip() for p in panels)
