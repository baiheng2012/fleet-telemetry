package reconcile

import "strings"

// 行级摘要。$1 = 滞后截断秒数：忽略最近 N 秒的行，避免把"在途数据"当成差异。
const rowInner = `
select {id}::bigint as id, equip_code,
       to_char(recorded_at at time zone 'UTC', 'YYYY-MM-DD') as day,
       hyd_psi,
       md5(concat_ws('|',
           {id}::text, equip_code,
           to_char(recorded_at at time zone 'UTC', 'YYYY-MM-DD"T"HH24:MI:SS.US'),
           coalesce(rpm::text, '~'),
           coalesce(round(coolant_c, 2)::text, '~'),
           coalesce(round(hyd_psi, 2)::text, '~'),
           coalesce(round(fuel_pct, 2)::text, '~'),
           coalesce(round(hours, 3)::text, '~'),
           coalesce(fault, '~'))) as rh
from {table}
where recorded_at < now() - make_interval(secs => $1::double precision)`

func build(q, table, idCol string) string {
	return strings.NewReplacer("{table}", table, "{id}", idCol).Replace(q)
}

// PartSQL 返回 (equip_code, day, n, sum_psi, hash)。
func PartSQL(table, idCol string) string {
	return build(`select equip_code, day, count(*), coalesce(sum(hyd_psi), 0)::text,
       md5(string_agg(rh, '' order by id))
from (`+rowInner+`) t group by 1, 2 order by 1, 2`, table, idCol)
}

// RowSQL 返回某个分区内每行的 (id, hash)。参数：$1 截断秒数 $2 equip $3 day。
func RowSQL(table, idCol string) string {
	return build(`select id, rh from (`+rowInner+`) t where equip_code = $2 and day = $3 order by id`, table, idCol)
}
