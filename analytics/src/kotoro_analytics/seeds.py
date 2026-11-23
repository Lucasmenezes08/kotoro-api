"""Deterministic development seed data for the analytics read model.

Run (database must be migrated first):

    python -m kotoro_analytics.seeds                 # end date 2026-11-20
    python -m kotoro_analytics.seeds --end-date 2026-12-01

The fixed IDs below are documented in analytics/README.md. Running the script again
replaces only the rows of the seed users; other data is never touched.
Aggregates are derived from the cycle facts, so all tables stay consistent.
"""

import argparse
import asyncio
import sys
import uuid
from collections import defaultdict
from dataclasses import dataclass, field
from datetime import date, datetime, time, timedelta
from typing import Any
from zoneinfo import ZoneInfo

from sqlalchemy import delete, insert
from sqlalchemy.ext.asyncio import AsyncEngine, create_async_engine
from sqlalchemy.pool import NullPool

from kotoro_analytics.config import Settings
from kotoro_analytics.domain.metrics import compute_streak, metric_date_for
from kotoro_analytics.infrastructure.database.models import (
    ProcessedEvent,
    SprintSummary,
    StudyCycleFact,
    UserDailyMetric,
    UserStreak,
    UserSubjectDailyMetric,
)

DEFAULT_END_DATE = date(2026, 11, 20)

# Fixed, documented identifiers. Never reuse them for real data.
USER_ANA = uuid.UUID("11111111-1111-4111-8111-111111111111")
USER_BRUNO = uuid.UUID("22222222-2222-4222-8222-222222222222")
USER_CARLA = uuid.UUID("33333333-3333-4333-8333-333333333333")  # no activity at all
SEED_USERS = (USER_ANA, USER_BRUNO, USER_CARLA)

SUBJECT_ANA_MATH = uuid.UUID("aaaaaaaa-0000-4000-8000-000000000001")
SUBJECT_ANA_PORTUGUESE = uuid.UUID("aaaaaaaa-0000-4000-8000-000000000002")
SUBJECT_ANA_BIOLOGY = uuid.UUID("aaaaaaaa-0000-4000-8000-000000000003")
SUBJECT_BRUNO_PHYSICS = uuid.UUID("bbbbbbbb-0000-4000-8000-000000000001")

# Namespace for derived IDs (sprints, blocks, cycles, events): uuid5(namespace, name).
SEED_NAMESPACE = uuid.UUID("5eed0000-0000-4000-8000-000000000000")

EVENT_TYPE = "study_cycle.finished"
EVENT_VERSION = 1


def _seed_id(*parts: object) -> uuid.UUID:
    return uuid.uuid5(SEED_NAMESPACE, ":".join(str(part) for part in parts))


@dataclass(frozen=True)
class _CycleSpec:
    user_id: uuid.UUID
    day: date
    sequence_number: int
    subject_id: uuid.UUID
    local_start: time
    elapsed_seconds: int
    stop_reason: str
    completed_tasks: int
    execution_mode: str = "bloco_materia"
    timer_type: str = "temporizado"


@dataclass
class SeedDataset:
    processed_events: list[dict[str, Any]] = field(default_factory=list)
    facts: list[dict[str, Any]] = field(default_factory=list)
    daily: list[dict[str, Any]] = field(default_factory=list)
    subject_daily: list[dict[str, Any]] = field(default_factory=list)
    sprints: list[dict[str, Any]] = field(default_factory=list)
    streaks: list[dict[str, Any]] = field(default_factory=list)


def _cycle_specs(end_date: date) -> list[_CycleSpec]:
    specs: list[_CycleSpec] = []

    # Ana: active from end-16 to end with one gap at end-7.
    # -> longest streak 9 days, current streak 7 days (as of end_date).
    ana_subjects = (SUBJECT_ANA_MATH, SUBJECT_ANA_PORTUGUESE, SUBJECT_ANA_BIOLOGY)
    ana_days = [end_date - timedelta(days=offset) for offset in range(16, -1, -1) if offset != 7]
    for index, day in enumerate(ana_days):
        specs.append(
            _CycleSpec(USER_ANA, day, 1, ana_subjects[index % 3], time(9, 0), 1500, "concluido", 1)
        )
        specs.append(
            _CycleSpec(
                USER_ANA, day, 2, ana_subjects[(index + 1) % 3], time(14, 0), 3000,
                "tempo_esgotado", 0, execution_mode="tarefa_individual",
            )
        )  # fmt: skip

    # Late-night cycle on Ana's last day: 22:00-22:50 in Sao Paulo is 01:00-01:50 UTC
    # of the next day, so it must still count for end_date.
    specs.append(
        _CycleSpec(
            USER_ANA, end_date, 3, SUBJECT_ANA_MATH, time(22, 0), 3000, "concluido", 2,
            execution_mode="multiplas_tarefas", timer_type="cronometro_livre",
        )
    )  # fmt: skip

    # Bruno: overlaps Ana's dates on purpose, to prove isolation between users.
    for offset in (2, 1, 0):
        day = end_date - timedelta(days=offset)
        specs.append(
            _CycleSpec(USER_BRUNO, day, 1, SUBJECT_BRUNO_PHYSICS, time(19, 0), 2700, "concluido", 2)
        )

    # Carla: intentionally no cycles (empty dashboard).
    return specs


def build_seed_dataset(end_date: date, timezone: ZoneInfo) -> SeedDataset:
    dataset = SeedDataset()

    daily: dict[tuple[uuid.UUID, date], dict[str, Any]] = {}
    subject_daily: dict[tuple[uuid.UUID, date, uuid.UUID], dict[str, Any]] = {}
    sprints: dict[uuid.UUID, dict[str, Any]] = {}
    active_days: dict[uuid.UUID, set[date]] = defaultdict(set)

    for spec in _cycle_specs(end_date):
        started_at = datetime.combine(spec.day, spec.local_start, tzinfo=timezone)
        ended_at = started_at + timedelta(seconds=spec.elapsed_seconds)
        metric_date = metric_date_for(ended_at, timezone)

        sprint_id = _seed_id("sprint", spec.user_id, spec.day)
        cycle_id = _seed_id("cycle", spec.user_id, spec.day, spec.sequence_number)
        event_id = _seed_id("event", cycle_id)

        dataset.processed_events.append(
            {
                "event_id": event_id,
                "event_type": EVENT_TYPE,
                "event_version": EVENT_VERSION,
                "occurred_at": ended_at,
            }
        )
        dataset.facts.append(
            {
                "cycle_id": cycle_id,
                "event_id": event_id,
                "user_id": spec.user_id,
                "sprint_id": sprint_id,
                "sprint_block_id": _seed_id("block", sprint_id, spec.subject_id),
                "subject_id": spec.subject_id,
                "sequence_number": spec.sequence_number,
                "execution_mode": spec.execution_mode,
                "timer_type": spec.timer_type,
                "stop_reason": spec.stop_reason,
                "started_at": started_at,
                "ended_at": ended_at,
                "elapsed_seconds": spec.elapsed_seconds,
                "completed_task_count": spec.completed_tasks,
                "metric_date": metric_date,
            }
        )

        for bucket in (
            daily.setdefault(
                (spec.user_id, metric_date),
                {"user_id": spec.user_id, "metric_date": metric_date},
            ),
            subject_daily.setdefault(
                (spec.user_id, metric_date, spec.subject_id),
                {
                    "user_id": spec.user_id,
                    "metric_date": metric_date,
                    "subject_id": spec.subject_id,
                },
            ),
        ):
            bucket["studied_seconds"] = bucket.get("studied_seconds", 0) + spec.elapsed_seconds
            bucket["completed_cycles"] = bucket.get("completed_cycles", 0) + 1
            bucket["completed_tasks"] = bucket.get("completed_tasks", 0) + spec.completed_tasks

        sprint = sprints.setdefault(
            sprint_id,
            {
                "sprint_id": sprint_id,
                "user_id": spec.user_id,
                "studied_seconds": 0,
                "completed_cycles": 0,
                "completed_tasks": 0,
                "first_cycle_started_at": started_at,
                "last_cycle_ended_at": ended_at,
            },
        )
        sprint["studied_seconds"] += spec.elapsed_seconds
        sprint["completed_cycles"] += 1
        sprint["completed_tasks"] += spec.completed_tasks
        sprint["first_cycle_started_at"] = min(sprint["first_cycle_started_at"], started_at)
        sprint["last_cycle_ended_at"] = max(sprint["last_cycle_ended_at"], ended_at)

        active_days[spec.user_id].add(metric_date)

    dataset.daily = list(daily.values())
    dataset.subject_daily = list(subject_daily.values())
    dataset.sprints = list(sprints.values())

    for user_id in SEED_USERS:
        streak = compute_streak(active_days.get(user_id, set()))
        dataset.streaks.append(
            {
                "user_id": user_id,
                "current_streak_days": streak.current_days,
                "longest_streak_days": streak.longest_days,
                "last_active_date": streak.last_active_date,
            }
        )

    return dataset


async def apply_seed(engine: AsyncEngine, dataset: SeedDataset) -> None:
    """Replace the seed users' rows in one transaction."""
    event_ids = [row["event_id"] for row in dataset.processed_events]

    async with engine.begin() as connection:
        for model in (
            StudyCycleFact,
            UserDailyMetric,
            UserSubjectDailyMetric,
            SprintSummary,
            UserStreak,
        ):
            await connection.execute(delete(model).where(model.user_id.in_(SEED_USERS)))
        await connection.execute(
            delete(ProcessedEvent).where(ProcessedEvent.event_id.in_(event_ids))
        )

        for model, rows in (
            (ProcessedEvent, dataset.processed_events),
            (StudyCycleFact, dataset.facts),
            (UserDailyMetric, dataset.daily),
            (UserSubjectDailyMetric, dataset.subject_daily),
            (SprintSummary, dataset.sprints),
            (UserStreak, dataset.streaks),
        ):
            if rows:
                await connection.execute(insert(model), rows)


async def _main(end_date: date) -> None:
    settings = Settings()
    if settings.environment == "production":
        sys.exit("refusing to seed: ANALYTICS_ENVIRONMENT is 'production'")

    dataset = build_seed_dataset(end_date, ZoneInfo(settings.metrics_timezone))
    # Own engine without the API's read-only session settings.
    engine = create_async_engine(settings.database_url, poolclass=NullPool)
    try:
        await apply_seed(engine, dataset)
    finally:
        await engine.dispose()

    print(
        f"seeded {len(dataset.facts)} cycles, {len(dataset.daily)} daily rows, "
        f"{len(dataset.sprints)} sprints for {len(SEED_USERS)} users (end date {end_date})"
    )


def main() -> None:
    parser = argparse.ArgumentParser(description="Seed the analytics read database.")
    parser.add_argument(
        "--end-date",
        type=date.fromisoformat,
        default=DEFAULT_END_DATE,
        help=f"last day with activity (default {DEFAULT_END_DATE})",
    )
    args = parser.parse_args()
    asyncio.run(_main(args.end_date))


if __name__ == "__main__":
    main()
