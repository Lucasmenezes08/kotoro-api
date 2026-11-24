"""HTTP response contracts. ORM models are never returned directly (onboarding §15)."""

from datetime import date

from pydantic import BaseModel, ConfigDict, Field

from kotoro_analytics.application.queries.summary import Summary


class PeriodResponse(BaseModel):
    # `from` is a Python keyword: the attribute is `start`, the JSON key is "from".
    start: date = Field(serialization_alias="from", description="First day, inclusive.")
    to: date = Field(description="Last day, inclusive.")
    timezone: str = Field(description="IANA zone used to define the days.")


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
            period=PeriodResponse(
                start=summary.period.start, to=summary.period.end, timezone=summary.timezone
            ),
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
