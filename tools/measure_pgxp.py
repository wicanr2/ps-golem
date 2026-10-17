#!/usr/bin/env python3
"""經 tools/go.sh 在 Docker 內交錯量測 PGXP，CPU 時間包含影子執行緒。

先建置 workplace/pgxp/psexe-before 與 psexe，再執行：
PSGOLEM_CPUSET=0-3 PSGOLEM_ORIG=~/cht/castlevinia_x \
PSGOLEM_GO_CMD=python3 tools/go.sh tools/measure_pgxp.py
"""
import hashlib
import json
import os
from pathlib import Path
import resource
import subprocess
import time

if not Path('/.dockerenv').exists() or Path.cwd() != Path('/src'):
    raise SystemExit('請透過 tools/go.sh 在容器內執行')
if not os.sched_getaffinity(0) <= {0, 1, 2, 3}:
    raise SystemExit('請設定 PSGOLEM_CPUSET=0-3 固定 P 核心')

out = Path('workplace/pgxp')
modes = {
    'before-native': ('psexe-before', []),
    'after-native': ('psexe', []),
    'before-hd2': ('psexe-before', ['-hd', '2']),
    'after-hd2': ('psexe', ['-hd', '2']),
    'pgxp-hd2': ('psexe', ['-hd', '2', '-pgxp']),
}
env = dict(os.environ, GOMAXPROCS='2')
runs = []
for round_no in range(3):
    order = list(modes)
    if round_no % 2:
        order.reverse()
    for mode in order:
        binary, flags = modes[mode]
        args = [str(out/binary), '-bios', '/orig/bios/SCPH5500.BIN', '-cue',
                '/orig/disc/sotn.cue', '-fields', '4400', '-cycles',
                '400000000000', '-digest'] + flags
        before = resource.getrusage(resource.RUSAGE_CHILDREN)
        start = time.monotonic()
        load = os.getloadavg()
        result = subprocess.run(args, capture_output=True, text=True,
                                errors='replace', timeout=1200, check=True, env=env)
        after = resource.getrusage(resource.RUSAGE_CHILDREN)
        record = dict(round=round_no+1, mode=mode, wall_seconds=time.monotonic()-start,
                      cpu_seconds=after.ru_utime+after.ru_stime-before.ru_utime-before.ru_stime,
                      loadavg=load, command=args,
                      digest=next(l for l in result.stdout.splitlines() if l.startswith('digest ')))
        runs.append(record)
        print(json.dumps(record, ensure_ascii=False), flush=True)

best = {mode: min(r['cpu_seconds'] for r in runs if r['mode'] == mode) for mode in modes}
noise = {mode: max(r['cpu_seconds'] for r in runs if r['mode'] == mode)/best[mode]-1
         for mode in modes}
off = {}
for path in ('native', 'hd2'):
    change = best['after-'+path]/best['before-'+path]-1
    band = max(noise['before-'+path], noise['after-'+path])
    off[path] = dict(change=change, noise_range=band, passed=abs(change) <= band)
slowdown = 1-best['after-hd2']/best['pgxp-hd2']
summary = dict(runs=runs, minimum_cpu_seconds=best, noise_range=noise, pgxp_off=off,
               pgxp_speed_decrease=slowdown, pgxp_passed=slowdown <= .25,
               affinity=sorted(os.sched_getaffinity(0)), GOMAXPROCS=2,
               cpu_model=next(l for l in Path('/proc/cpuinfo').read_text().splitlines()
                              if l.startswith('model name')),
               binary_sha256={name: hashlib.sha256((out/name).read_bytes()).hexdigest()
                              for name in ('psexe-before', 'psexe')},
               baseline_commit='b68b4c7', fields=[0, 4400])
if len({r['digest'] for r in runs}) != 1:
    raise SystemExit('量測摘要不同')
(out/'performance.json').write_text(json.dumps(summary, ensure_ascii=False, indent=2)+'\n')
print(json.dumps({k: v for k, v in summary.items() if k != 'runs'}, ensure_ascii=False, indent=2))
if not all(v['passed'] for v in off.values()):
    raise SystemExit('停下：不開 PGXP 的 A/B 差異超出本次雜訊範圍')
if not summary['pgxp_passed']:
    raise SystemExit('PGXP 速度降幅超出 25%')
