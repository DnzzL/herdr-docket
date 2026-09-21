"""Seed the fleet the demo records: two queues and runs that started minutes ago.

The queues are copied out of the checked-in templates beside this file, so the
recording can press `p` — which writes `disabled:` into an AGENT.md — without
touching the template that is under version control.
"""

import datetime as dt
import json
import os
import shutil
import sys

demo = sys.argv[1]
here = os.path.dirname(os.path.abspath(__file__))
now = dt.datetime.now(dt.timezone.utc)

shutil.rmtree(demo, ignore_errors=True)
os.makedirs(os.path.join(demo, "state"))
os.makedirs(os.path.join(demo, "cfg"))

# Two queues behind one board, plus the three agents they route to. `ops` ships
# disabled, which is the mark the board puts next to a row that routes to it.
for name in ("fleet", "ops"):
    shutil.copytree(os.path.join(here, name), os.path.join(demo, name))

# No default_agent on purpose: every task here is unassigned or already
# assigned, so a daemon that starts against this file claims nothing.
with open(os.path.join(demo, "cfg", "fleet.yaml"), "w") as f:
    f.write(
        f"""# Written by scripts/demo/seed.py — the fleet the board demo records.
dir: {demo}/fleet
sources:
  myapp:
    kind: backlogmd
    dir: {demo}/fleet
  ops:
    kind: backlogmd
    dir: {demo}/ops
"""
    )


def rec(run_id, task, agent, status, workspace, **ago):
    return json.dumps(
        {
            "run_id": run_id,
            "task": task,
            "agent": agent,
            "trigger": "poll",
            "status": status,
            "at": (now - dt.timedelta(**ago)).strftime("%Y-%m-%dT%H:%M:%S.000000Z"),
            # The workspace the run happened in, and the pane inside it: the id
            # `x` closes. A running record without one is not a shape the
            # daemon writes, and a fixture that had one would answer "no run to
            # stop" on a row that reads Running.
            "workspace_id": workspace,
            "pane_id": workspace + ":p1",
        }
    )


# A fresh run and a stale one: `example` has no timeout_minutes, so its run
# counts against the fleet's default of 60 and has been past it for hours.
lines = [
    rec("myapp/TASK-1-1", "myapp/TASK-1", "dev", "running", "wR:p7", minutes=12),
    rec("myapp/TASK-2-2", "myapp/TASK-2", "example", "running", "wR:p8", hours=4, minutes=30),
    rec("ops/TASK-2-3", "ops/TASK-2", "ops", "done", "wR:p9", hours=3),
    rec("myapp/TASK-4-4", "myapp/TASK-4", "dev", "done", "wR:pA", days=1),
]
with open(os.path.join(demo, "state", "history.jsonl"), "w") as f:
    f.write("\n".join(lines) + "\n")
