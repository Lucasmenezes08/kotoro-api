from datetime import date
from zoneinfo import ZoneInfo

import pytest

from kotoro_analytics.seeds import (
    DEFAULT_END_DATE,
    USER_ANA,
    USER_BRUNO,
    USER_CARLA,
    SeedDataset,
    build_seed_dataset,
)


@pytest.fixture(scope="module")
def dataset() -> SeedDataset:
    return build_seed_dataset(DEFAULT_END_DATE, ZoneInfo("America/Sao_Paulo"))


def _rows_for(rows: list[dict], user_id: object) -> list[dict]:
    return [row for row in rows if row["user_id"] == user_id]


def test_build_is_deterministic(dataset: SeedDataset) -> None:
    again = build_seed_dataset(DEFAULT_END_DATE, ZoneInfo("America/Sao_Paulo"))

    assert again == dataset


def test_ana_totals(dataset: SeedDataset) -> None:
    daily = _rows_for(dataset.daily, USER_ANA)

    assert len(daily) == 16
    assert sum(row["studied_seconds"] for row in daily) == 75_000
    assert sum(row["completed_cycles"] for row in daily) == 33
    assert sum(row["completed_tasks"] for row in daily) == 18


def test_ana_streak(dataset: SeedDataset) -> None:
    [streak] = _rows_for(dataset.streaks, USER_ANA)

    assert streak["current_streak_days"] == 7
    assert streak["longest_streak_days"] == 9
    assert streak["last_active_date"] == DEFAULT_END_DATE


def test_late_night_cycle_counts_for_end_date(dataset: SeedDataset) -> None:
    [late] = [f for f in dataset.facts if f["user_id"] == USER_ANA and f["sequence_number"] == 3]

    assert late["ended_at"].astimezone(ZoneInfo("UTC")).date() == date(2026, 11, 21)
    assert late["metric_date"] == DEFAULT_END_DATE


def test_bruno_is_isolated_from_ana(dataset: SeedDataset) -> None:
    bruno_subjects = {row["subject_id"] for row in _rows_for(dataset.subject_daily, USER_BRUNO)}
    ana_subjects = {row["subject_id"] for row in _rows_for(dataset.subject_daily, USER_ANA)}

    assert sum(row["studied_seconds"] for row in _rows_for(dataset.daily, USER_BRUNO)) == 8_100
    assert bruno_subjects.isdisjoint(ana_subjects)
    # Same calendar days as Ana on purpose: isolation must come from user_id, not dates.
    bruno_days = {row["metric_date"] for row in _rows_for(dataset.daily, USER_BRUNO)}
    ana_days = {row["metric_date"] for row in _rows_for(dataset.daily, USER_ANA)}
    assert bruno_days <= ana_days


def test_carla_has_no_activity_but_has_empty_streak(dataset: SeedDataset) -> None:
    assert _rows_for(dataset.facts, USER_CARLA) == []
    [streak] = _rows_for(dataset.streaks, USER_CARLA)
    assert streak["current_streak_days"] == 0
    assert streak["last_active_date"] is None


def test_aggregates_match_facts(dataset: SeedDataset) -> None:
    facts_seconds = sum(f["elapsed_seconds"] for f in dataset.facts)

    assert sum(r["studied_seconds"] for r in dataset.daily) == facts_seconds
    assert sum(r["studied_seconds"] for r in dataset.subject_daily) == facts_seconds
    assert sum(r["studied_seconds"] for r in dataset.sprints) == facts_seconds
    assert len(dataset.processed_events) == len(dataset.facts)
