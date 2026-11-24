"""End-to-end demo: a study cycle finished in the Go API shows up in the Analytics API.

Go API -> outbox_events -> Outbox Relay -> RabbitMQ -> Analytics Consumer
       -> PostgreSQL Analytics -> Analytics API

Run with the whole stack up (`docker compose up --build -d`):

    python analytics/scripts/demo_end_to_end.py

Standard library only. Creates a fresh demo user on every run.
"""

import json
import os
import sys
import time
import urllib.error
import urllib.request
import uuid
from datetime import datetime
from typing import Any
from zoneinfo import ZoneInfo

API = os.environ.get("KOTORO_API_URL", "http://localhost:8080")
ANALYTICS = os.environ.get("KOTORO_ANALYTICS_URL", "http://localhost:8001")
TIMEOUT_SECONDS = 30


def call(method: str, url: str, body: dict | None = None, token: str | None = None) -> Any:
    request = urllib.request.Request(url, method=method)
    request.add_header("Content-Type", "application/json")
    if token:
        request.add_header("Authorization", f"Bearer {token}")
    data = json.dumps(body).encode() if body is not None else None
    try:
        with urllib.request.urlopen(request, data=data, timeout=10) as response:
            raw = response.read()
    except urllib.error.HTTPError as error:
        sys.exit(f"{method} {url} -> {error.code}: {error.read().decode(errors='replace')}")
    return json.loads(raw) if raw else None


def created(list_url: str, name: str, token: str) -> dict:
    items = call("GET", list_url, token=token) or []
    matches = [item for item in items if item.get("name") == name]
    if not matches:
        sys.exit(f"could not find {name!r} in {list_url}")
    return matches[-1]


def step(text: str) -> None:
    print(f"\n== {text}")


def main() -> None:
    today = datetime.now(ZoneInfo("America/Sao_Paulo")).date().isoformat()
    period = f"from={today}&to={today}"

    step("1. Register a demo user in the Go API")
    email = f"demo-{uuid.uuid4().hex[:8]}@kotoro.local"
    auth = call(
        "POST",
        f"{API}/auth/register",
        {"name": "Demo Analytics", "email": email, "password": "SenhaForte123!"},
    )
    token, user_id = auth["token"], auth["user"]["id"]
    print(f"user_id={user_id}")

    step("2. Analytics before studying (expected: zeros)")
    before = call("GET", f"{ANALYTICS}/api/v1/analytics/summary?{period}", token=token)
    print(json.dumps(before))

    step("3. Plan and run a study cycle in the Go API")
    # The Go API answers 201 without a body for subjects and sprints: look them up by name.
    suffix = uuid.uuid4().hex[:6]
    subject_name, sprint_name = f"Sistemas Distribuidos {suffix}", f"Demo Analytics {suffix}"
    call("POST", f"{API}/subjects", {"name": subject_name, "color": "blue"}, token)
    subject = created(f"{API}/subjects", subject_name, token)
    call("POST", f"{API}/sprints", {"name": sprint_name, "sprint_date": today}, token)
    sprint = created(f"{API}/sprints", sprint_name, token)
    call(
        "POST",
        f"{API}/sprints/{sprint['id']}/blocks",
        {
            "subject_id": subject["id"],
            "title": "Mensageria",
            "position": 1,
            "estimated_seconds": 1800,
        },
        token,
    )
    call("PATCH", f"{API}/sprints/{sprint['id']}", {"status": "in_progress"}, token)
    cycle = call(
        "POST",
        f"{API}/sprints/{sprint['id']}/study-cycles/play",
        {
            "execution_mode": "bloco_materia",
            "timer_type": "cronometro_livre",
            "planned_duration_seconds": None,
            "block_task_ids": [],
        },
        token,
    )
    time.sleep(3)
    stopped = call("POST", f"{API}/study-cycles/{cycle['id']}/stop", {"reason": "concluido"}, token)
    finished_at = time.monotonic()
    print(f"cycle {cycle['id']} -> {stopped['status']}, {stopped['elapsed_seconds']} s")

    step("4. Wait for the event to reach the Analytics API (eventual consistency)")
    while True:
        summary = call("GET", f"{ANALYTICS}/api/v1/analytics/summary?{period}", token=token)
        if summary["completed_cycles"] >= 1:
            break
        if time.monotonic() - finished_at > TIMEOUT_SECONDS:
            sys.exit(
                "timeout: check `docker compose logs outbox-relay analytics-consumer` "
                "(the relay must be restarted after a RabbitMQ restart)"
            )
        time.sleep(0.25)
    print(f"visible after {time.monotonic() - finished_at:.2f} s")

    step("5. Analytics API now")
    print("summary :", json.dumps(summary))
    daily = call("GET", f"{ANALYTICS}/api/v1/analytics/daily?{period}", token=token)
    print("daily   :", json.dumps(daily["days"]))
    subjects = call("GET", f"{ANALYTICS}/api/v1/analytics/subjects?{period}", token=token)
    print("subjects:", json.dumps(subjects["subjects"]))
    sprint_summary = call(
        "GET", f"{ANALYTICS}/api/v1/analytics/sprints/{sprint['id']}", token=token
    )
    print("sprint  :", json.dumps(sprint_summary))

    assert summary["studied_seconds"] == stopped["elapsed_seconds"], "seconds must match the Go API"
    assert subjects["subjects"][0]["subject_id"] == subject["id"], "subject must match"
    print("\nOK: the cycle finished in the Go API is reflected in every analytics endpoint.")


if __name__ == "__main__":
    main()
