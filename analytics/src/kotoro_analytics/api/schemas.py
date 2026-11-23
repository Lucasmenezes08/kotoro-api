"""HTTP response contracts. ORM models are never returned directly (onboarding §15)."""

from datetime import date

from pydantic import BaseModel, ConfigDict, Field

from kotoro_analytics.application.queries.daily import DailySeries
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
