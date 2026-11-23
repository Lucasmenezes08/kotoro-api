"""Reporting period: inclusive calendar days in the metrics time zone (ADR 0002)."""

from dataclasses import dataclass
from datetime import date

MAX_PERIOD_DAYS = 366


class InvalidPeriodError(ValueError):
    """The period cannot be used. `field` names the query parameter to blame."""

    def __init__(self, field: str, message: str) -> None:
        super().__init__(message)
        self.field = field
        self.message = message


@dataclass(frozen=True)
class Period:
    start: date
    end: date

    def __post_init__(self) -> None:
        if self.start > self.end:
            raise InvalidPeriodError("from", "must be on or before 'to'")
        if self.days > MAX_PERIOD_DAYS:
            raise InvalidPeriodError("to", f"period must be at most {MAX_PERIOD_DAYS} days")

    @property
    def days(self) -> int:
        """Number of calendar days, both ends included."""
        return (self.end - self.start).days + 1
