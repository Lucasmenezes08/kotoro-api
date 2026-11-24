"""HTTP response contracts. ORM models are never returned directly (onboarding §15)."""

import uuid
from datetime import UTC, date, datetime

from pydantic import BaseModel, ConfigDict, Field

from kotoro_analytics.application.queries.daily import DailySeries
from kotoro_analytics.application.queries.sprints import SprintSummary
from kotoro_analytics.application.queries.subjects import SubjectMetrics, SubjectsReport
from kotoro_analytics.application.queries.summary import Summary
from kotoro_analytics.domain.period import Period


class PeriodResponse(BaseModel):
    # `from` is a Python keyword: the attribute is `start`, the JSON key is "from".
    start: date = Field(serialization_alias="from", description="First day, inclusive.")
    to: date = Field(description="Last day, inclusive.")
    timezone: str = Field(description="IANA zone used to define the days.")

    @classmethod
    def of(cls, period: Period, timezone: str) -> "PeriodResponse":
        return cls(start=period.start, to=period.end, timezone=timezone)


class StreakResponse(BaseModel):
    current_days: int = Field(
        description="Consecutive active days ending today or yesterday; 0 otherwise. "
        "Not affected by the period."
    )
    longest_days: int = Field(description="Longest run ever. Not affected by the period.")
    last_active_date: date | None = Field(description="null when the user never studied.")


class SummaryResponse(BaseModel):
    model_config = ConfigDict(
        json_schema_extra={
            "example": {
                "period": {
                    "from": "2026-11-01",
                    "to": "2026-11-30",
                    "timezone": "America/Sao_Paulo",
                },
                "studied_seconds": 75000,
                "completed_cycles": 33,
                "completed_tasks": 18,
                "active_days": 16,
                "streak": {"current_days": 7, "longest_days": 9, "last_active_date": "2026-11-20"},
            }
        }
    )

    period: PeriodResponse
    studied_seconds: int = Field(description="Seconds studied in finished cycles in the period.")
    completed_cycles: int = Field(description="Finished cycles in the period.")
    completed_tasks: int = Field(description="Tasks completed by those cycles.")
    active_days: int = Field(description="Days in the period with at least one finished cycle.")
    streak: StreakResponse

    @classmethod
    def from_summary(cls, summary: Summary) -> "SummaryResponse":
        return cls(
            period=PeriodResponse.of(summary.period, summary.timezone),
            studied_seconds=summary.totals.studied_seconds,
            completed_cycles=summary.totals.completed_cycles,
            completed_tasks=summary.totals.completed_tasks,
            active_days=summary.totals.active_days,
            streak=StreakResponse(
                current_days=summary.current_streak_days,
                longest_days=summary.longest_streak_days,
                last_active_date=summary.last_active_date,
            ),
        )


class DayResponse(BaseModel):
    # Attribute `day` avoids shadowing the `date` type; the JSON key is "date".
    day: date = Field(serialization_alias="date", description="Calendar day.")
    studied_seconds: int
    completed_cycles: int
    completed_tasks: int


class DailyResponse(BaseModel):
    model_config = ConfigDict(
        json_schema_extra={
            "example": {
                "period": {
                    "from": "2026-11-12",
                    "to": "2026-11-14",
                    "timezone": "America/Sao_Paulo",
                },
                "days": [
                    {
                        "date": "2026-11-12",
                        "studied_seconds": 4500,
                        "completed_cycles": 2,
                        "completed_tasks": 1,
                    },
                    {
                        "date": "2026-11-13",
                        "studied_seconds": 0,
                        "completed_cycles": 0,
                        "completed_tasks": 0,
                    },
                    {
                        "date": "2026-11-14",
                        "studied_seconds": 4500,
                        "completed_cycles": 2,
                        "completed_tasks": 1,
                    },
                ],
            }
        }
    )

    period: PeriodResponse
    days: list[DayResponse] = Field(
        description="One entry per day of the period, ascending; inactive days are zeros."
    )

    @classmethod
    def from_series(cls, series: DailySeries) -> "DailyResponse":
        return cls(
            period=PeriodResponse.of(series.period, series.timezone),
            days=[
                DayResponse(
                    day=day.day,
                    studied_seconds=day.studied_seconds,
                    completed_cycles=day.completed_cycles,
                    completed_tasks=day.completed_tasks,
                )
                for day in series.days
            ],
        )


class SubjectResponse(BaseModel):
    subject_id: uuid.UUID = Field(description="Resolve the name with the Go API's subjects.")
    studied_seconds: int
    completed_cycles: int
    completed_tasks: int

    @classmethod
    def of(cls, subject: SubjectMetrics) -> "SubjectResponse":
        return cls(
            subject_id=subject.subject_id,
            studied_seconds=subject.studied_seconds,
            completed_cycles=subject.completed_cycles,
            completed_tasks=subject.completed_tasks,
        )


class SubjectsResponse(BaseModel):
    model_config = ConfigDict(
        json_schema_extra={
            "example": {
                "period": {
                    "from": "2026-11-01",
                    "to": "2026-11-30",
                    "timezone": "America/Sao_Paulo",
                },
                "subjects": [
                    {
                        "subject_id": "aaaaaaaa-0000-4000-8000-000000000001",
                        "studied_seconds": 27000,
                        "completed_cycles": 12,
                        "completed_tasks": 8,
                    },
                    {
                        "subject_id": "aaaaaaaa-0000-4000-8000-000000000002",
                        "studied_seconds": 25500,
                        "completed_cycles": 11,
                        "completed_tasks": 5,
                    },
                    {
                        "subject_id": "aaaaaaaa-0000-4000-8000-000000000003",
                        "studied_seconds": 22500,
                        "completed_cycles": 10,
                        "completed_tasks": 5,
                    },
                ],
            }
        }
    )

    period: PeriodResponse
    subjects: list[SubjectResponse] = Field(
        description="Subjects with activity in the period, most studied first "
        "(ties by subject_id). Empty when there is no activity."
    )

    @classmethod
    def from_report(cls, report: SubjectsReport) -> "SubjectsResponse":
        return cls(
            period=PeriodResponse.of(report.period, report.timezone),
            subjects=[SubjectResponse.of(subject) for subject in report.subjects],
        )


def _utc(moment: datetime | None) -> datetime | None:
    return moment.astimezone(UTC) if moment is not None else None


class SprintSummaryResponse(BaseModel):
    model_config = ConfigDict(
        json_schema_extra={
            "example": {
                "sprint_id": "34603048-a584-5fe3-b49c-7f792b0fe040",
                "studied_seconds": 7500,
                "completed_cycles": 3,
                "completed_tasks": 3,
                "first_cycle_started_at": "2026-11-20T12:00:00Z",
                "last_cycle_ended_at": "2026-11-21T01:50:00Z",
                "subjects": [
                    {
                        "subject_id": "aaaaaaaa-0000-4000-8000-000000000001",
                        "studied_seconds": 4500,
                        "completed_cycles": 2,
                        "completed_tasks": 3,
                    },
                    {
                        "subject_id": "aaaaaaaa-0000-4000-8000-000000000002",
                        "studied_seconds": 3000,
                        "completed_cycles": 1,
                        "completed_tasks": 0,
                    },
                ],
            }
        }
    )

    sprint_id: uuid.UUID
    studied_seconds: int
    completed_cycles: int
    completed_tasks: int
    first_cycle_started_at: datetime | None = Field(description="UTC.")
    last_cycle_ended_at: datetime | None = Field(description="UTC.")
    subjects: list[SubjectResponse] = Field(description="Same item shape and order as /subjects.")

    @classmethod
    def from_summary(cls, summary: SprintSummary) -> "SprintSummaryResponse":
        totals = summary.totals
        return cls(
            sprint_id=totals.sprint_id,
            studied_seconds=totals.studied_seconds,
            completed_cycles=totals.completed_cycles,
            completed_tasks=totals.completed_tasks,
            first_cycle_started_at=_utc(totals.first_cycle_started_at),
            last_cycle_ended_at=_utc(totals.last_cycle_ended_at),
            subjects=[SubjectResponse.of(subject) for subject in summary.subjects],
        )
