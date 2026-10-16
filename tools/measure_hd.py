#!/usr/bin/env python3
"""在 tools/go.sh 的容器內量測；不接受主機執行。

PSGOLEM_ORIG=~/cht/castlevinia_x PSGOLEM_GO_CMD=python3 tools/go.sh tools/measure_hd.py
預先以 tools/go.sh build 建立 workplace/hd/psexe 與 psexe-before。
結果寫入 workplace/hd/performance.json，原版輸入只從 /orig 讀取。
"""
import json
import os
from pathlib import Path
import resource
import subprocess
import time

if not Path('/.dockerenv').exists() or Path.cwd() != Path('/src'):
    raise SystemExit('請透過 tools/go.sh 在容器內執行')

out = Path('workplace/hd')
runs = []
for round_no in range(3):
    for mode in ('before', 'native', 'hd2'):
        binary = out / ('psexe-before' if mode == 'before' else 'psexe')
        args = [str(binary), '-bios', '/orig/bios/SCPH5500.BIN', '-cue',
                '/orig/disc/sotn.cue', '-fields', '4400', '-cycles',
                '400000000000', '-digest']
        if mode == 'hd2':
            args += ['-hd', '2']
        usage = resource.getrusage(resource.RUSAGE_CHILDREN)
        start = time.monotonic()
        load = os.getloadavg()
        result = subprocess.run(args, capture_output=True, text=True, errors='replace',
                                timeout=600, check=True)
        after = resource.getrusage(resource.RUSAGE_CHILDREN)
        record = dict(round=round_no+1, mode=mode, wall_seconds=time.monotonic()-start,
                      cpu_seconds=after.ru_utime+after.ru_stime-usage.ru_utime-usage.ru_stime,
                      loadavg=load, digest=next(l for l in result.stdout.splitlines() if l.startswith('digest ')),
                      command=args)
        runs.append(record)
        print(json.dumps(record, ensure_ascii=False), flush=True)

best = {mode: {key: min(r[key] for r in runs if r['mode'] == mode)
               for key in ('wall_seconds', 'cpu_seconds')}
        for mode in ('before', 'native', 'hd2')}
summary = dict(runs=runs, best=best, cpu_count=os.cpu_count(),
               cpu_model=next(l for l in Path('/proc/cpuinfo').read_text().splitlines() if l.startswith('model name')),
               cgroup_cpu=Path('/sys/fs/cgroup/cpu.max').read_text().strip(),
               cgroup_memory=Path('/sys/fs/cgroup/memory.max').read_text().strip())
if len({r['digest'] for r in runs}) != 1:
    raise SystemExit('量測結果的原生狀態摘要不同')
(out/'performance.json').write_text(json.dumps(summary,ensure_ascii=False,indent=2)+'\n')
print(json.dumps(best,indent=2),flush=True)
